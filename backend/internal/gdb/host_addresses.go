package gdb

import (
	"errors"
	"net"
	"sort"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const hostAddressesTable = "host_addresses"

// RecordHostAddressObservations records all addresses seen in one successful scan.
// Rows not present in this scan remain as history but are marked inactive.
func RecordHostAddressObservations(hosts []models.Host) error {
	observations := normalizeHostAddressObservations(hosts)

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Transaction(func(txDB *gorm.DB) error {
		if err := txDB.Table(hostAddressesTable).
			Where("\"ID\" > ?", 0).
			Update("ACTIVE", false).Error; err != nil {
			return err
		}

		keys := make([]string, 0, len(observations))
		for key := range observations {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			observation := observations[key]
			var existing models.HostAddress
			err := txDB.Table(hostAddressesTable).
				Where("\"MAC\" = ? AND \"ADDRESS\" = ?", observation.Mac, observation.Address).
				First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := txDB.Table(hostAddressesTable).Create(&observation).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}

			if existing.FirstSeen == "" || (observation.FirstSeen != "" && observation.FirstSeen < existing.FirstSeen) {
				existing.FirstSeen = observation.FirstSeen
			}
			if existing.LastSeen == "" || observation.LastSeen > existing.LastSeen {
				existing.LastSeen = observation.LastSeen
			}
			existing.Family = observation.Family
			existing.Iface = observation.Iface
			existing.Active = true
			if err := txDB.Table(hostAddressesTable).Save(&existing).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SelectHostAddressesByMAC returns retained address observations for one MAC identity.
func SelectHostAddressesByMAC(mac string) ([]models.HostAddress, error) {
	canonical, err := identity.NormalizeMAC(mac)
	if err != nil {
		return nil, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.HostAddress
	err = activeDB.Table(hostAddressesTable).
		Where("\"MAC\" = ?", canonical).
		Order("\"ACTIVE\" DESC, \"LAST_SEEN\" DESC, \"ADDRESS\" ASC").
		Find(&rows).Error
	return rows, err
}

// SelectHostAddressesByAddress returns every MAC identity observed with an address.
func SelectHostAddressesByAddress(address string) ([]models.HostAddress, error) {
	canonical, _, ok := normalizeIPAddress(address)
	if !ok {
		return nil, errors.New("invalid IP address")
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.HostAddress
	err = activeDB.Table(hostAddressesTable).
		Where("\"ADDRESS\" = ?", canonical).
		Order("\"ACTIVE\" DESC, \"LAST_SEEN\" DESC, \"MAC\" ASC").
		Find(&rows).Error
	return rows, err
}

func normalizeHostAddressObservations(hosts []models.Host) map[string]models.HostAddress {
	observations := make(map[string]models.HostAddress)
	for _, host := range hosts {
		mac, err := identity.NormalizeMAC(host.Mac)
		if err != nil {
			continue
		}
		address, family, ok := normalizeIPAddress(host.IP)
		if !ok || strings.TrimSpace(host.Date) == "" {
			continue
		}

		key := mac + "\x00" + address
		candidate := models.HostAddress{
			Mac:       mac,
			Address:   address,
			Family:    family,
			Iface:     strings.TrimSpace(host.Iface),
			FirstSeen: host.Date,
			LastSeen:  host.Date,
			Active:    true,
		}
		if existing, found := observations[key]; found {
			if candidate.FirstSeen < existing.FirstSeen {
				existing.FirstSeen = candidate.FirstSeen
			}
			if candidate.LastSeen > existing.LastSeen {
				existing.LastSeen = candidate.LastSeen
			}
			if existing.Iface == "" || (candidate.Iface != "" && candidate.Iface < existing.Iface) {
				existing.Iface = candidate.Iface
			}
			observations[key] = existing
			continue
		}
		observations[key] = candidate
	}
	return observations
}

func normalizeIPAddress(value string) (address, family string, ok bool) {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return "", "", false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String(), "ipv4", true
	}
	return ip.String(), "ipv6", true
}

func backfillHostAddresses(activeDB *gorm.DB) error {
	type historyRow struct {
		Mac       string `gorm:"column:MAC"`
		Address   string `gorm:"column:ADDRESS"`
		FirstSeen string `gorm:"column:FIRST_SEEN"`
		LastSeen  string `gorm:"column:LAST_SEEN"`
	}

	return activeDB.Transaction(func(txDB *gorm.DB) error {
		var historyRows []historyRow
		if err := txDB.Table("history").
			Select("\"MAC\", \"IP\" AS \"ADDRESS\", MIN(\"DATE\") AS \"FIRST_SEEN\", MAX(\"DATE\") AS \"LAST_SEEN\"").
			Where("\"MAC\" <> ? AND \"IP\" <> ?", "", "").
			Where("\"IFACE\" <> ?", "").
			Where("\"NOW\" = ?", 1).
			Group("\"MAC\", \"IP\"").
			Scan(&historyRows).Error; err != nil {
			return err
		}

		var currentHosts []models.Host
		if err := txDB.Table("now").
			Where("\"MAC\" <> ? AND \"IP\" <> ? AND \"IFACE\" <> ?", "", "", "").
			Find(&currentHosts).Error; err != nil {
			return err
		}

		currentByKey := make(map[string]models.Host, len(currentHosts))
		for _, host := range currentHosts {
			mac, err := identity.NormalizeMAC(host.Mac)
			if err != nil {
				continue
			}
			address, _, ok := normalizeIPAddress(host.IP)
			if !ok {
				continue
			}
			currentByKey[mac+"\x00"+address] = host
		}

		var existingRows []models.HostAddress
		if err := txDB.Table(hostAddressesTable).Find(&existingRows).Error; err != nil {
			return err
		}
		existingByKey := make(map[string]models.HostAddress, len(existingRows))
		for _, row := range existingRows {
			existingByKey[row.Mac+"\x00"+row.Address] = row
		}

		if err := txDB.Table(hostAddressesTable).
			Where("\"ID\" > ?", 0).
			Update("ACTIVE", false).Error; err != nil {
			return err
		}

		for _, row := range historyRows {
			mac, err := identity.NormalizeMAC(row.Mac)
			if err != nil {
				continue
			}
			address, family, ok := normalizeIPAddress(row.Address)
			if !ok {
				continue
			}
			key := mac + "\x00" + address
			if existing, found := existingByKey[key]; found {
				mergeAddressTimeBounds(&existing, row.FirstSeen, row.LastSeen)
				existing.Family = family
				if current, currentFound := currentByKey[key]; currentFound {
					mergeCurrentHostAddressObservation(&existing, current)
				}
				if err := txDB.Table(hostAddressesTable).Save(&existing).Error; err != nil {
					return err
				}
				existingByKey[key] = existing
				continue
			}

			observation := models.HostAddress{
				Mac:       mac,
				Address:   address,
				Family:    family,
				FirstSeen: row.FirstSeen,
				LastSeen:  row.LastSeen,
			}
			if current, found := currentByKey[key]; found {
				mergeCurrentHostAddressObservation(&observation, current)
			}
			if err := txDB.Table(hostAddressesTable).Create(&observation).Error; err != nil {
				return err
			}
			existingByKey[key] = observation
		}

		for key, current := range currentByKey {
			if existing, found := existingByKey[key]; found {
				mergeCurrentHostAddressObservation(&existing, current)
				if err := txDB.Table(hostAddressesTable).Save(&existing).Error; err != nil {
					return err
				}
				existingByKey[key] = existing
				continue
			}
			mac, err := identity.NormalizeMAC(current.Mac)
			if err != nil {
				continue
			}
			address, family, ok := normalizeIPAddress(current.IP)
			if !ok || current.Date == "" {
				continue
			}
			observation := models.HostAddress{
				Mac:     mac,
				Address: address,
				Family:  family,
			}
			mergeCurrentHostAddressObservation(&observation, current)
			if err := txDB.Table(hostAddressesTable).Create(&observation).Error; err != nil {
				return err
			}
			existingByKey[key] = observation
		}
		return nil
	})
}

func mergeCurrentHostAddressObservation(observation *models.HostAddress, current models.Host) {
	mergeAddressTimeBounds(observation, current.Date, current.Date)
	observation.Active = current.Now == 1
	observation.Iface = current.Iface
}

func mergeAddressTimeBounds(observation *models.HostAddress, firstSeen, lastSeen string) {
	if firstSeen != "" && (observation.FirstSeen == "" || firstSeen < observation.FirstSeen) {
		observation.FirstSeen = firstSeen
	}
	if lastSeen != "" && (observation.LastSeen == "" || lastSeen > observation.LastSeen) {
		observation.LastSeen = lastSeen
	}
}
