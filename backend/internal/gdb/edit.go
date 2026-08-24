package gdb

import (
	"errors"
	"log/slog"

	"github.com/aceberg/WatchYourLAN/internal/check"
	"github.com/aceberg/WatchYourLAN/internal/models"

	"gorm.io/gorm"
)

// Update - update or create host
func Update(table string, oneHost models.Host) {

	check.IfError(UpdateWithError(table, oneHost))
}

// UpdateWithError updates or creates a host and returns persistence errors.
func UpdateWithError(table string, oneHost models.Host) error {

	tab := db.Table(table)
	result := tab.Save(&oneHost)

	return result.Error
}

// UpdateDeviceType updates only the manual DeviceType field for a host.
func UpdateDeviceType(id int, deviceType string) (models.Host, error) {
	var host models.Host

	tab := db.Table("now")
	result := tab.Model(&models.Host{}).
		Where("\"ID\" = ?", id).
		Update("DEVICE_TYPE", deviceType)
	if result.Error != nil {
		return host, result.Error
	}
	if result.RowsAffected == 0 {
		return host, gorm.ErrRecordNotFound
	}

	err := tab.First(&host, id).Error
	return host, err
}

// Delete - delete host from DB
func Delete(table string, id int) {

	tab := db.Table(table)
	result := tab.Delete(&models.Host{}, id)
	check.IfError(result.Error)
}

// DeleteOldHistory - delete a list of hosts from History
func DeleteOldHistory(date string) int64 {

	tab := db.Table("history")
	result := tab.Where("\"DATE\" < ?", date).Delete(&models.Host{})
	check.IfError(result.Error)

	return result.RowsAffected
}

// AddEvent stores a validated host activity event.
func AddEvent(event models.HostEvent) error {

	if !models.IsValidHostEventType(event.EventType) {
		return errors.New("invalid host event type")
	}

	tab := db.Table("events")
	result := tab.Create(&event)

	return result.Error
}

// RecordHostEvent stores an activity event and logs failures without failing callers.
func RecordHostEvent(host models.Host, eventType models.HostEventType, oldValue, newValue string) {

	err := AddEvent(models.NewHostEvent(host, eventType, oldValue, newValue))
	if err != nil {
		slog.Error("Failed to record host event", "eventType", eventType, "mac", host.Mac, "err", err)
	}
}

// DeleteOldEvents removes host activity events older than date.
func DeleteOldEvents(date string) int64 {

	tab := db.Table("events")
	result := tab.Where("\"DATE\" < ?", date).Delete(&models.HostEvent{})
	check.IfError(result.Error)

	return result.RowsAffected
}

// Clear - delete all hosts from table
func Clear(table string) {

	tab := db.Table(table)
	result := tab.Where("1 = 1").Delete(&models.Host{})
	check.IfError(result.Error)
}
