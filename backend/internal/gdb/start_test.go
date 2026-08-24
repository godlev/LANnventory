package gdb

import (
	"path/filepath"
	"testing"

	"github.com/aceberg/WatchYourLAN/internal/conf"
	"github.com/aceberg/WatchYourLAN/internal/models"
	sqlite "github.com/aceberg/gorm-sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestStartMigratesLegacySQLiteSchemaWithoutChangingRows(t *testing.T) {
	oldConfig := conf.AppConfig
	dbPath := filepath.Join(t.TempDir(), "legacy-watchyourlan.db")

	legacyDB := openMigrationFixtureDB(t, dbPath)
	createLegacyHostTable(t, legacyDB, "now")
	createLegacyHostTable(t, legacyDB, "history")
	insertLegacyHost(t, legacyDB, "now", 1, "router", "router.lan", "eth0", "192.168.1.1", "AA:BB:CC:DD:EE:01", "Gateway Vendor", "2026-08-24 08:00:00", 1, 1)
	insertLegacyHost(t, legacyDB, "now", 2, "unknown", "", "wifi0", "192.168.1.50", "AA:BB:CC:DD:EE:50", "Mobile Vendor", "2026-08-24 07:55:00", 0, 0)
	insertLegacyHost(t, legacyDB, "history", 10, "router", "router.lan", "eth0", "192.168.1.1", "AA:BB:CC:DD:EE:01", "Gateway Vendor", "2026-08-24 07:00:00", 1, 1)
	insertLegacyHost(t, legacyDB, "history", 11, "unknown", "", "wifi0", "192.168.1.50", "AA:BB:CC:DD:EE:50", "Mobile Vendor", "2026-08-24 07:05:00", 0, 0)
	closeFixtureDB(t, legacyDB)

	conf.AppConfig = models.Conf{
		UseDB:  "sqlite",
		DBPath: dbPath,
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.AppConfig = oldConfig
	})

	Start()
	assertMigratedLegacyRows(t)
	assertNoInventedEvents(t)

	if err := Close(); err != nil {
		t.Fatalf("Close before idempotent restart: %v", err)
	}

	Start()
	assertMigratedLegacyRows(t)
	assertNoInventedEvents(t)
}

func openMigrationFixtureDB(t *testing.T, dbPath string) *gorm.DB {
	t.Helper()

	fixtureDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			NoLowerCase: true,
		},
	})
	if err != nil {
		t.Fatalf("open migration fixture DB: %v", err)
	}

	return fixtureDB
}

func closeFixtureDB(t *testing.T, fixtureDB *gorm.DB) {
	t.Helper()

	sqlDB, err := fixtureDB.DB()
	if err != nil {
		t.Fatalf("fixture DB handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close fixture DB: %v", err)
	}
}

func createLegacyHostTable(t *testing.T, fixtureDB *gorm.DB, table string) {
	t.Helper()

	if err := fixtureDB.Exec(`
		CREATE TABLE ` + table + ` (
			ID integer PRIMARY KEY AUTOINCREMENT,
			NAME text,
			DNS text,
			IFACE text,
			IP text,
			MAC text,
			HW text,
			DATE text,
			KNOWN integer,
			NOW integer
		)
	`).Error; err != nil {
		t.Fatalf("create legacy %s table: %v", table, err)
	}
}

func insertLegacyHost(t *testing.T, fixtureDB *gorm.DB, table string, id int, name, dns, iface, ip, mac, hw, date string, known, now int) {
	t.Helper()

	if err := fixtureDB.Exec(`
		INSERT INTO `+table+` (ID, NAME, DNS, IFACE, IP, MAC, HW, DATE, KNOWN, NOW)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, dns, iface, ip, mac, hw, date, known, now).Error; err != nil {
		t.Fatalf("insert legacy %s row %d: %v", table, id, err)
	}
}

func assertMigratedLegacyRows(t *testing.T) {
	t.Helper()

	for _, table := range []string{"now", "history"} {
		if !db.Table(table).Migrator().HasColumn(&models.Host{}, "DEVICE_TYPE") {
			t.Fatalf("%s table missing DEVICE_TYPE after startup migration", table)
		}
	}
	if !db.Migrator().HasTable("events") {
		t.Fatal("events table missing after startup migration")
	}

	var hosts []models.Host
	if err := db.Table("now").Order("\"ID\" ASC").Find(&hosts).Error; err != nil {
		t.Fatalf("read migrated now rows: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("now rows len = %d, want 2: %+v", len(hosts), hosts)
	}
	assertHostRow(t, hosts[0], 1, "router", "router.lan", "eth0", "192.168.1.1", "AA:BB:CC:DD:EE:01", "Gateway Vendor", "2026-08-24 08:00:00", 1, 1)
	assertHostRow(t, hosts[1], 2, "unknown", "", "wifi0", "192.168.1.50", "AA:BB:CC:DD:EE:50", "Mobile Vendor", "2026-08-24 07:55:00", 0, 0)

	var history []models.Host
	if err := db.Table("history").Order("\"ID\" ASC").Find(&history).Error; err != nil {
		t.Fatalf("read migrated history rows: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history rows len = %d, want 2: %+v", len(history), history)
	}
	assertHostRow(t, history[0], 10, "router", "router.lan", "eth0", "192.168.1.1", "AA:BB:CC:DD:EE:01", "Gateway Vendor", "2026-08-24 07:00:00", 1, 1)
	assertHostRow(t, history[1], 11, "unknown", "", "wifi0", "192.168.1.50", "AA:BB:CC:DD:EE:50", "Mobile Vendor", "2026-08-24 07:05:00", 0, 0)
}

func assertHostRow(t *testing.T, host models.Host, id int, name, dns, iface, ip, mac, hw, date string, known, now int) {
	t.Helper()

	if host.ID != id ||
		host.Name != name ||
		host.DNS != dns ||
		host.Iface != iface ||
		host.IP != ip ||
		host.Mac != mac ||
		host.Hw != hw ||
		host.Date != date ||
		host.Known != known ||
		host.Now != now ||
		host.DeviceType != "" {
		t.Fatalf("host row = %+v, want unchanged legacy row with empty DeviceType", host)
	}
}

func assertNoInventedEvents(t *testing.T) {
	t.Helper()

	events, ok := SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed after migration")
	}
	if len(events) != 0 {
		t.Fatalf("events len = %d, want no invented historical events: %+v", len(events), events)
	}
}
