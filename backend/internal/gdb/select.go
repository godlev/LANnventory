package gdb

import (
	"github.com/aceberg/WatchYourLAN/internal/check"
	"github.com/aceberg/WatchYourLAN/internal/models"
)

// EventQuery describes filters for persistent activity event selection.
type EventQuery struct {
	Limit      int
	Offset     int
	Mac        string
	HostID     int
	EventTypes []models.HostEventType
}

// Select - get all hosts
func Select(table string) (dbHosts []models.Host, ok bool) {

	tab := db.Table(table)
	err := tab.Find(&dbHosts).Error

	return dbHosts, !check.IfError(err)
}

// SelectByID - get host by ID
func SelectByID(id int) (host models.Host) {

	tab := db.Table("now")
	tab.First(&host, id)

	return host
}

// SelectByMAC - get all hosts by MAC
func SelectByMAC(table, mac string) (hosts []models.Host) {

	tab := db.Table(table)
	tab.Where("\"MAC\" = ?", mac).Find(&hosts)

	return hosts
}

// SelectByDate - get all hosts by MAC and DATE
func SelectByDate(mac, date string) (hosts []models.Host) {

	tab := db.Table("history")
	tab.
		Where("\"MAC\" = ?", mac).
		Where("\"DATE\" LIKE ?", date+"%").
		Find(&hosts)

	return hosts
}

// SelectLatest - get latest hosts by MAC
func SelectLatest(mac string, number int) (hosts []models.Host) {

	tab := db.Table("history")
	tab.
		Where("\"MAC\" = ?", mac).
		Order("\"DATE\" DESC").
		Limit(number).
		Find(&hosts)

	return hosts
}

// SelectEvents returns recent host activity events, newest first.
func SelectEvents(limit int, mac string) (events []models.HostEvent, ok bool) {

	return SelectEventsFiltered(EventQuery{
		Limit: limit,
		Mac:   mac,
	})
}

// SelectEventsFiltered returns recent host activity events matching query, newest first.
func SelectEventsFiltered(query EventQuery) (events []models.HostEvent, ok bool) {

	tab := db.Table("events")
	if query.Mac != "" {
		tab = tab.Where("\"MAC\" = ?", query.Mac)
	}
	if query.HostID > 0 {
		tab = tab.Where("\"HOST_ID\" = ?", query.HostID)
	}
	if len(query.EventTypes) > 0 {
		eventTypes := make([]string, 0, len(query.EventTypes))
		for _, eventType := range query.EventTypes {
			eventTypes = append(eventTypes, string(eventType))
		}
		tab = tab.Where("\"EVENT_TYPE\" IN ?", eventTypes)
	}
	err := tab.
		Order("\"DATE\" DESC").
		Order("\"ID\" DESC").
		Limit(query.Limit).
		Offset(query.Offset).
		Find(&events).Error

	return events, !check.IfError(err)
}

// SelectEventsByHostID returns recent host activity events for one host, newest first.
func SelectEventsByHostID(hostID int, limit int) (events []models.HostEvent, ok bool) {

	return SelectEventsFiltered(EventQuery{
		Limit:  limit,
		HostID: hostID,
	})
}
