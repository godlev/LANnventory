package gdb

import (
	"errors"
	"sort"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const hostDiscoveryEvidenceTable = "host_discovery_evidence"

// RecordHostDiscoveryEvidence replaces the current values for one discovery
// source/kind scope while retaining superseded values as inactive evidence.
// This data is observational and is intentionally separate from user-managed
// inventory fields such as Name, DeviceType, Owner, Location, Notes and Tags.
func RecordHostDiscoveryEvidence(mac, address, source, kind string, values []string, observedAt string) error {
	canonicalMAC, err := identity.NormalizeMAC(mac)
	if err != nil {
		return err
	}

	canonicalAddress := ""
	if strings.TrimSpace(address) != "" {
		var ok bool
		canonicalAddress, _, ok = normalizeIPAddress(address)
		if !ok {
			return errors.New("invalid IP address")
		}
	}

	source = strings.ToLower(strings.TrimSpace(source))
	kind = strings.ToLower(strings.TrimSpace(kind))
	observedAt = strings.TrimSpace(observedAt)
	if source == "" {
		return errors.New("discovery source is required")
	}
	if kind == "" {
		return errors.New("discovery kind is required")
	}
	if observedAt == "" {
		return errors.New("observation timestamp is required")
	}

	values = normalizeDiscoveryEvidenceValues(values)

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Transaction(func(txDB *gorm.DB) error {
		scope := txDB.Table(hostDiscoveryEvidenceTable).
			Where("\"MAC\" = ? AND \"ADDRESS\" = ? AND \"SOURCE\" = ? AND \"KIND\" = ?", canonicalMAC, canonicalAddress, source, kind)
		if err := scope.Update("ACTIVE", false).Error; err != nil {
			return err
		}

		for _, value := range values {
			var existing models.HostDiscoveryEvidence
			err := txDB.Table(hostDiscoveryEvidenceTable).
				Where("\"MAC\" = ? AND \"ADDRESS\" = ? AND \"SOURCE\" = ? AND \"KIND\" = ? AND \"VALUE\" = ?", canonicalMAC, canonicalAddress, source, kind, value).
				First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				row := models.HostDiscoveryEvidence{
					Mac:       canonicalMAC,
					Address:   canonicalAddress,
					Source:    source,
					Kind:      kind,
					Value:     value,
					FirstSeen: observedAt,
					LastSeen:  observedAt,
					Active:    true,
				}
				if err := txDB.Table(hostDiscoveryEvidenceTable).Create(&row).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}

			if existing.FirstSeen == "" || observedAt < existing.FirstSeen {
				existing.FirstSeen = observedAt
			}
			if existing.LastSeen == "" || observedAt > existing.LastSeen {
				existing.LastSeen = observedAt
			}
			existing.Active = true
			if err := txDB.Table(hostDiscoveryEvidenceTable).Save(&existing).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SelectHostDiscoveryEvidenceByMAC returns both current and historical evidence
// for a MAC identity. Current values are returned first.
func SelectHostDiscoveryEvidenceByMAC(mac string) ([]models.HostDiscoveryEvidence, error) {
	canonicalMAC, err := identity.NormalizeMAC(mac)
	if err != nil {
		return nil, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.HostDiscoveryEvidence
	err = activeDB.Table(hostDiscoveryEvidenceTable).
		Where("\"MAC\" = ?", canonicalMAC).
		Order("\"ACTIVE\" DESC, \"SOURCE\" ASC, \"KIND\" ASC, \"ADDRESS\" ASC, \"LAST_SEEN\" DESC, \"VALUE\" ASC").
		Find(&rows).Error
	return rows, err
}

func normalizeDiscoveryEvidenceValues(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		unique[value] = struct{}{}
	}

	normalized := make([]string, 0, len(unique))
	for value := range unique {
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}
