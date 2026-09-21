package gdb

import (
	"errors"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/workloadmatch"
	"gorm.io/gorm"
)

// reconcileExactMACWorkloadLinksTx maintains only automatically-created
// exact-MAC workload links. Manual links are never changed by this function.
// Weaker address/name candidates remain suggestions and are never persisted here.
func reconcileExactMACWorkloadLinksTx(txDB *gorm.DB, hypervisorMac, changedAt string) error {
	canonical, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return err
	}
	changedAt = strings.TrimSpace(changedAt)
	if changedAt == "" {
		return errors.New("workload link reconciliation timestamp is required")
	}

	records, err := selectInfrastructureWorkloadsByHypervisorMACDB(txDB, canonical)
	if err != nil {
		return err
	}

	var hosts []models.Host
	if err := txDB.Table("now").Order(`"ID" ASC`).Find(&hosts).Error; err != nil {
		return err
	}
	hostByID := make(map[int]models.Host, len(hosts))
	for _, host := range hosts {
		if host.ID > 0 {
			hostByID[host.ID] = host
		}
	}

	for _, record := range records {
		if record.Workload.Source != models.InfrastructureWorkloadSourceScriptImport {
			continue
		}
		if record.Link != nil && record.Link.LinkSource == models.InfrastructureWorkloadLinkSourceManual {
			continue
		}

		match := workloadmatch.Match(record, canonical, hosts, nil, nil)
		targetID := match.DeterministicExactHostID
		if targetID == 0 {
			if record.Link != nil && record.Link.LinkSource == models.InfrastructureWorkloadLinkSourceExactMAC {
				if err := txDB.Table(infrastructureWorkloadHostLinksTable).
					Where(`"WORKLOAD_ID" = ?`, record.Workload.ID).
					Delete(&models.InfrastructureWorkloadHostLink{}).Error; err != nil {
					return err
				}
			}
			continue
		}

		target, exists := hostByID[targetID]
		if !exists || target.ID < 1 {
			return errors.New("exact-MAC match target is missing")
		}
		targetMAC, err := identity.NormalizeMAC(target.Mac)
		if err != nil {
			return err
		}

		if record.Link != nil &&
			record.Link.LinkSource == models.InfrastructureWorkloadLinkSourceExactMAC &&
			record.Link.HostID == target.ID &&
			record.Link.HostMac == targetMAC {
			continue
		}

		link := models.InfrastructureWorkloadHostLink{
			WorkloadID: record.Workload.ID,
			HostID:     target.ID,
			HostMac:    targetMAC,
			LinkSource: models.InfrastructureWorkloadLinkSourceExactMAC,
			LinkedAt:   changedAt,
			UpdatedAt:  changedAt,
		}
		if err := txDB.Table(infrastructureWorkloadHostLinksTable).Save(&link).Error; err != nil {
			return err
		}
	}
	return nil
}
