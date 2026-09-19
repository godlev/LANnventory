package gdb

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const (
	infrastructureWorkloadsTable          = "infrastructure_workloads"
	infrastructureWorkloadInterfacesTable = "infrastructure_workload_interfaces"
	infrastructureWorkloadHostLinksTable  = "infrastructure_workload_host_links"
)

// SelectInfrastructureWorkloadsByHypervisorMAC returns workload inventory owned
// by one hypervisor Host. Workloads are not projected into the Host table.
func SelectInfrastructureWorkloadsByHypervisorMAC(mac string) ([]models.InfrastructureWorkloadRecord, error) {
	canonical, err := identity.NormalizeMAC(mac)
	if err != nil {
		return nil, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	return selectInfrastructureWorkloadsByHypervisorMACDB(activeDB, canonical)
}

// SelectInfrastructureWorkloadByID returns one workload aggregate.
func SelectInfrastructureWorkloadByID(id uint) (models.InfrastructureWorkloadRecord, bool, error) {
	if id == 0 {
		return models.InfrastructureWorkloadRecord{}, false, errors.New("invalid workload id")
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.InfrastructureWorkloadRecord{}, false, err
	}
	defer release()

	record, found, err := selectInfrastructureWorkloadByIDDB(activeDB, id)
	return record, found, err
}

// UpsertInfrastructureWorkload writes one normalized workload and replaces its
// allowlisted interfaces atomically.
func UpsertInfrastructureWorkload(hypervisorMac string, input models.InfrastructureWorkloadUpsert, observedAt string) (models.InfrastructureWorkloadRecord, error) {
	canonical, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return models.InfrastructureWorkloadRecord{}, err
	}

	nativeID := strings.TrimSpace(input.NativeID)
	workloadType := strings.TrimSpace(input.WorkloadType)
	if nativeID == "" {
		return models.InfrastructureWorkloadRecord{}, errors.New("workload native id is required")
	}
	if workloadType != models.InfrastructureWorkloadTypeVM && workloadType != models.InfrastructureWorkloadTypeContainer {
		return models.InfrastructureWorkloadRecord{}, errors.New("invalid workload type")
	}

	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = models.InfrastructureWorkloadStatusUnknown
	}
	if status != models.InfrastructureWorkloadStatusUnknown &&
		status != models.InfrastructureWorkloadStatusRunning &&
		status != models.InfrastructureWorkloadStatusStopped {
		return models.InfrastructureWorkloadRecord{}, errors.New("invalid workload status")
	}

	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = models.InfrastructureWorkloadSourceManual
	}
	observedAt = strings.TrimSpace(observedAt)
	if observedAt == "" {
		observedAt = time.Now().UTC().Format(time.RFC3339)
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339)

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.InfrastructureWorkloadRecord{}, err
	}
	defer release()

	var workloadID uint
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var workload models.InfrastructureWorkload
		err := txDB.Table(infrastructureWorkloadsTable).
			Where(`"HYPERVISOR_MAC" = ? AND "NATIVE_ID" = ? AND "WORKLOAD_TYPE" = ?`, canonical, nativeID, workloadType).
			First(&workload).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			workload = models.InfrastructureWorkload{
				HypervisorMac: canonical,
				NativeID:      nativeID,
				WorkloadType:  workloadType,
				FirstSeen:     observedAt,
			}
		} else if err != nil {
			return err
		}

		workload.Name = strings.TrimSpace(input.Name)
		workload.Status = status
		workload.Source = source
		if workload.FirstSeen == "" {
			workload.FirstSeen = observedAt
		}
		workload.LastSeen = observedAt
		workload.RetiredAt = ""
		workload.UpdatedAt = updatedAt

		if workload.ID == 0 {
			if err := txDB.Table(infrastructureWorkloadsTable).Create(&workload).Error; err != nil {
				return err
			}
		} else if err := txDB.Table(infrastructureWorkloadsTable).Save(&workload).Error; err != nil {
			return err
		}
		workloadID = workload.ID

		if err := txDB.Table(infrastructureWorkloadInterfacesTable).
			Where(`"WORKLOAD_ID" = ?`, workload.ID).
			Delete(&models.InfrastructureWorkloadInterface{}).Error; err != nil {
			return err
		}

		names := make(map[string]struct{}, len(input.Interfaces))
		for _, candidate := range input.Interfaces {
			iface := candidate
			iface.ID = 0
			iface.WorkloadID = workload.ID
			iface.Name = strings.TrimSpace(iface.Name)
			if iface.Name == "" {
				return errors.New("workload interface name is required")
			}
			if _, exists := names[iface.Name]; exists {
				return errors.New("duplicate workload interface name")
			}
			names[iface.Name] = struct{}{}

			if strings.TrimSpace(iface.Mac) != "" {
				normalized, err := identity.NormalizeMAC(iface.Mac)
				if err != nil {
					return errors.New("invalid workload interface mac")
				}
				iface.Mac = normalized
			}
			iface.Bridge = strings.TrimSpace(iface.Bridge)
			iface.VLANTag = strings.TrimSpace(iface.VLANTag)
			iface.ConfiguredAddress = strings.TrimSpace(iface.ConfiguredAddress)
			iface.ConfiguredNetwork = strings.TrimSpace(iface.ConfiguredNetwork)
			iface.UpdatedAt = updatedAt

			if err := txDB.Table(infrastructureWorkloadInterfacesTable).Create(&iface).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.InfrastructureWorkloadRecord{}, err
	}

	record, found, err := selectInfrastructureWorkloadByIDDB(activeDB, workloadID)
	if err != nil {
		return models.InfrastructureWorkloadRecord{}, err
	}
	if !found {
		return models.InfrastructureWorkloadRecord{}, gorm.ErrRecordNotFound
	}
	return record, nil
}

// DeleteManualInfrastructureWorkload hard-deletes only a manually maintained
// workload. Imported workload retirement is handled by snapshot reconciliation.
func DeleteManualInfrastructureWorkload(id uint, hypervisorMac string) error {
	canonical, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return err
	}
	if id == 0 {
		return errors.New("invalid workload id")
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Transaction(func(txDB *gorm.DB) error {
		var workload models.InfrastructureWorkload
		err := txDB.Table(infrastructureWorkloadsTable).
			Where(`"ID" = ? AND "HYPERVISOR_MAC" = ?`, id, canonical).
			First(&workload).Error
		if err != nil {
			return err
		}
		if workload.Source != models.InfrastructureWorkloadSourceManual {
			return errors.New("only manual workloads can be deleted directly")
		}
		return deleteInfrastructureWorkloadTx(txDB, workload.ID)
	})
}

// SetInfrastructureWorkloadHostLink creates or replaces the logical MATCHES
// relation for a workload without merging either identity.
func SetInfrastructureWorkloadHostLink(workloadID uint, host models.Host, linkSource, changedAt string) (models.InfrastructureWorkloadHostLink, error) {
	if workloadID == 0 || host.ID < 1 {
		return models.InfrastructureWorkloadHostLink{}, errors.New("invalid workload or host")
	}
	hostMac, err := identity.NormalizeMAC(host.Mac)
	if err != nil {
		return models.InfrastructureWorkloadHostLink{}, err
	}
	linkSource = strings.TrimSpace(linkSource)
	if linkSource != models.InfrastructureWorkloadLinkSourceManual && linkSource != models.InfrastructureWorkloadLinkSourceExactMAC {
		return models.InfrastructureWorkloadHostLink{}, errors.New("invalid workload link source")
	}
	changedAt = strings.TrimSpace(changedAt)
	if changedAt == "" {
		changedAt = time.Now().UTC().Format(time.RFC3339)
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.InfrastructureWorkloadHostLink{}, err
	}
	defer release()

	var result models.InfrastructureWorkloadHostLink
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var workload models.InfrastructureWorkload
		if err := txDB.Table(infrastructureWorkloadsTable).Where(`"ID" = ?`, workloadID).First(&workload).Error; err != nil {
			return err
		}

		var link models.InfrastructureWorkloadHostLink
		err := txDB.Table(infrastructureWorkloadHostLinksTable).
			Where(`"WORKLOAD_ID" = ?`, workloadID).
			First(&link).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			link = models.InfrastructureWorkloadHostLink{
				WorkloadID: workloadID,
				LinkedAt:   changedAt,
			}
		} else if err != nil {
			return err
		}
		if link.LinkedAt == "" {
			link.LinkedAt = changedAt
		}
		link.HostID = host.ID
		link.HostMac = hostMac
		link.LinkSource = linkSource
		link.UpdatedAt = changedAt

		if err := txDB.Table(infrastructureWorkloadHostLinksTable).Save(&link).Error; err != nil {
			return err
		}
		result = link
		return nil
	})
	return result, err
}

// DeleteInfrastructureWorkloadHostLink removes one logical MATCHES relation.
func DeleteInfrastructureWorkloadHostLink(workloadID uint) error {
	if workloadID == 0 {
		return errors.New("invalid workload id")
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Table(infrastructureWorkloadHostLinksTable).
		Where(`"WORKLOAD_ID" = ?`, workloadID).
		Delete(&models.InfrastructureWorkloadHostLink{}).Error
}

func selectInfrastructureWorkloadsByHypervisorMACDB(activeDB *gorm.DB, canonical string) ([]models.InfrastructureWorkloadRecord, error) {
	var workloads []models.InfrastructureWorkload
	if err := activeDB.Table(infrastructureWorkloadsTable).
		Where(`"HYPERVISOR_MAC" = ?`, canonical).
		Order(`"RETIRED_AT" ASC, "WORKLOAD_TYPE" ASC, "NATIVE_ID" ASC, "ID" ASC`).
		Find(&workloads).Error; err != nil {
		return nil, err
	}

	records := make([]models.InfrastructureWorkloadRecord, 0, len(workloads))
	for _, workload := range workloads {
		record, found, err := selectInfrastructureWorkloadByIDDB(activeDB, workload.ID)
		if err != nil {
			return nil, err
		}
		if found {
			records = append(records, record)
		}
	}
	return records, nil
}

func selectInfrastructureWorkloadByIDDB(activeDB *gorm.DB, id uint) (models.InfrastructureWorkloadRecord, bool, error) {
	var workload models.InfrastructureWorkload
	err := activeDB.Table(infrastructureWorkloadsTable).Where(`"ID" = ?`, id).First(&workload).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.InfrastructureWorkloadRecord{}, false, nil
	}
	if err != nil {
		return models.InfrastructureWorkloadRecord{}, false, err
	}

	var interfaces []models.InfrastructureWorkloadInterface
	if err := activeDB.Table(infrastructureWorkloadInterfacesTable).
		Where(`"WORKLOAD_ID" = ?`, id).
		Order(`"NAME" ASC, "ID" ASC`).
		Find(&interfaces).Error; err != nil {
		return models.InfrastructureWorkloadRecord{}, false, err
	}

	var link models.InfrastructureWorkloadHostLink
	linkFound := true
	err = activeDB.Table(infrastructureWorkloadHostLinksTable).
		Where(`"WORKLOAD_ID" = ?`, id).
		First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		linkFound = false
	} else if err != nil {
		return models.InfrastructureWorkloadRecord{}, false, err
	}

	record := models.InfrastructureWorkloadRecord{
		Workload:   workload,
		Interfaces: interfaces,
	}
	if linkFound {
		record.Link = &link
	}
	return record, true, nil
}

func deleteInfrastructureWorkloadTx(txDB *gorm.DB, workloadID uint) error {
	if err := txDB.Table(infrastructureWorkloadHostLinksTable).
		Where(`"WORKLOAD_ID" = ?`, workloadID).
		Delete(&models.InfrastructureWorkloadHostLink{}).Error; err != nil {
		return err
	}
	if err := txDB.Table(infrastructureWorkloadInterfacesTable).
		Where(`"WORKLOAD_ID" = ?`, workloadID).
		Delete(&models.InfrastructureWorkloadInterface{}).Error; err != nil {
		return err
	}
	return txDB.Table(infrastructureWorkloadsTable).
		Where(`"ID" = ?`, workloadID).
		Delete(&models.InfrastructureWorkload{}).Error
}

// deleteInfrastructureRelationsForHost keeps workload/Host relations consistent
// when a current Host is deleted. Workloads hosted by the deleted hypervisor are
// removed; workloads hosted elsewhere are merely unlinked from the deleted guest Host.
func deleteInfrastructureRelationsForHost(txDB *gorm.DB, host models.Host) error {
	mac, err := identity.NormalizeMAC(host.Mac)
	if err != nil {
		return nil
	}

	if err := txDB.Table(infrastructureWorkloadHostLinksTable).
		Where(`"HOST_MAC" = ? OR "HOST_ID" = ?`, mac, host.ID).
		Delete(&models.InfrastructureWorkloadHostLink{}).Error; err != nil {
		return err
	}

	var workloadIDs []uint
	if err := txDB.Table(infrastructureWorkloadsTable).
		Where(`"HYPERVISOR_MAC" = ?`, mac).
		Pluck("ID", &workloadIDs).Error; err != nil {
		return err
	}
	sort.Slice(workloadIDs, func(i, j int) bool { return workloadIDs[i] < workloadIDs[j] })
	for _, workloadID := range workloadIDs {
		if err := deleteInfrastructureWorkloadTx(txDB, workloadID); err != nil {
			return err
		}
	}
	return nil
}
