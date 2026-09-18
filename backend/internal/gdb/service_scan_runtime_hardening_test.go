package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestUpdateServiceScanRuntimeIfCurrentRejectsStaleConfiguration(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "service-runtime-hardening.db"),
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

	expected, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             "AA:BB:CC:DD:EE:F0",
		Enabled:         true,
		IntervalMinutes: 60,
		PortsJSON:       "[22]",
		NextScanAt:      "2026-09-18 10:00:00",
	})
	if err != nil {
		t.Fatalf("Upsert expected settings: %v", err)
	}

	current, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             expected.Mac,
		Enabled:         true,
		IntervalMinutes: 120,
		PortsJSON:       "[443]",
		NextScanAt:      "2026-09-18 11:05:00",
	})
	if err != nil {
		t.Fatalf("Upsert changed settings: %v", err)
	}

	updated, err := UpdateServiceScanRuntimeIfCurrent(
		expected,
		"2026-09-18 12:00:00",
		"2026-09-18 11:00:00",
		"stale result",
		true,
	)
	if err != nil {
		t.Fatalf("UpdateServiceScanRuntimeIfCurrent stale: %v", err)
	}
	if updated {
		t.Fatal("stale runtime update unexpectedly matched changed configuration")
	}

	stored, found, err := SelectServiceScanSettingsByMAC(expected.Mac)
	if err != nil || !found {
		t.Fatalf("Select stale-protected settings found=%v err=%v", found, err)
	}
	if stored.IntervalMinutes != current.IntervalMinutes ||
		stored.PortsJSON != current.PortsJSON ||
		stored.NextScanAt != current.NextScanAt ||
		stored.LastAttemptAt != "" ||
		stored.LastSuccessfulAt != "" ||
		stored.LastError != "" {
		t.Fatalf("stale update changed current settings: %+v", stored)
	}

	updated, err = UpdateServiceScanRuntimeIfCurrent(
		current,
		"2026-09-18 13:05:00",
		"2026-09-18 11:05:00",
		"",
		true,
	)
	if err != nil {
		t.Fatalf("UpdateServiceScanRuntimeIfCurrent current: %v", err)
	}
	if !updated {
		t.Fatal("current runtime update did not match")
	}

	stored, found, err = SelectServiceScanSettingsByMAC(expected.Mac)
	if err != nil || !found {
		t.Fatalf("Select current settings found=%v err=%v", found, err)
	}
	if stored.NextScanAt != "2026-09-18 13:05:00" ||
		stored.LastAttemptAt != "2026-09-18 11:05:00" ||
		stored.LastSuccessfulAt != "2026-09-18 11:05:00" {
		t.Fatalf("current runtime update = %+v", stored)
	}
}
