package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestStartBackfillsLegacyMultiIPAddressHistoryIdempotently(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	dbPath := filepath.Join(t.TempDir(), "legacy-multi-ip.db")

	legacyDB := openMigrationFixtureDB(t, dbPath)
	createLegacyHostTable(t, legacyDB, "now")
	createLegacyHostTable(t, legacyDB, "history")
	insertLegacyHost(t, legacyDB, "history", 1, "device", "", "eth0", "192.168.1.10", "AA:BB:CC:DD:EE:60", "Vendor", "2026-09-15 07:00:00", 1, 1)
	insertLegacyHost(t, legacyDB, "history", 2, "device", "", "eth0", "192.168.1.11", "AA:BB:CC:DD:EE:60", "Vendor", "2026-09-16 08:00:00", 1, 1)
	insertLegacyHost(t, legacyDB, "now", 1, "device", "", "eth0", "192.168.1.11", "AA:BB:CC:DD:EE:60", "Vendor", "2026-09-17 09:00:00", 1, 1)
	closeFixtureDB(t, legacyDB)

	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: dbPath,
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr first migration: %v", err)
	}
	assertLegacyMultiIPAddressRows(t)

	if err := Close(); err != nil {
		t.Fatalf("Close before idempotent restart: %v", err)
	}
	if err := StartErr(); err != nil {
		t.Fatalf("StartErr second migration: %v", err)
	}
	assertLegacyMultiIPAddressRows(t)
}

func assertLegacyMultiIPAddressRows(t *testing.T) {
	t.Helper()

	if !db.Migrator().HasTable(hostAddressesTable) {
		t.Fatal("host_addresses table missing after startup migration")
	}

	rows, err := SelectHostAddressesByMAC("AA:BB:CC:DD:EE:60")
	if err != nil {
		t.Fatalf("SelectHostAddressesByMAC: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("address rows len = %d, want 2 after idempotent migration: %+v", len(rows), rows)
	}

	active := rows[0]
	if active.Address != "192.168.1.11" || !active.Active || active.Family != "ipv4" || active.Iface != "eth0" || active.FirstSeen != "2026-09-16 08:00:00" || active.LastSeen != "2026-09-17 09:00:00" {
		t.Fatalf("active address = %+v, want current IP with history first-seen and current last-seen", active)
	}

	previous := rows[1]
	if previous.Address != "192.168.1.10" || previous.Active || previous.Family != "ipv4" || previous.FirstSeen != "2026-09-15 07:00:00" || previous.LastSeen != "2026-09-15 07:00:00" {
		t.Fatalf("previous address = %+v, want retained inactive historical IP", previous)
	}

	var count int64
	if err := db.Table(hostAddressesTable).Count(&count).Error; err != nil {
		t.Fatalf("count host_addresses: %v", err)
	}
	if count != 2 {
		t.Fatalf("host_addresses count = %d, want 2", count)
	}
}
