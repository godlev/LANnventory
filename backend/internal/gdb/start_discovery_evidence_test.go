package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestStartCreatesDiscoveryEvidenceTableWithoutInventingLegacyProvenance(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	dbPath := filepath.Join(t.TempDir(), "legacy-discovery-evidence.db")

	legacyDB := openMigrationFixtureDB(t, dbPath)
	createLegacyHostTable(t, legacyDB, "now")
	createLegacyHostTable(t, legacyDB, "history")
	insertLegacyHost(t, legacyDB, "now", 1, "Living Room TV", "bravia.home", "eth0", "192.168.1.40", "AA:BB:CC:DD:EE:40", "Sony", "2026-09-17 08:00:00", 1, 1)
	insertLegacyHost(t, legacyDB, "history", 1, "Living Room TV", "bravia.home", "eth0", "192.168.1.40", "AA:BB:CC:DD:EE:40", "Sony", "2026-09-16 08:00:00", 1, 1)
	closeFixtureDB(t, legacyDB)

	conf.SetAppConfigForTest(models.Conf{UseDB: "sqlite", DBPath: dbPath})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr first migration: %v", err)
	}
	assertNoInventedDiscoveryEvidence(t)

	hosts := SelectByMAC("now", "AA:BB:CC:DD:EE:40")
	if len(hosts) != 1 || hosts[0].Name != "Living Room TV" || hosts[0].DNS != "bravia.home" || hosts[0].Hw != "Sony" {
		t.Fatalf("legacy host inventory/discovery fields changed during evidence migration: %+v", hosts)
	}

	if err := Close(); err != nil {
		t.Fatalf("Close before idempotent restart: %v", err)
	}
	if err := StartErr(); err != nil {
		t.Fatalf("StartErr second migration: %v", err)
	}
	assertNoInventedDiscoveryEvidence(t)
}

func assertNoInventedDiscoveryEvidence(t *testing.T) {
	t.Helper()

	if !db.Migrator().HasTable(hostDiscoveryEvidenceTable) {
		t.Fatal("host_discovery_evidence table missing after startup migration")
	}
	for _, column := range []string{"MAC", "ADDRESS", "SOURCE", "KIND", "VALUE", "FIRST_SEEN", "LAST_SEEN", "ACTIVE"} {
		if !db.Table(hostDiscoveryEvidenceTable).Migrator().HasColumn(&models.HostDiscoveryEvidence{}, column) {
			t.Fatalf("host_discovery_evidence missing %s column", column)
		}
	}

	var count int64
	if err := db.Table(hostDiscoveryEvidenceTable).Count(&count).Error; err != nil {
		t.Fatalf("count host_discovery_evidence: %v", err)
	}
	if count != 0 {
		t.Fatalf("legacy migration invented %d discovery evidence rows", count)
	}
}
