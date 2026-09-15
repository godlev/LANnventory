package gdb

import (
	"sort"
	"strconv"
	"time"

	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const hostPortsTable = "host_ports"

// SelectHostPorts returns persisted TCP port states for one current host.
func SelectHostPorts(hostID int) ([]models.HostPort, error) {
	states := []models.HostPort{}

	activeDB, release, err := acquireDB()
	if err != nil {
		return states, err
	}
	defer release()

	err = activeDB.Table(hostPortsTable).
		Where("\"HOST_ID\" = ?", hostID).
		Order("\"PORT\" ASC").
		Find(&states).Error
	return states, err
}

// ReconcileHostPortObservations persists one completed/cancelled scan's observed
// ports and records events only when an open/closed transition is meaningful.
func ReconcileHostPortObservations(host models.Host, observations map[int]bool, observedAt string) ([]models.HostEvent, error) {
	events := []models.HostEvent{}
	if host.ID < 1 || len(observations) == 0 {
		return events, nil
	}
	if observedAt == "" {
		observedAt = time.Now().Format(models.HostEventDateLayout)
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return events, err
	}
	defer release()

	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		existing := []models.HostPort{}
		if err := txDB.Table(hostPortsTable).
			Where("\"HOST_ID\" = ? AND \"PROTOCOL\" = ?", host.ID, "tcp").
			Find(&existing).Error; err != nil {
			return err
		}

		byPort := make(map[int]models.HostPort, len(existing))
		for _, state := range existing {
			byPort[state.Port] = state
		}

		ports := make([]int, 0, len(observations))
		for port := range observations {
			ports = append(ports, port)
		}
		sort.Ints(ports)

		for _, port := range ports {
			if port < 1 || port > 65535 {
				continue
			}

			open := observations[port]
			state, exists := byPort[port]
			if !exists {
				if !open {
					continue
				}

				state = models.HostPort{
					HostID:      host.ID,
					Port:        port,
					Protocol:    "tcp",
					Open:        true,
					FirstSeen:   observedAt,
					LastScanned: observedAt,
					LastChanged: observedAt,
				}
				if err := txDB.Table(hostPortsTable).Create(&state).Error; err != nil {
					return err
				}

				event := models.NewHostEvent(host, models.EventPortOpen, "", strconv.Itoa(port))
				event.Date = observedAt
				if err := addEventTx(txDB, event); err != nil {
					return err
				}
				events = append(events, event)
				continue
			}

			state.LastScanned = observedAt
			if state.Open == open {
				if err := txDB.Table(hostPortsTable).Save(&state).Error; err != nil {
					return err
				}
				continue
			}

			state.Open = open
			state.LastChanged = observedAt
			if open && state.FirstSeen == "" {
				state.FirstSeen = observedAt
			}
			if err := txDB.Table(hostPortsTable).Save(&state).Error; err != nil {
				return err
			}

			eventType := models.EventPortClosed
			if open {
				eventType = models.EventPortOpen
			}
			event := models.NewHostEvent(host, eventType, "", strconv.Itoa(port))
			event.Date = observedAt
			if err := addEventTx(txDB, event); err != nil {
				return err
			}
			events = append(events, event)
		}

		return nil
	})
	return events, err
}

func deleteHostPortsByHostID(txDB *gorm.DB, hostID int) error {
	return txDB.Table(hostPortsTable).
		Where("\"HOST_ID\" = ?", hostID).
		Delete(&models.HostPort{}).Error
}
