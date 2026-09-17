package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestServiceInventoryMigrationPreservesLegacyHosts(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	dbPath := filepath.Join(t.TempDir(), "legacy-services.db")

	legacyDB := openMigrationFixtureDB(t, dbPath)
	createLegacyHostTable(t, legacyDB, "now")
	createLegacyHostTable(t, legacyDB, "history")
	insertLegacyHost(t, legacyDB, "now", 1, "nas", "nas.lan", "eth0", "192.168.1.5", "AA:BB:CC:DD:EE:45", "NAS Vendor", "2026-09-18 08:00:00", 1, 1)
	insertLegacyHost(t, legacyDB, "history", 10, "nas", "nas.lan", "eth0", "192.168.1.5", "AA:BB:CC:DD:EE:45", "NAS Vendor", "2026-09-18 07:00:00", 1, 1)
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
		t.Fatalf("StartErr: %v", err)
	}
	if !db.Migrator().HasTable(servicesTable) {
		t.Fatal("services table missing after legacy migration")
	}
	if !db.Migrator().HasTable(serviceScanSettingsTable) {
		t.Fatal("service_scan_settings table missing after legacy migration")
	}

	hosts := SelectByMAC("now", "AA:BB:CC:DD:EE:45")
	if len(hosts) != 1 {
		t.Fatalf("legacy host count = %d, want 1", len(hosts))
	}
	if hosts[0].Name != "nas" || hosts[0].IP != "192.168.1.5" || hosts[0].Known != 1 || hosts[0].Now != 1 {
		t.Fatalf("legacy host changed during service migration: %+v", hosts[0])
	}

	var serviceCount int64
	if err := db.Table(servicesTable).Count(&serviceCount).Error; err != nil {
		t.Fatalf("count services: %v", err)
	}
	if serviceCount != 0 {
		t.Fatalf("service migration invented %d service rows", serviceCount)
	}
}
