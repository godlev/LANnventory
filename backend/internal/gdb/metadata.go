package gdb

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const hostMetadataTable = "host_metadata"

var errEmptyMetadataMAC = errors.New("metadata mac is empty")

// IsEmptyMetadataMACError reports whether err means metadata cannot be saved for an empty MAC.
func IsEmptyMetadataMACError(err error) bool {
	return errors.Is(err, errEmptyMetadataMAC)
}

// SelectCurrentHostsWithMetadata returns current hosts enriched through batched inventory queries.
func SelectCurrentHostsWithMetadata() (hosts []models.Host, ok bool) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return hosts, !check.IfError(err)
	}
	defer release()

	if err := activeDB.Table("now").Find(&hosts).Error; err != nil {
		return hosts, !check.IfError(err)
	}
	if err := enrichHostsWithInventory(activeDB, hosts); err != nil {
		return hosts, !check.IfError(err)
	}

	return hosts, true
}

// SelectHostWithMetadataByID returns one current host enriched with inventory data.
func SelectHostWithMetadataByID(id int) (host models.Host, err error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return host, err
	}
	defer release()

	if err := activeDB.Table("now").First(&host, id).Error; err != nil {
		return host, err
	}

	hosts := []models.Host{host}
	if err := enrichHostsWithInventory(activeDB, hosts); err != nil {
		return host, err
	}

	return hosts[0], nil
}

// SelectHostMetadataByMAC returns metadata for one MAC address.
func SelectHostMetadataByMAC(mac string) (metadata models.HostMetadata, ok bool, err error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return metadata, false, err
	}
	defer release()

	return selectHostMetadataByMAC(activeDB, mac)
}

// SelectHostMetadataForMACs returns all metadata records matching the provided MACs.
func SelectHostMetadataForMACs(macs []string) (map[string]models.HostMetadata, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	return selectHostMetadataForMACs(activeDB, macs)
}

// UpsertHostMetadata applies a partial metadata update for the exact MAC.
func UpsertHostMetadata(mac string, update models.HostMetadataUpdate) (models.HostMetadata, error) {
	if mac == "" {
		return models.HostMetadata{}, errEmptyMetadataMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.HostMetadata{}, err
	}
	defer release()

	var saved models.HostMetadata
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		metadata, ok, err := selectHostMetadataByMAC(txDB, mac)
		if err != nil {
			return err
		}
		if !ok {
			metadata = models.HostMetadata{
				Mac:      mac,
				TagsJSON: "[]",
			}
		}

		if update.Owner != nil {
			metadata.Owner = *update.Owner
		}
		if update.Location != nil {
			metadata.Location = *update.Location
		}
		if update.Notes != nil {
			metadata.Notes = *update.Notes
		}
		if update.Tags != nil {
			metadata.TagsJSON = models.EncodeMetadataTags(*update.Tags)
		}
		if update.Pinned != nil {
			metadata.Pinned = *update.Pinned
		}

		if err := txDB.Table(hostMetadataTable).Save(&metadata).Error; err != nil {
			return err
		}

		saved = metadata
		return nil
	})

	return saved, err
}

// UpdateHostMetadataWithEvents applies metadata changes and records one event per actual changed field.
func UpdateHostMetadataWithEvents(host models.Host, update models.HostMetadataUpdate) (models.HostMetadata, error) {
	if host.Mac == "" {
		return models.HostMetadata{}, errEmptyMetadataMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.HostMetadata{}, err
	}
	defer release()

	var saved models.HostMetadata
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		metadata, ok, err := selectHostMetadataByMAC(txDB, host.Mac)
		if err != nil {
			return err
		}
		if !ok {
			metadata = models.HostMetadata{
				Mac:      host.Mac,
				TagsJSON: "[]",
			}
		}

		eventDate := time.Now().Format(models.HostEventDateLayout)
		events := make([]models.HostEvent, 0, 5)

		if update.Owner != nil && metadata.Owner != *update.Owner {
			events = append(events, metadataEvent(host, models.EventOwnerChanged, metadata.Owner, *update.Owner, eventDate))
			metadata.Owner = *update.Owner
		}
		if update.Location != nil && metadata.Location != *update.Location {
			events = append(events, metadataEvent(host, models.EventLocationChanged, metadata.Location, *update.Location, eventDate))
			metadata.Location = *update.Location
		}
		if update.Notes != nil && metadata.Notes != *update.Notes {
			events = append(events, metadataEvent(host, models.EventNotesChanged, metadata.Notes, *update.Notes, eventDate))
			metadata.Notes = *update.Notes
		}
		if update.Tags != nil {
			oldTagsJSON := models.EncodeMetadataTags(models.DecodeMetadataTags(metadata.TagsJSON))
			newTagsJSON := models.EncodeMetadataTags(*update.Tags)
			if oldTagsJSON != newTagsJSON {
				events = append(events, metadataEvent(host, models.EventTagsChanged, oldTagsJSON, newTagsJSON, eventDate))
				metadata.TagsJSON = newTagsJSON
			}
		}
		if update.Pinned != nil && metadata.Pinned != *update.Pinned {
			events = append(events, metadataEvent(
				host,
				models.EventPinnedChanged,
				strconv.FormatBool(metadata.Pinned),
				strconv.FormatBool(*update.Pinned),
				eventDate,
			))
			metadata.Pinned = *update.Pinned
		}

		if len(events) == 0 {
			saved = metadata
			return nil
		}

		if err := txDB.Table(hostMetadataTable).Save(&metadata).Error; err != nil {
			return err
		}
		for _, event := range events {
			if err := addEventTx(txDB, event); err != nil {
				return err
			}
		}

		saved = metadata
		return nil
	})

	return saved, err
}

// UpdateHostInventoryWithEvents applies host and metadata edits atomically and records ordered change events.
func UpdateHostInventoryWithEvents(id int, update models.HostInventoryUpdate) (models.Host, error) {
	var saved models.Host

	activeDB, release, err := acquireDB()
	if err != nil {
		return saved, err
	}
	defer release()

	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var host models.Host
		if err := txDB.Table("now").First(&host, id).Error; err != nil {
			return err
		}

		metadataUpdateRequested := update.Owner != nil || update.Location != nil || update.Notes != nil || update.Tags != nil
		var metadata models.HostMetadata
		if metadataUpdateRequested {
			if host.Mac == "" {
				return errEmptyMetadataMAC
			}

			var ok bool
			var err error
			metadata, ok, err = selectHostMetadataByMAC(txDB, host.Mac)
			if err != nil {
				return err
			}
			if !ok {
				metadata = models.HostMetadata{
					Mac:      host.Mac,
					TagsJSON: "[]",
				}
			}
		}

		oldKnown := host.Known
		oldDeviceType := host.DeviceType
		hostChanged := false
		if update.Name != nil && host.Name != *update.Name {
			host.Name = *update.Name
			hostChanged = true
		}
		if update.Known != nil && host.Known != *update.Known {
			host.Known = *update.Known
			hostChanged = true
		}
		if update.DeviceType != nil && host.DeviceType != *update.DeviceType {
			host.DeviceType = *update.DeviceType
			hostChanged = true
		}

		eventDate := time.Now().Format(models.HostEventDateLayout)
		events := make([]models.HostEvent, 0, 6)
		if update.Known != nil && oldKnown != host.Known {
			eventType := models.EventUnknown
			if host.Known == 1 {
				eventType = models.EventKnown
			}
			events = append(events, metadataEvent(host, eventType, "", "", eventDate))
		}
		if update.DeviceType != nil && oldDeviceType != host.DeviceType {
			events = append(events, metadataEvent(host, models.EventDeviceTypeChanged, oldDeviceType, host.DeviceType, eventDate))
		}

		metadataChanged := false
		if update.Owner != nil && metadata.Owner != *update.Owner {
			events = append(events, metadataEvent(host, models.EventOwnerChanged, metadata.Owner, *update.Owner, eventDate))
			metadata.Owner = *update.Owner
			metadataChanged = true
		}
		if update.Location != nil && metadata.Location != *update.Location {
			events = append(events, metadataEvent(host, models.EventLocationChanged, metadata.Location, *update.Location, eventDate))
			metadata.Location = *update.Location
			metadataChanged = true
		}
		if update.Notes != nil && metadata.Notes != *update.Notes {
			events = append(events, metadataEvent(host, models.EventNotesChanged, metadata.Notes, *update.Notes, eventDate))
			metadata.Notes = *update.Notes
			metadataChanged = true
		}
		if update.Tags != nil {
			oldTagsJSON := models.EncodeMetadataTags(models.DecodeMetadataTags(metadata.TagsJSON))
			newTagsJSON := models.EncodeMetadataTags(*update.Tags)
			if oldTagsJSON != newTagsJSON {
				events = append(events, metadataEvent(host, models.EventTagsChanged, oldTagsJSON, newTagsJSON, eventDate))
				metadata.TagsJSON = newTagsJSON
				metadataChanged = true
			}
		}

		if hostChanged {
			if err := txDB.Table("now").Save(&host).Error; err != nil {
				return err
			}
		}
		if metadataChanged {
			if err := txDB.Table(hostMetadataTable).Save(&metadata).Error; err != nil {
				return err
			}
		}
		for _, event := range events {
			if err := addEventTx(txDB, event); err != nil {
				return err
			}
		}

		hosts := []models.Host{host}
		if err := enrichHostsWithInventory(txDB, hosts); err != nil {
			return err
		}
		saved = hosts[0]

		return nil
	})

	return saved, err
}

// SelectInventoryOptions returns distinct owner and location values from current inventory metadata.
func SelectInventoryOptions() (models.InventoryOptions, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return models.InventoryOptions{}, err
	}
	defer release()

	type inventoryOptionRow struct {
		Owner    string `gorm:"column:OWNER"`
		Location string `gorm:"column:LOCATION"`
	}
	var rows []inventoryOptionRow
	if err := activeDB.Table(hostMetadataTable + " AS m").
		Select("m.\"OWNER\", m.\"LOCATION\"").
		Joins("INNER JOIN \"now\" AS n ON n.\"MAC\" = m.\"MAC\"").
		Order("n.\"ID\" ASC").
		Order("m.\"MAC\" ASC").
		Scan(&rows).Error; err != nil {
		return models.InventoryOptions{}, err
	}

	owners := make([]string, 0, len(rows))
	locations := make([]string, 0, len(rows))
	for _, row := range rows {
		owners = append(owners, row.Owner)
		locations = append(locations, row.Location)
	}

	return models.InventoryOptions{
		Owners:    normalizeInventoryOptions(owners),
		Locations: normalizeInventoryOptions(locations),
	}, nil
}

// DeleteHostMetadataByMAC removes metadata for a deleted current host.
func DeleteHostMetadataByMAC(mac string) error {
	if mac == "" {
		return nil
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return deleteHostMetadataByMAC(activeDB, mac)
}

func deleteHostMetadataByMAC(activeDB *gorm.DB, mac string) error {
	if mac == "" {
		return nil
	}

	return activeDB.Table(hostMetadataTable).
		Where("\"MAC\" = ?", mac).
		Delete(&models.HostMetadata{}).Error
}

func enrichHostsWithMetadata(activeDB *gorm.DB, hosts []models.Host) error {
	metadataByMAC, err := selectHostMetadataForHosts(activeDB, hosts)
	if err != nil {
		return err
	}

	for i := range hosts {
		metadata, ok := metadataByMAC[hosts[i].Mac]
		models.ApplyHostMetadata(&hosts[i], metadata, ok)
	}

	return nil
}

func metadataEvent(host models.Host, eventType models.HostEventType, oldValue, newValue, date string) models.HostEvent {
	event := models.NewHostEvent(host, eventType, oldValue, newValue)
	event.Date = date
	return event
}

func normalizeInventoryOptions(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		normalized = append(normalized, value)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		left := strings.ToLower(normalized[i])
		right := strings.ToLower(normalized[j])
		if left == right {
			return normalized[i] < normalized[j]
		}
		return left < right
	})

	return normalized
}

func enrichHostsWithInventory(activeDB *gorm.DB, hosts []models.Host) error {
	if err := enrichHostsWithMetadata(activeDB, hosts); err != nil {
		return err
	}
	return enrichHostsWithLifecycle(activeDB, hosts)
}

func selectHostMetadataForHosts(activeDB *gorm.DB, hosts []models.Host) (map[string]models.HostMetadata, error) {
	macs := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if host.Mac == "" {
			continue
		}
		if _, ok := seen[host.Mac]; ok {
			continue
		}

		seen[host.Mac] = struct{}{}
		macs = append(macs, host.Mac)
	}

	return selectHostMetadataForMACs(activeDB, macs)
}

func selectHostMetadataForMACs(activeDB *gorm.DB, macs []string) (map[string]models.HostMetadata, error) {
	metadataByMAC := make(map[string]models.HostMetadata, len(macs))
	macs = normalizeMacs(macs)
	if len(macs) == 0 {
		return metadataByMAC, nil
	}

	var rows []models.HostMetadata
	if err := activeDB.Table(hostMetadataTable).
		Where("\"MAC\" IN ?", macs).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		metadataByMAC[row.Mac] = row
	}

	return metadataByMAC, nil
}

func selectHostMetadataByMAC(activeDB *gorm.DB, mac string) (metadata models.HostMetadata, ok bool, err error) {
	if mac == "" {
		return metadata, false, nil
	}

	err = activeDB.Table(hostMetadataTable).
		Where("\"MAC\" = ?", mac).
		First(&metadata).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return metadata, false, nil
	}
	if err != nil {
		return metadata, false, err
	}

	return metadata, true, nil
}
