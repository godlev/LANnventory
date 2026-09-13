package gdb

import (
	"errors"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestUpdateHostMetadataWithEventsCreatesOrderedEvents(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:         7,
		Name:       "NAS",
		IP:         "192.168.1.20",
		Iface:      "eth0",
		Mac:        "AA:BB:CC:DD:EE:20",
		DeviceType: "nas",
	}
	seedExportCurrentHost(t, host)

	owner := "Storage Team"
	location := "Rack 1"
	notes := "Primary backup target"
	tags := []string{"storage", "critical"}
	pinned := true
	if _, err := UpdateHostMetadataWithEvents(host, models.HostMetadataUpdate{
		Owner:    &owner,
		Location: &location,
		Notes:    &notes,
		Tags:     &tags,
		Pinned:   &pinned,
	}); err != nil {
		t.Fatalf("UpdateHostMetadataWithEvents: %v", err)
	}

	events := readEventsAscending(t)
	wantTypes := []models.HostEventType{
		models.EventOwnerChanged,
		models.EventLocationChanged,
		models.EventNotesChanged,
		models.EventTagsChanged,
		models.EventPinnedChanged,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("events len = %d, want %d: %+v", len(events), len(wantTypes), events)
	}
	for index, wantType := range wantTypes {
		if events[index].EventType != string(wantType) {
			t.Fatalf("events[%d].EventType = %q, want %q; events: %+v", index, events[index].EventType, wantType, events)
		}
		if events[index].HostID != host.ID || events[index].Mac != host.Mac || events[index].Name != host.Name || events[index].IP != host.IP || events[index].Iface != host.Iface || events[index].DeviceType != host.DeviceType {
			t.Fatalf("events[%d] snapshot = %+v, want host %+v", index, events[index], host)
		}
		if events[index].Date != events[0].Date {
			t.Fatalf("events[%d].Date = %q, want shared date %q", index, events[index].Date, events[0].Date)
		}
	}

	assertEventValue(t, events[0], "", "Storage Team")
	assertEventValue(t, events[1], "", "Rack 1")
	assertEventValue(t, events[2], "", "Primary backup target")
	assertEventValue(t, events[3], "[]", `["storage","critical"]`)
	assertEventValue(t, events[4], "false", "true")
}

func TestUpdateHostMetadataWithEventsSkipsCanonicalNoOps(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:   1,
		Name: "router",
		Mac:  "AA:BB:CC:DD:EE:01",
	}
	seedExportCurrentHost(t, host)
	owner := "Miroslav"
	tags := []string{"server"}
	pinned := false
	if _, err := UpsertHostMetadata(host.Mac, models.HostMetadataUpdate{
		Owner:  &owner,
		Tags:   &tags,
		Pinned: &pinned,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}

	if _, err := UpdateHostMetadataWithEvents(host, models.HostMetadataUpdate{
		Owner:  &owner,
		Tags:   &tags,
		Pinned: &pinned,
	}); err != nil {
		t.Fatalf("UpdateHostMetadataWithEvents no-op: %v", err)
	}

	events := readEventsAscending(t)
	if len(events) != 0 {
		t.Fatalf("no-op metadata update created events: %+v", events)
	}
}

func TestUpdateHostMetadataWithEventsRollsBackOnEventFailure(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:   1,
		Name: "router",
		Mac:  "AA:BB:CC:DD:EE:01",
	}
	seedExportCurrentHost(t, host)
	owner := "Network Team"
	if _, err := UpsertHostMetadata(host.Mac, models.HostMetadataUpdate{Owner: &owner}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}

	if err := db.Exec(`
		CREATE TRIGGER fail_metadata_event_insert
		BEFORE INSERT ON events
		BEGIN
			SELECT RAISE(ABORT, 'forced event insert failure');
		END;
	`).Error; err != nil {
		t.Fatalf("create event insert failure trigger: %v", err)
	}

	nextOwner := "Operations"
	if _, err := UpdateHostMetadataWithEvents(host, models.HostMetadataUpdate{Owner: &nextOwner}); err == nil {
		t.Fatal("UpdateHostMetadataWithEvents error = nil, want forced event insert failure")
	}

	metadata, ok, err := SelectHostMetadataByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC: %v", err)
	}
	if !ok || metadata.Owner != owner {
		t.Fatalf("metadata after event failure = %+v, ok=%v, want rollback to %q", metadata, ok, owner)
	}
}

func TestUpdateHostMetadataWithEventsRejectsEmptyMAC(t *testing.T) {
	startSelectTestDB(t)

	owner := "Network Team"
	_, err := UpdateHostMetadataWithEvents(models.Host{ID: 1}, models.HostMetadataUpdate{Owner: &owner})
	if !errors.Is(err, errEmptyMetadataMAC) {
		t.Fatalf("error = %v, want errEmptyMetadataMAC", err)
	}
}

func readEventsAscending(t *testing.T) []models.HostEvent {
	t.Helper()

	var events []models.HostEvent
	if err := db.Table("events").Order("\"ID\" ASC").Find(&events).Error; err != nil {
		t.Fatalf("read events: %v", err)
	}
	return events
}

func assertEventValue(t *testing.T, event models.HostEvent, oldValue, newValue string) {
	t.Helper()

	if event.OldValue != oldValue || event.NewValue != newValue {
		t.Fatalf("%s values = old %q new %q, want old %q new %q", event.EventType, event.OldValue, event.NewValue, oldValue, newValue)
	}
}
