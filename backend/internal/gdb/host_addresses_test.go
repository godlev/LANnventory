package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestHostAddressMigrationCreatesSeparateTableOnly(t *testing.T) {
	startSelectTestDB(t)

	if !db.Migrator().HasTable(hostAddressesTable) {
		t.Fatal("host_addresses table was not migrated")
	}
	for _, table := range []string{"now", "history"} {
		for _, column := range []string{"ADDRESS", "FAMILY", "FIRST_SEEN", "LAST_SEEN", "ACTIVE"} {
			if db.Table(table).Migrator().HasColumn(&models.Host{}, column) {
				t.Fatalf("%s unexpectedly has host-address column %s", table, column)
			}
		}
}

func TestRecordHostAddressObservationsTracksMultipleAddressesAndActivity(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:10"

	if err := RecordHostAddressObservations([]models.Host{
		{Mac: "aa-bb-cc-dd-ee-10", IP: "192.168.1.10", Iface: "eth0", Date: "2026-09-17 10:00:00"},
		{Mac: mac, IP: "192.168.1.11", Iface: "eth0", Date: "2026-09-17 10:00:00"},
	}); err != nil {
		t.Fatalf("RecordHostAddressObservations first scan: %v", err)
	}

	rows, err := SelectHostAddressesByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostAddressesByMAC: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if !row.Active || row.FirstSeen != "2026-09-17 10:00:00" || row.LastSeen != "2026-09-17 10:00:00" {
			t.Fatalf("initial observation = %+v, want active exact first/last", row)
		}
		if row.Mac != mac || row.Family != "ipv4" {
			t.Fatalf("normalized observation = %+v", row)
		}
	}

	if err := RecordHostAddressObservations([]models.Host{
		{Mac: mac, IP: "192.168.1.11", Iface: "wifi0", Date: "2026-09-17 10:05:00"},
	}); err != nil {
		t.Fatalf("RecordHostAddressObservations second scan: %v", err)
	}

	rows, err = SelectHostAddressesByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostAddressesByMAC second: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows len after second scan = %d, want 2: %+v", len(rows), rows)
	}

	byAddress := make(map[string]models.HostAddress, len(rows))
	for _, row := range rows {
		byAddress[row.Address] = row
	}
	old := byAddress["192.168.1.10"]
	if old.Active || old.LastSeen != "2026-09-17 10:00:00" {
		t.Fatalf("inactive retained address = %+v", old)
	}
	current := byAddress["192.168.1.11"]
	if !current.Active || current.FirstSeen != "2026-09-17 10:00:00" || current.LastSeen != "2026-09-17 10:05:00" || current.Iface != "wifi0" {
		t.Fatalf("current address = %+v", current)
	}
}

func TestSelectHostAddressesByAddressKeepsPreviousMACAssociations(t *testing.T) {
	startSelectTestDB(t)

	if err := RecordHostAddressObservations([]models.Host{
		{Mac: "00:11:22:33:44:55", IP: "10.4.1.27", Iface: "eth0", Date: "2026-09-17 09:00:00"},
	}); err != nil {
		t.Fatalf("first observation: %v", err)
	}
	if err := RecordHostAddressObservations([]models.Host{
		{Mac: "02:11:22:33:44:66", IP: "10.4.1.27", Iface: "eth0", Date: "2026-09-17 11:00:00"},
	}); err != nil {
		t.Fatalf("second observation: %v", err)
	}

	rows, err := SelectHostAddressesByAddress("10.4.1.27")
	if err != nil {
		t.Fatalf("SelectHostAddressesByAddress: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2: %+v", len(rows), rows)
	}
	if rows[0].Mac != "02:11:22:33:44:66" || !rows[0].Active {
		t.Fatalf("active association = %+v", rows[0])
	}
	if rows[1].Mac != "00:11:22:33:44:55" || rows[1].Active {
		t.Fatalf("historical association = %+v", rows[1])
	}
}

func TestBackfillHostAddressesUsesScannerHistoryAndIgnoresManualRows(t *testing.T) {
	startSelectTestDB(t)

	if err := db.Table("history").Create(&models.Host{
		Mac: "AA:BB:CC:DD:EE:21", IP: "10.4.1.21", Iface: "eth0", Date: "2026-09-15 08:00:00", Now: 1,
	}).Error; err != nil {
		t.Fatalf("seed scanner history first: %v", err)
	}
	if err := db.Table("history").Create(&models.Host{
		Mac: "AA:BB:CC:DD:EE:21", IP: "10.4.1.21", Iface: "eth0", Date: "2026-09-16 09:00:00", Now: 1,
	}).Error; err != nil {
		t.Fatalf("seed scanner history second: %v", err)
	}
	if err := db.Table("now").Create(&models.Host{
		Mac: "AA:BB:CC:DD:EE:21", IP: "10.4.1.21", Iface: "eth0", Date: "2026-09-16 09:00:00", Now: 1,
	}).Error; err != nil {
		t.Fatalf("seed current scanner host: %v", err)
	}
	if err := db.Table("history").Create(&models.Host{
		Mac: "AA:BB:CC:DD:EE:22", IP: "10.4.1.22", Iface: "", Date: "2026-09-15 08:00:00", Now: 1,
	}).Error; err != nil {
		t.Fatalf("seed manual history: %v", err)
	}
	if err := db.Table("now").Create(&models.Host{
		Mac: "AA:BB:CC:DD:EE:22", IP: "10.4.1.22", Iface: "", Date: "", Now: 0,
	}).Error; err != nil {
		t.Fatalf("seed manual current host: %v", err)
	}

	if err := backfillHostAddresses(db); err != nil {
		t.Fatalf("backfillHostAddresses first: %v", err)
	}
	if err := backfillHostAddresses(db); err != nil {
		t.Fatalf("backfillHostAddresses second: %v", err)
	}

	rows, err := SelectHostAddressesByMAC("AA:BB:CC:DD:EE:21")
	if err != nil {
		t.Fatalf("Select scanner backfill: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("scanner rows len = %d, want 1: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.FirstSeen != "2026-09-15 08:00:00" || row.LastSeen != "2026-09-16 09:00:00" || !row.Active || row.Iface != "eth0" {
		t.Fatalf("scanner backfill = %+v", row)
	}

	manualRows, err := SelectHostAddressesByMAC("AA:BB:CC:DD:EE:22")
	if err != nil {
		t.Fatalf("Select manual backfill: %v", err)
	}
	if len(manualRows) != 0 {
		t.Fatalf("manual host created scanner address history: %+v", manualRows)
	}

	var count int64
	if err := db.Table(hostAddressesTable).Count(&count).Error; err != nil {
		t.Fatalf("count host_addresses: %v", err)
	}
	if count != 1 {
		t.Fatalf("host_addresses rows = %d, want 1 after idempotent backfill", count)
	}
}
