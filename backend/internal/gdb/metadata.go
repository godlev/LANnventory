package gdb

import (
	"errors"

	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const hostMetadataTable = "host_metadata"

var errEmptyMetadataMAC = errors.New("metadata mac is empty")

// SelectCurrentHostsWithMetadata returns current hosts enriched through one metadata query.
func SelectCurrentHostsWithMetadata() (hosts []models.Host, ok bool) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return hosts, !check.IfError(err)
	}
	defer release()

	if err := activeDB.Table("now").Find(&hosts).Error; err != nil {
		return hosts, !check.IfError(err)
	}
	if err := enrichHostsWithMetadata(activeDB, hosts); err != nil {
		return hosts, !check.IfError(err)
	}

	return hosts, true
}

// SelectHostWithMetadataByID returns one current host enriched with inventory metadata.
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
	if err := enrichHostsWithMetadata(activeDB, hosts); err != nil {
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
