package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestRecordServiceObservationMaintainsSummaryWithoutClosedPortExplosion(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "service-observations.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	base := models.Service{
		Mac:            "AA:BB:CC:DD:EE:48",
		Address:        "192.168.1.48",
		Protocol:       "tcp",
		Port:           22,
		LastScanSource: "manual",
	}

	closed := base
	closed.State = string(models.ServiceStateClosed)
	_, persisted, changed, err := RecordServiceObservation(closed, "2026-09-18 10:00:00")
	if err != nil {
		t.Fatalf("RecordServiceObservation unseen closed: %v", err)
	}
	if persisted || changed {
		t.Fatalf("unseen closed persisted=%v changed=%v, want both false", persisted, changed)
	}

	open := base
	open.State = string(models.ServiceStateOpen)
	first, persisted, changed, err := RecordServiceObservation(open, "2026-09-18 10:05:00")
	if err != nil {
		t.Fatalf("RecordServiceObservation first open: %v", err)
	}
	if !persisted || !changed {
		t.Fatalf("first open persisted=%v changed=%v, want both true", persisted, changed)
	}
	if first.FirstDetected != "2026-09-18 10:05:00" || first.LastDetected != "2026-09-18 10:05:00" || first.LastChecked != "2026-09-18 10:05:00" {
		t.Fatalf("first open summary = %+v", first)
	}

	second, persisted, changed, err := RecordServiceObservation(open, "2026-09-18 10:10:00")
	if err != nil {
		t.Fatalf("RecordServiceObservation repeated open: %v", err)
	}
	if !persisted || changed {
		t.Fatalf("repeated open persisted=%v changed=%v, want true/false", persisted, changed)
	}
	if second.FirstDetected != first.FirstDetected || second.LastDetected != "2026-09-18 10:10:00" || second.StateChangedAt != first.StateChangedAt {
		t.Fatalf("repeated open summary = %+v", second)
	}

	closed.LastScanSource = "scheduled"
	third, persisted, changed, err := RecordServiceObservation(closed, "2026-09-18 10:20:00")
	if err != nil {
		t.Fatalf("RecordServiceObservation close: %v", err)
	}
	if !persisted || !changed {
		t.Fatalf("close persisted=%v changed=%v, want both true", persisted, changed)
	}
	if third.State != "closed" || third.LastDetected != "2026-09-18 10:10:00" || third.LastChecked != "2026-09-18 10:20:00" || third.StateChangedAt != "2026-09-18 10:20:00" {
		t.Fatalf("closed summary = %+v", third)
	}

	fourth, persisted, changed, err := RecordServiceObservation(open, "2026-09-18 10:30:00")
	if err != nil {
		t.Fatalf("RecordServiceObservation reopen: %v", err)
	}
	if !persisted || !changed {
		t.Fatalf("reopen persisted=%v changed=%v, want both true", persisted, changed)
	}
	if fourth.State != "open" || fourth.FirstDetected != first.FirstDetected || fourth.LastDetected != "2026-09-18 10:30:00" || fourth.StateChangedAt != "2026-09-18 10:30:00" {
		t.Fatalf("reopened summary = %+v", fourth)
	}

	services, err := SelectServicesByMAC(base.Mac)
	if err != nil {
		t.Fatalf("SelectServicesByMAC: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("services len = %d, want one durable summary: %+v", len(services), services)
	}
}
