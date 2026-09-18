package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestServiceHintsPopulateNewObservationsAndBackfillEmptyRows(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "service-hints.db"),
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

	stored, _, _, err := RecordServiceObservation(models.Service{
		Mac:            "AA:BB:CC:DD:EE:E0",
		Address:        "192.168.1.100",
		Protocol:       "tcp",
		Port:           443,
		State:          "open",
		LastScanSource: "manual",
	}, "2026-09-18 12:00:00")
	if err != nil {
		t.Fatalf("RecordServiceObservation: %v", err)
	}
	if stored.ServiceHint != "HTTPS" {
		t.Fatalf("new observation hint = %q, want HTTPS", stored.ServiceHint)
	}

	legacy := models.Service{
		Mac:            "AA:BB:CC:DD:EE:E1",
		Address:        "192.168.1.101",
		AddressFamily:  "ipv4",
		Protocol:       "tcp",
		Port:           22,
		State:          "open",
		FirstDetected:  "2026-09-18 11:00:00",
		LastDetected:   "2026-09-18 11:00:00",
		LastChecked:    "2026-09-18 11:00:00",
		StateChangedAt: "2026-09-18 11:00:00",
		ServiceHint:    "",
		LastScanSource: "manual",
	}
	if err := db.Table(servicesTable).Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy service: %v", err)
	}
	if err := backfillServiceHints(db); err != nil {
		t.Fatalf("backfillServiceHints: %v", err)
	}

	selected, found, err := SelectServiceByIdentity(legacy.Mac, legacy.Address, legacy.Protocol, legacy.Port)
	if err != nil || !found {
		t.Fatalf("SelectServiceByIdentity found=%v err=%v", found, err)
	}
	if selected.ServiceHint != "SSH" {
		t.Fatalf("backfilled hint = %q, want SSH", selected.ServiceHint)
	}

	custom := models.Service{
		Mac:            "AA:BB:CC:DD:EE:E2",
		Address:        "192.168.1.102",
		AddressFamily:  "ipv4",
		Protocol:       "tcp",
		Port:           443,
		State:          "open",
		FirstDetected:  "2026-09-18 11:00:00",
		LastDetected:   "2026-09-18 11:00:00",
		LastChecked:    "2026-09-18 11:00:00",
		StateChangedAt: "2026-09-18 11:00:00",
		ServiceHint:    "Custom gateway",
		LastScanSource: "manual",
	}
	if err := db.Table(servicesTable).Create(&custom).Error; err != nil {
		t.Fatalf("create custom service: %v", err)
	}
	if err := backfillServiceHints(db); err != nil {
		t.Fatalf("second backfillServiceHints: %v", err)
	}
	selected, found, err = SelectServiceByIdentity(custom.Mac, custom.Address, custom.Protocol, custom.Port)
	if err != nil || !found {
		t.Fatalf("Select custom found=%v err=%v", found, err)
	}
	if selected.ServiceHint != "Custom gateway" {
		t.Fatalf("custom hint overwritten: %q", selected.ServiceHint)
	}
}
