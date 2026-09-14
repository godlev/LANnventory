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

func TestUpdateHostInventoryWithEventsUpdatesHostMetadataAndOrderedEvents(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:         1,
		Name:       "desktop",
		IP:         "192.168.1.42",
		Iface:      "eth0",
		Mac:        "AA:BB:CC:DD:EE:42",
		Known:      0,
		Now:        1,
		DeviceType: "",
	}
	seedExportCurrentHost(t, host)
	if err := RecordHostObservation(host.Mac, "2026-09-01 09:30:00"); err != nil {
		t.Fatalf("RecordHostObservation: %v", err)
	}

	name := "workstation"
	known := 1
	deviceType := "desktop"
	owner := "Miroslav"
	location := "Office"
	notes := "Primary workstation"
	tags := []string{"daily", "trusted"}
	updated, err := UpdateHostInventoryWithEvents(host.ID, models.HostInventoryUpdate{
		Name:       &name,
		Known:      &known,
		DeviceType: &deviceType,
		Owner:      &owner,
		Location:   &location,
		Notes:      &notes,
		Tags:       &tags,
	})
	if err != nil {
		t.Fatalf("UpdateHostInventoryWithEvents: %v", err)
	}

	if updated.Name != name || updated.Known != known || updated.DeviceType != deviceType {
		t.Fatalf("updated host fields = %+v, want name/known/device type changes", updated)
	}
	if updated.Owner != owner || updated.Location != location || updated.Notes != notes || updated.Pinned {
		t.Fatalf("updated metadata fields = %+v, want owner/location/notes and unchanged pinned", updated)
	}
	if updated.FirstSeen != "2026-09-01 09:30:00" || updated.LastSeen != "2026-09-01 09:30:00" || updated.FirstSeenEstimated {
		t.Fatalf("updated lifecycle fields = %+v, want enriched lifecycle", updated)
	}
	assertStringSlice(t, updated.Tags, tags, "updated tags")

	events := readEventsAscending(t)
	wantTypes := []models.HostEventType{
		models.EventKnown,
		models.EventDeviceTypeChanged,
		models.EventOwnerChanged,
		models.EventLocationChanged,
		models.EventNotesChanged,
		models.EventTagsChanged,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("events len = %d, want %d: %+v", len(events), len(wantTypes), events)
	}
	for index, wantType := range wantTypes {
		if events[index].EventType != string(wantType) {
			t.Fatalf("events[%d].EventType = %q, want %q; events: %+v", index, events[index].EventType, wantType, events)
		}
		if events[index].HostID != host.ID || events[index].Mac != host.Mac || events[index].Name != name || events[index].DeviceType != deviceType {
			t.Fatalf("events[%d] snapshot = %+v, want final host fields", index, events[index])
		}
	}
	assertEventValue(t, events[1], "", "desktop")
	assertEventValue(t, events[2], "", "Miroslav")
	assertEventValue(t, events[3], "", "Office")
	assertEventValue(t, events[4], "", "Primary workstation")
	assertEventValue(t, events[5], "[]", `["daily","trusted"]`)
}

func TestUpdateHostInventoryWithEventsNameOnlyCreatesNoEvents(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Known: 1,
		Now:   1,
	}
	seedExportCurrentHost(t, host)

	name := "gateway"
	updated, err := UpdateHostInventoryWithEvents(host.ID, models.HostInventoryUpdate{Name: &name})
	if err != nil {
		t.Fatalf("UpdateHostInventoryWithEvents name only: %v", err)
	}
	if updated.Name != name {
		t.Fatalf("updated.Name = %q, want %q", updated.Name, name)
	}
	events := readEventsAscending(t)
	if len(events) != 0 {
		t.Fatalf("name-only update created events: %+v", events)
	}
}

func TestUpdateHostInventoryWithEventsRollsBackMetadataAndEventsOnHostFailure(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Known: 0,
	}
	seedExportCurrentHost(t, host)

	if err := db.Exec(`
		CREATE TRIGGER fail_now_update
		BEFORE UPDATE ON now
		BEGIN
			SELECT RAISE(ABORT, 'forced host update failure');
		END;
	`).Error; err != nil {
		t.Fatalf("create host update failure trigger: %v", err)
	}

	name := "gateway"
	known := 1
	owner := "Network Team"
	if _, err := UpdateHostInventoryWithEvents(host.ID, models.HostInventoryUpdate{
		Name:  &name,
		Known: &known,
		Owner: &owner,
	}); err == nil {
		t.Fatal("UpdateHostInventoryWithEvents error = nil, want forced host update failure")
	}

	reread := SelectByID(host.ID)
	if reread.Name != host.Name || reread.Known != host.Known {
		t.Fatalf("host after rollback = %+v, want original %+v", reread, host)
	}
	_, ok, err := SelectHostMetadataByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC: %v", err)
	}
	if ok {
		t.Fatal("metadata row exists despite host update rollback")
	}
	if events := readEventsAscending(t); len(events) != 0 {
		t.Fatalf("events after host update rollback = %+v, want none", events)
	}
}

func TestUpdateHostInventoryWithEventsRollsBackHostAndEventsOnMetadataFailure(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Known: 0,
	}
	seedExportCurrentHost(t, host)

	if err := db.Exec(`
		CREATE TRIGGER fail_host_metadata_insert
		BEFORE INSERT ON host_metadata
		BEGIN
			SELECT RAISE(ABORT, 'forced metadata insert failure');
		END;
	`).Error; err != nil {
		t.Fatalf("create metadata insert failure trigger: %v", err)
	}

	name := "gateway"
	known := 1
	owner := "Network Team"
	if _, err := UpdateHostInventoryWithEvents(host.ID, models.HostInventoryUpdate{
		Name:  &name,
		Known: &known,
		Owner: &owner,
	}); err == nil {
		t.Fatal("UpdateHostInventoryWithEvents error = nil, want forced metadata insert failure")
	}

	reread := SelectByID(host.ID)
	if reread.Name != host.Name || reread.Known != host.Known {
		t.Fatalf("host after rollback = %+v, want original %+v", reread, host)
	}
	_, ok, err := SelectHostMetadataByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC: %v", err)
	}
	if ok {
		t.Fatal("metadata row exists despite metadata insert rollback")
	}
	if events := readEventsAscending(t); len(events) != 0 {
		t.Fatalf("events after metadata insert rollback = %+v, want none", events)
	}
}

func TestUpdateHostInventoryWithEventsRollsBackHostAndMetadataOnEventFailure(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:         1,
		Name:       "NAS",
		Mac:        "AA:BB:CC:DD:EE:20",
		Known:      0,
		DeviceType: "",
	}
	seedExportCurrentHost(t, host)

	if err := db.Exec(`
		CREATE TRIGGER fail_inventory_event_insert
		BEFORE INSERT ON events
		BEGIN
			SELECT RAISE(ABORT, 'forced inventory event insert failure');
		END;
	`).Error; err != nil {
		t.Fatalf("create event insert failure trigger: %v", err)
	}

	name := "Storage"
	known := 1
	deviceType := "nas"
	owner := "Storage Team"
	if _, err := UpdateHostInventoryWithEvents(host.ID, models.HostInventoryUpdate{
		Name:       &name,
		Known:      &known,
		DeviceType: &deviceType,
		Owner:      &owner,
	}); err == nil {
		t.Fatal("UpdateHostInventoryWithEvents error = nil, want forced event insert failure")
	}

	reread := SelectByID(host.ID)
	if reread.Name != host.Name || reread.Known != host.Known || reread.DeviceType != host.DeviceType {
		t.Fatalf("host after rollback = %+v, want original %+v", reread, host)
	}
	_, ok, err := SelectHostMetadataByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC: %v", err)
	}
	if ok {
		t.Fatal("metadata row exists despite rollback")
	}
}

func TestSelectInventoryOptionsUsesCurrentMetadataAndSortsCaseInsensitively(t *testing.T) {
	startSelectTestDB(t)
	seedExportCurrentHost(t, models.Host{ID: 1, Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	seedExportCurrentHost(t, models.Host{ID: 2, Name: "nas", Mac: "AA:BB:CC:DD:EE:02"})
	seedExportCurrentHost(t, models.Host{ID: 3, Name: "desktop", Mac: "AA:BB:CC:DD:EE:03"})

	office := "Office"
	lowerOffice := "office"
	lab := "Lab"
	zoe := "Zoe"
	alex := "Alex"
	lowerAlex := "alex"
	if _, err := UpsertHostMetadata("AA:BB:CC:DD:EE:01", models.HostMetadataUpdate{Owner: &zoe, Location: &office}); err != nil {
		t.Fatalf("UpsertHostMetadata router: %v", err)
	}
	if _, err := UpsertHostMetadata("AA:BB:CC:DD:EE:02", models.HostMetadataUpdate{Owner: &alex, Location: &lab}); err != nil {
		t.Fatalf("UpsertHostMetadata nas: %v", err)
	}
	if _, err := UpsertHostMetadata("AA:BB:CC:DD:EE:03", models.HostMetadataUpdate{Owner: &lowerAlex, Location: &lowerOffice}); err != nil {
		t.Fatalf("UpsertHostMetadata desktop: %v", err)
	}
	if _, err := UpsertHostMetadata("AA:BB:CC:DD:EE:99", models.HostMetadataUpdate{Owner: &zoe, Location: &lab}); err != nil {
		t.Fatalf("UpsertHostMetadata orphan: %v", err)
	}

	options, err := SelectInventoryOptions()
	if err != nil {
		t.Fatalf("SelectInventoryOptions: %v", err)
	}
	assertStringSlice(t, options.Owners, []string{"Alex", "Zoe"}, "owners")
	assertStringSlice(t, options.Locations, []string{"Lab", "Office"}, "locations")
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
