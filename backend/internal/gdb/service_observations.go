package gdb

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/godlev/LANnventory/internal/models"
)

// RecordServiceObservation applies one definitive open/closed observation to the
// durable service summary without emitting a host activity event. It is used by
// lower-level callers and tests; host-aware scan paths should use RecordHostServiceObservation.
func RecordServiceObservation(service models.Service, observedAt string) (stored models.Service, persisted bool, stateChanged bool, err error) {
	normalized, err := normalizeObservedService(service, observedAt)
	if err != nil {
		return models.Service{}, false, false, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.Service{}, false, false, err
	}
	defer release()

	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var previousState string
		stored, persisted, stateChanged, previousState, err = recordServiceObservationTx(txDB, normalized, observedAt)
		_ = previousState
		return err
	})
	return stored, persisted, stateChanged, err
}

// RecordHostServiceObservation atomically applies a definitive service state and,
// only when the state changes, records service-opened/service-closed activity.
func RecordHostServiceObservation(host models.Host, service models.Service, observedAt string) (stored models.Service, persisted bool, stateChanged bool, err error) {
	normalized, err := normalizeObservedService(service, observedAt)
	if err != nil {
		return models.Service{}, false, false, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.Service{}, false, false, err
	}
	defer release()

	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var previousState string
		stored, persisted, stateChanged, previousState, err = recordServiceObservationTx(txDB, normalized, observedAt)
		if err != nil || !persisted || !stateChanged {
			return err
		}

		eventType := models.EventServiceOpened
		if normalized.State == string(models.ServiceStateClosed) {
			eventType = models.EventServiceClosed
		}

		eventHost := host
		eventHost.Mac = normalized.Mac
		eventHost.IP = normalized.Address
		event := models.NewHostEvent(
			eventHost,
			eventType,
			previousState,
			fmt.Sprintf("%s/%d", normalized.Protocol, normalized.Port),
		)
		event.Date = observedAt
		return addEventTx(txDB, event)
	})
	return stored, persisted, stateChanged, err
}

func normalizeObservedService(service models.Service, observedAt string) (models.Service, error) {
	service.LastChecked = observedAt
	service.StateChangedAt = observedAt
	return normalizeService(service)
}

func recordServiceObservationTx(txDB *gorm.DB, normalized models.Service, observedAt string) (stored models.Service, persisted bool, stateChanged bool, previousState string, err error) {
	existing, ok, err := selectServiceByIdentityForUpdate(txDB, normalized.Mac, normalized.Address, normalized.Protocol, normalized.Port)
	if err != nil {
		return models.Service{}, false, false, "", err
	}

	if !ok {
		if normalized.State == string(models.ServiceStateClosed) {
			return models.Service{}, false, false, "", nil
		}
		normalized.FirstDetected = observedAt
		normalized.LastDetected = observedAt
		normalized.LastChecked = observedAt
		normalized.StateChangedAt = observedAt
		if err := txDB.Table(servicesTable).Create(&normalized).Error; err != nil {
			return models.Service{}, false, false, "", err
		}
		return normalized, true, true, "", nil
	}

	previousState = existing.State
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
		return models.Service{}, false, false, previousState, err
	}
	return existing, true, stateChanged, previousState, nil
}

func selectServiceByIdentityForUpdate(activeDB *gorm.DB, mac, address, protocol string, port int) (service models.Service, ok bool, err error) {
	err = activeDB.Table(servicesTable).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("\"MAC\" = ? AND \"ADDRESS\" = ? AND \"PROTOCOL\" = ? AND \"PORT\" = ?", mac, address, protocol, port).
		First(&service).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return service, false, nil
	}
	if err != nil {
		return service, false, err
	}
	return service, true, nil
}
