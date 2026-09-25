package gdb

import (
	"errors"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

var (
	ErrInfrastructureWorkloadSourceConflict = errors.New("infrastructure workload source conflict")
	ErrProxmoxAPIConfigChanged              = errors.New("Proxmox API configuration changed")
)

type proxmoxAPIImportGuard struct {
	ConfigRevision   uint64
	RequireAutomatic bool
}

// ApplyProxmoxScriptImport preserves the Phase 35A script-import entry point.
func ApplyProxmoxScriptImport(hypervisorMac string, state models.ProxmoxSourceState, workloads []models.InfrastructureWorkloadUpsert) error {
	if state.Source != models.InfrastructureWorkloadSourceScriptImport {
		return errors.New("invalid Proxmox script import source")
	}
	return ApplyProxmoxImport(hypervisorMac, state, workloads)
}

// ApplyProxmoxImport atomically applies one complete, validated Proxmox snapshot.
// Manual or unknown-source workload rows are never overwritten. Script/API
// provenance can transition because they are two collection modes for the same
// canonical infrastructure workload inventory.
func ApplyProxmoxImport(hypervisorMac string, state models.ProxmoxSourceState, workloads []models.InfrastructureWorkloadUpsert) error {
	return applyProxmoxImport(hypervisorMac, state, workloads, nil)
}

// ApplyProxmoxAPIImportIfRevision atomically verifies that the API integration
// still has the expected configuration revision before applying the snapshot.
// The guard is acquired inside the same transaction as the inventory write so
// a concurrent configuration change cannot slip between validation and apply.
func ApplyProxmoxAPIImportIfRevision(
	hypervisorMac string,
	state models.ProxmoxSourceState,
	workloads []models.InfrastructureWorkloadUpsert,
	configRevision uint64,
	requireAutomatic bool,
) error {
	if state.Source != models.InfrastructureWorkloadSourceProxmoxAPI {
		return errors.New("revision-guarded Proxmox import requires proxmox-api source")
	}
	return applyProxmoxImport(hypervisorMac, state, workloads, &proxmoxAPIImportGuard{
		ConfigRevision:   configRevision,
		RequireAutomatic: requireAutomatic,
	})
}

func applyProxmoxImport(hypervisorMac string, state models.ProxmoxSourceState, workloads []models.InfrastructureWorkloadUpsert, guard *proxmoxAPIImportGuard) error {
	canonical, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return err
	}
	if !isManagedProxmoxWorkloadSource(state.Source) {
		return errors.New("invalid Proxmox import source")
	}
	if !state.Complete {
		return errors.New("incomplete Proxmox snapshot cannot be applied")
	}
	if strings.TrimSpace(state.CollectedAt) == "" || strings.TrimSpace(state.ImportedAt) == "" || strings.TrimSpace(state.SnapshotDigest) == "" {
		return errors.New("Proxmox source state is incomplete")
	}

	state.HypervisorMac = canonical

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Transaction(func(txDB *gorm.DB) error {
		if guard != nil {
			query := txDB.Table(proxmoxAPIConfigsTable).
				Where(`"HYPERVISOR_MAC" = ? AND "CONFIG_REVISION" = ? AND "ENABLED" = ?`,
					canonical, guard.ConfigRevision, true)
			if guard.RequireAutomatic {
				query = query.Where(`"AUTOMATIC_SYNC" = ?`, true)
			}
			result := query.Update("CONFIG_REVISION", guard.ConfigRevision)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrProxmoxAPIConfigChanged
			}
		}

		incoming := make(map[string]struct{}, len(workloads))
		for _, input := range workloads {
			if input.Source != state.Source || !isManagedProxmoxWorkloadSource(input.Source) {
				return errors.New("invalid imported workload source")
			}
			nativeID := strings.TrimSpace(input.NativeID)
			workloadType := strings.TrimSpace(input.WorkloadType)
			if nativeID == "" {
				return errors.New("workload native id is required")
			}
			if workloadType != models.InfrastructureWorkloadTypeVM && workloadType != models.InfrastructureWorkloadTypeContainer {
				return errors.New("invalid workload type")
			}
			status := strings.TrimSpace(input.Status)
			if status != models.InfrastructureWorkloadStatusUnknown &&
				status != models.InfrastructureWorkloadStatusRunning &&
				status != models.InfrastructureWorkloadStatusStopped {
				return errors.New("invalid workload status")
			}

			key := workloadType + "\x00" + nativeID
			if _, exists := incoming[key]; exists {
				return errors.New("duplicate imported workload")
			}
			incoming[key] = struct{}{}

			var workload models.InfrastructureWorkload
			err := txDB.Table(infrastructureWorkloadsTable).
				Where(`"HYPERVISOR_MAC" = ? AND "NATIVE_ID" = ? AND "WORKLOAD_TYPE" = ?`, canonical, nativeID, workloadType).
				First(&workload).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				workload = models.InfrastructureWorkload{
					HypervisorMac: canonical,
					NativeID:      nativeID,
					WorkloadType:  workloadType,
					FirstSeen:     state.CollectedAt,
				}
			} else if err != nil {
				return err
			} else if !isManagedProxmoxWorkloadSource(workload.Source) {
				return ErrInfrastructureWorkloadSourceConflict
			}

			workload.NodeName = strings.TrimSpace(input.NodeName)
			workload.Name = strings.TrimSpace(input.Name)
			workload.Status = status
			workload.Source = state.Source
			if workload.FirstSeen == "" {
				workload.FirstSeen = state.CollectedAt
			}
			workload.LastSeen = state.CollectedAt
			workload.RetiredAt = ""
			workload.UpdatedAt = state.ImportedAt

			if workload.ID == 0 {
				if err := txDB.Table(infrastructureWorkloadsTable).Create(&workload).Error; err != nil {
					return err
				}
			} else if err := txDB.Table(infrastructureWorkloadsTable).Save(&workload).Error; err != nil {
				return err
			}

			if err := txDB.Table(infrastructureWorkloadInterfacesTable).
				Where(`"WORKLOAD_ID" = ?`, workload.ID).
				Delete(&models.InfrastructureWorkloadInterface{}).Error; err != nil {
				return err
			}

			interfaceNames := make(map[string]struct{}, len(input.Interfaces))
			for _, candidate := range input.Interfaces {
				iface := candidate
				iface.ID = 0
				iface.WorkloadID = workload.ID
				iface.Name = strings.TrimSpace(iface.Name)
				if iface.Name == "" {
					return errors.New("workload interface name is required")
				}
				if _, exists := interfaceNames[iface.Name]; exists {
					return errors.New("duplicate workload interface name")
				}
				interfaceNames[iface.Name] = struct{}{}

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
				iface.UpdatedAt = state.ImportedAt
				if err := txDB.Table(infrastructureWorkloadInterfacesTable).Create(&iface).Error; err != nil {
					return err
				}
			}
		}

		var existingImported []models.InfrastructureWorkload
		if err := txDB.Table(infrastructureWorkloadsTable).
			Where(`"HYPERVISOR_MAC" = ? AND "SOURCE" IN ?`, canonical, []string{
				models.InfrastructureWorkloadSourceScriptImport,
				models.InfrastructureWorkloadSourceProxmoxAPI,
			}).
			Find(&existingImported).Error; err != nil {
			return err
		}
		for _, workload := range existingImported {
			key := workload.WorkloadType + "\x00" + workload.NativeID
			if _, present := incoming[key]; present || workload.RetiredAt != "" {
				continue
			}
			if !shouldRetireMissingImportedWorkload(state.Source, workload.Source) {
				continue
			}
			if err := txDB.Table(infrastructureWorkloadsTable).
				Where(`"ID" = ?`, workload.ID).
				Updates(map[string]any{
					"RETIRED_AT": state.CollectedAt,
					"UPDATED_AT": state.ImportedAt,
				}).Error; err != nil {
				return err
			}
		}

		if err := reconcileExactMACWorkloadLinksTx(txDB, canonical, state.ImportedAt); err != nil {
			return err
		}

		var existingState models.ProxmoxSourceState
		err := txDB.Table(proxmoxSourceStatesTable).
			Where(`"HYPERVISOR_MAC" = ? AND "SOURCE" = ?`, canonical, state.Source).
			First(&existingState).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return txDB.Table(proxmoxSourceStatesTable).Create(&state).Error
		}
		if err != nil {
			return err
		}
		return txDB.Table(proxmoxSourceStatesTable).
			Where(`"HYPERVISOR_MAC" = ? AND "SOURCE" = ?`, canonical, state.Source).
			Updates(map[string]any{
				"SCHEMA_VERSION":    state.SchemaVersion,
				"COLLECTOR_VERSION": state.CollectorVersion,
				"COLLECTED_AT":      state.CollectedAt,
				"COMPLETE":          state.Complete,
				"NODE_HOSTNAME":     state.NodeHostname,
				"NODE_PVE_VERSION":  state.NodePVEVersion,
				"NODE_CLUSTER_NAME": state.NodeClusterName,
				"NODE_STATUS":       state.NodeStatus,
				"SNAPSHOT_DIGEST":   state.SnapshotDigest,
				"IMPORTED_AT":       state.ImportedAt,
			}).Error
	})
}


func isManagedProxmoxWorkloadSource(source string) bool {
	switch strings.TrimSpace(source) {
	case models.InfrastructureWorkloadSourceScriptImport, models.InfrastructureWorkloadSourceProxmoxAPI:
		return true
	default:
		return false
	}
}

func shouldRetireMissingImportedWorkload(incomingSource, currentSource string) bool {
	incomingSource = strings.TrimSpace(incomingSource)
	currentSource = strings.TrimSpace(currentSource)
	switch incomingSource {
	case models.InfrastructureWorkloadSourceProxmoxAPI:
		// API inventory is cluster-aware and authoritative across the managed
		// Proxmox inventory scope, including migration from script imports.
		return isManagedProxmoxWorkloadSource(currentSource)
	case models.InfrastructureWorkloadSourceScriptImport:
		// Script imports are node-local. They may retire their own prior script
		// rows, but must not retire API-managed rows from another cluster node.
		return currentSource == models.InfrastructureWorkloadSourceScriptImport
	default:
		return false
	}
}
