package gdb

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/godlev/LANnventory/internal/models"
)

// RecordServiceObservation applies one definitive open/closed observation to the
// durable service summary. Closed observations for never-seen services are ignored
// so broad scans do not create an inventory row for every closed port.
func RecordServiceObservation(service models.Service, observedAt string) (stored models.Service, persisted bool, stateChanged bool, err error) {
	service.LastChecked = observedAt
	service.StateChangedAt = observedAt
	normalized, err := normalizeService(service)
	if err != nil {
		return models.Service{}, false, false, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.Service{}, false, false, err
	}
	defer release()

	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		existing, ok, selectErr := selectServiceByIdentityForUpdate(txDB, normalized.Mac, normalized.Address, normalized.Protocol, normalized.Port)
		if selectErr != nil {
			return selectErr
		}

		if !ok {
			if normalized.State == string(models.ServiceStateClosed) {
				return nil
			}
			normalized.FirstDetected = observedAt
			normalized.LastDetected = observedAt
			normalized.LastChecked = observedAt
			normalized.StateChangedAt = observedAt
			if err := txDB.Table(servicesTable).Create(&normalized).Error; err != nil {
				return err
			}
			stored = normalized
			persisted = true
			stateChanged = true
			return nil
		}

		stateChanged = existing.State != normalized.State
		existing.State = normalized.State
		existing.LastChecked = observedAt
		existing.LastScanSource = normalized.LastScanSource
		if normalized.ServiceHint != "" {
			existing.ServiceHint = normalized.ServiceHint
		}
		if normalized.State == string(models.ServiceStateOpen) {
			if existing.FirstDetected == "" {
				existing.FirstDetected = observedAt
			}
			existing.LastDetected = observedAt
		}
		if stateChanged {
			existing.StateChangedAt = observedAt
		}

		if err := txDB.Table(servicesTable).Save(&existing).Error; err != nil {
			return err
		}
		stored = existing
		persisted = true
		return nil
	})
	return stored, persisted, stateChanged, err
}

func selectServiceByIdentityForUpdate(activeDB *gorm.DB, mac, address, protocol string, port int) (service models.Service, ok bool, err error) {
	err = activeDB.Table(servicesTable).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("\"MAC\" = ? AND \"ADDRESS\" = ? AND \"PROTOCOL\" = ? AND \"PORT\" = ?", mac, address, protocol, port).
		First(&service).Error
	if err == gorm.ErrRecordNotFound {
		return service, false, nil
	}
	if err != nil {
		return service, false, err
	}
	return service, true, nil
}
