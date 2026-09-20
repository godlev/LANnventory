package gdb

import (
	"errors"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const infrastructureWorkloadCandidateRejectionsTable = "infrastructure_workload_candidate_rejections"

// SetInfrastructureWorkloadCandidateRejection stores a non-destructive user
// rejection for one workload/Host candidate and the exact evidence state the
// user reviewed.
func SetInfrastructureWorkloadCandidateRejection(
	workloadID uint,
	host models.Host,
	evidenceFingerprint,
	evidenceStrength,
	changedAt string,
) (models.InfrastructureWorkloadCandidateRejection, error) {
	if workloadID == 0 || host.ID < 1 {
		return models.InfrastructureWorkloadCandidateRejection{}, errors.New("invalid workload or host")
	}
	hostMac, err := identity.NormalizeMAC(host.Mac)
	if err != nil {
		return models.InfrastructureWorkloadCandidateRejection{}, err
	}
	evidenceFingerprint = strings.TrimSpace(evidenceFingerprint)
	evidenceStrength = strings.TrimSpace(evidenceStrength)
	if evidenceFingerprint == "" {
		return models.InfrastructureWorkloadCandidateRejection{}, errors.New("evidence fingerprint is required")
	}
	if evidenceStrength == "" {
		return models.InfrastructureWorkloadCandidateRejection{}, errors.New("evidence strength is required")
	}
	changedAt = strings.TrimSpace(changedAt)
	if changedAt == "" {
		changedAt = time.Now().UTC().Format(time.RFC3339)
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.InfrastructureWorkloadCandidateRejection{}, err
	}
	defer release()

	var result models.InfrastructureWorkloadCandidateRejection
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var workload models.InfrastructureWorkload
		if err := txDB.Table(infrastructureWorkloadsTable).Where(`"ID" = ?`, workloadID).First(&workload).Error; err != nil {
			return err
		}

		var existing models.InfrastructureWorkloadCandidateRejection
		err := txDB.Table(infrastructureWorkloadCandidateRejectionsTable).
			Where(`"WORKLOAD_ID" = ? AND "HOST_ID" = ?`, workloadID, host.ID).
			First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = models.InfrastructureWorkloadCandidateRejection{
				WorkloadID:          workloadID,
				HostID:              host.ID,
				HostMac:             hostMac,
				EvidenceFingerprint: evidenceFingerprint,
				EvidenceStrength:    evidenceStrength,
				CreatedAt:           changedAt,
				UpdatedAt:           changedAt,
			}
			return txDB.Table(infrastructureWorkloadCandidateRejectionsTable).Create(&result).Error
		}
		if err != nil {
			return err
		}

		existing.HostMac = hostMac
		existing.EvidenceFingerprint = evidenceFingerprint
		existing.EvidenceStrength = evidenceStrength
		existing.UpdatedAt = changedAt
		if existing.CreatedAt == "" {
			existing.CreatedAt = changedAt
		}
		result = existing
		return txDB.Table(infrastructureWorkloadCandidateRejectionsTable).Save(&existing).Error
	})
	return result, err
}

// DeleteInfrastructureWorkloadCandidateRejection clears only the explicit
// rejection. It never changes the workload, Host, or an existing manual link.
func DeleteInfrastructureWorkloadCandidateRejection(workloadID uint, hostID int) error {
	if workloadID == 0 || hostID < 1 {
		return errors.New("invalid workload or host")
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Table(infrastructureWorkloadCandidateRejectionsTable).
		Where(`"WORKLOAD_ID" = ? AND "HOST_ID" = ?`, workloadID, hostID).
		Delete(&models.InfrastructureWorkloadCandidateRejection{}).Error
}

// SelectInfrastructureWorkloadCandidateRejections returns all saved rejection
// decisions for one workload. Stale fingerprints are retained as user history
// but do not suppress newly changed evidence.
func SelectInfrastructureWorkloadCandidateRejections(workloadID uint) ([]models.InfrastructureWorkloadCandidateRejection, error) {
	if workloadID == 0 {
		return nil, errors.New("invalid workload id")
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var rows []models.InfrastructureWorkloadCandidateRejection
	err = activeDB.Table(infrastructureWorkloadCandidateRejectionsTable).
		Where(`"WORKLOAD_ID" = ?`, workloadID).
		Order(`"HOST_ID" ASC`).
		Find(&rows).Error
	return rows, err
}
