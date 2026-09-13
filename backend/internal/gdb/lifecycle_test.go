package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestHostLifecycleMigrationCreatesSeparateTableOnly(t *testing.T) {
	startSelectTestDB(t)

	if !db.Migrator().HasTable(hostLifecycleTable) {
		t.Fatal("host_lifecycle table was not migrated")
	}

	for _, table := range []string{"now", "history"} {
		for _, column := range []string{"FIRST_SEEN", "LAST_SEEN", "FIRST_SEEN_ESTIMATED"} {
			if db.Table(table).Migrator().HasColumn(&models.Host{}, column) {
				t.Fatalf("%s unexpectedly has lifecycle column %s", table, column)
			}
		}
	}
}

func TestRecordHostObservationCreatesAndUpdatesLifecycle(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:10"

	if err := RecordHostObservation(mac, "2026-09-05 10:00:00"); err != nil {
		t.Fatalf("RecordHostObservation create: %v", err)
	}
	lifecycle := assertLifecycle(t, mac)
	if lifecycle.FirstSeen != "2026-09-05 10:00:00" || lifecycle.LastSeen != "2026-09-05 10:00:00" || lifecycle.FirstSeenEstimated {
		t.Fatalf("new lifecycle = %+v, want exact first/last observation", lifecycle)
	}

	if err := RecordHostObservation(mac, "2026-09-05 10:05:00"); err != nil {
		t.Fatalf("RecordHostObservation update: %v", err)
	}
	lifecycle = assertLifecycle(t, mac)
	if lifecycle.FirstSeen != "2026-09-05 10:00:00" || lifecycle.LastSeen != "2026-09-05 10:05:00" || lifecycle.FirstSeenEstimated {
		t.Fatalf("updated lifecycle = %+v, want first preserved and last updated", lifecycle)
	}
}

func TestManualLifecyclePlaceholderIsInitializedByLaterObservation(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:20"

	if err := EnsureHostLifecyclePlaceholder(mac); err != nil {
		t.Fatalf("EnsureHostLifecyclePlaceholder: %v", err)
	}
	lifecycle := assertLifecycle(t, mac)
	if lifecycle.FirstSeen != "" || lifecycle.LastSeen != "" || lifecycle.FirstSeenEstimated {
		t.Fatalf("placeholder lifecycle = %+v, want empty exact placeholder", lifecycle)
	}

	if err := RecordHostObservation(mac, "2026-09-05 11:00:00"); err != nil {
		t.Fatalf("RecordHostObservation: %v", err)
	}
	lifecycle = assertLifecycle(t, mac)
	if lifecycle.FirstSeen != "2026-09-05 11:00:00" || lifecycle.LastSeen != "2026-09-05 11:00:00" || lifecycle.FirstSeenEstimated {
		t.Fatalf("observed placeholder lifecycle = %+v, want exact scanner observation", lifecycle)
	}
}

func TestBackfillHostLifecycleUsesRetainedEvidenceAndIsIdempotent(t *testing.T) {
	startSelectTestDB(t)

	seedExportCurrentHost(t, models.Host{
		ID:   1,
		Name: "event-priority",
		Mac:  "AA:BB:CC:DD:EE:01",
		Date: "2026-09-05 09:00:00",
	})
	seedExportCurrentHost(t, models.Host{
		ID:   2,
		Name: "history-priority",
		Mac:  "AA:BB:CC:DD:EE:02",
		Date: "2026-09-05 10:00:00",
	})
	seedExportCurrentHost(t, models.Host{
		ID:   3,
		Name: "current-fallback",
		Mac:  "AA:BB:CC:DD:EE:03",
		Date: "2026-09-05 11:00:00",
	})
	seedExportCurrentHost(t, models.Host{
		ID:   4,
		Name: "existing-exact",
		Mac:  "AA:BB:CC:DD:EE:04",
		Date: "2026-09-05 12:00:00",
	})

	seedLifecycleTestEvent(t, models.Host{ID: 1, Mac: "AA:BB:CC:DD:EE:01"}, "2026-09-01 08:00:00")
	if err := UpdateWithError("history", models.Host{Mac: "AA:BB:CC:DD:EE:01", Date: "2026-08-31 08:00:00"}); err != nil {
		t.Fatalf("seed history 1: %v", err)
	}
	if err := UpdateWithError("history", models.Host{Mac: "AA:BB:CC:DD:EE:02", Date: "2026-09-02 08:00:00"}); err != nil {
		t.Fatalf("seed history 2: %v", err)
	}
	if err := db.Table(hostLifecycleTable).Create(&models.HostLifecycle{
		Mac:                "AA:BB:CC:DD:EE:04",
		FirstSeen:          "2026-08-01 00:00:00",
		LastSeen:           "2026-08-02 00:00:00",
		FirstSeenEstimated: false,
	}).Error; err != nil {
		t.Fatalf("seed existing lifecycle: %v", err)
	}

	if err := backfillHostLifecycle(db); err != nil {
		t.Fatalf("backfillHostLifecycle first: %v", err)
	}
	if err := backfillHostLifecycle(db); err != nil {
		t.Fatalf("backfillHostLifecycle second: %v", err)
	}

	assertLifecycleValues(t, "AA:BB:CC:DD:EE:01", "2026-09-01 08:00:00", "2026-09-05 09:00:00", true)
	assertLifecycleValues(t, "AA:BB:CC:DD:EE:02", "2026-09-02 08:00:00", "2026-09-05 10:00:00", true)
	assertLifecycleValues(t, "AA:BB:CC:DD:EE:03", "2026-09-05 11:00:00", "2026-09-05 11:00:00", true)
	assertLifecycleValues(t, "AA:BB:CC:DD:EE:04", "2026-08-01 00:00:00", "2026-08-02 00:00:00", false)

	var count int64
	if err := db.Table(hostLifecycleTable).Count(&count).Error; err != nil {
		t.Fatalf("count lifecycle rows: %v", err)
	}
	if count != 4 {
		t.Fatalf("lifecycle rows = %d, want 4", count)
	}
}

func TestDeleteCurrentHostWithMetadataRemovesLifecycle(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:   1,
		Name: "router",
		Mac:  "AA:BB:CC:DD:EE:01",
		Date: "2026-09-05 09:00:00",
	}
	seedExportCurrentHost(t, host)
	if err := RecordHostObservation(host.Mac, host.Date); err != nil {
		t.Fatalf("RecordHostObservation: %v", err)
	}

	if err := DeleteCurrentHostWithMetadata(host); err != nil {
		t.Fatalf("DeleteCurrentHostWithMetadata: %v", err)
	}

	if _, ok, err := SelectHostLifecycleByMAC(host.Mac); err != nil {
		t.Fatalf("SelectHostLifecycleByMAC: %v", err)
	} else if ok {
		t.Fatal("host lifecycle still exists after explicit host delete")
	}
}

func TestDeleteCurrentHostWithMetadataRollsBackOnLifecycleFailure(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:         1,
		Name:       "camera",
		IP:         "192.168.1.60",
		Mac:        "AA:BB:CC:DD:EE:60",
		Known:      1,
		Now:        1,
		DeviceType: "camera",
		Date:       "2026-09-05 09:00:00",
	}
	seedExportCurrentHost(t, host)
	if err := RecordHostObservation(host.Mac, host.Date); err != nil {
		t.Fatalf("RecordHostObservation: %v", err)
	}
	owner := "Facilities"
	if _, err := UpsertHostMetadata(host.Mac, models.HostMetadataUpdate{Owner: &owner}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}
	seedLifecycleTestEvent(t, host, "2026-09-05 09:01:00")

	if err := db.Exec(`
		CREATE TRIGGER fail_host_lifecycle_delete
		BEFORE DELETE ON host_lifecycle
		BEGIN
			SELECT RAISE(ABORT, 'forced lifecycle delete failure');
		END;
	`).Error; err != nil {
		t.Fatalf("create lifecycle delete failure trigger: %v", err)
	}

	if err := DeleteCurrentHostWithMetadata(host); err == nil {
		t.Fatal("DeleteCurrentHostWithMetadata error = nil, want forced lifecycle delete failure")
	}
	if got := SelectByID(host.ID); got.ID != host.ID {
		t.Fatalf("host was deleted despite lifecycle failure: %+v", got)
	}
	assertLifecycleValues(t, host.Mac, host.Date, host.Date, false)
	assertMetadataStillExistsInGDB(t, host.Mac, owner)
	if events, ok := SelectEvents(10, ""); !ok || len(events) != 1 || events[0].EventType != string(models.EventDiscovered) {
		t.Fatalf("events after rollback = %+v, ok=%v, want discovered event preserved", events, ok)
	}
}

func seedLifecycleTestEvent(t *testing.T, host models.Host, date string) {
	t.Helper()

	event := models.NewHostEvent(host, models.EventDiscovered, "", "")
	event.Date = date
	if err := AddEvent(event); err != nil {
		t.Fatalf("AddEvent discovered: %v", err)
	}
}

func assertLifecycle(t *testing.T, mac string) models.HostLifecycle {
	t.Helper()

	lifecycle, ok, err := SelectHostLifecycleByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostLifecycleByMAC: %v", err)
	}
	if !ok {
		t.Fatalf("lifecycle for %s not found", mac)
	}
	return lifecycle
}

func assertLifecycleValues(t *testing.T, mac, firstSeen, lastSeen string, estimated bool) {
	t.Helper()

	lifecycle := assertLifecycle(t, mac)
	if lifecycle.FirstSeen != firstSeen || lifecycle.LastSeen != lastSeen || lifecycle.FirstSeenEstimated != estimated {
		t.Fatalf("lifecycle for %s = %+v, want first=%q last=%q estimated=%v", mac, lifecycle, firstSeen, lastSeen, estimated)
	}
}

func assertMetadataStillExistsInGDB(t *testing.T, mac string, owner string) {
	t.Helper()

	metadata, ok, err := SelectHostMetadataByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC: %v", err)
	}
	if !ok || metadata.Owner != owner {
		t.Fatalf("metadata for %s = %+v, ok=%v, want owner %q", mac, metadata, ok, owner)
	}
}
