package gdb

import (
	"database/sql"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestExportTransactionOptionsPostgresUsesRepeatableReadReadOnly(t *testing.T) {
	options := exportTransactionOptions("postgres")
	if options == nil {
		t.Fatal("postgres options = nil, want transaction options")
	}
	if options.Isolation != sql.LevelRepeatableRead {
		t.Fatalf("Isolation = %v, want %v", options.Isolation, sql.LevelRepeatableRead)
	}
	if !options.ReadOnly {
		t.Fatal("ReadOnly = false, want true")
	}
}

func TestExportTransactionOptionsSQLiteUsesDefaultTransaction(t *testing.T) {
	if options := exportTransactionOptions("sqlite"); options != nil {
		t.Fatalf("sqlite options = %+v, want nil/default transaction behavior", options)
	}
}

func TestExportTransactionOptionsUnknownDialectUsesDefaultTransaction(t *testing.T) {
	if options := exportTransactionOptions("mysql"); options != nil {
		t.Fatalf("unknown dialect options = %+v, want nil/default transaction behavior", options)
	}
}

func TestExportCurrentHostsOrdersByIDAscending(t *testing.T) {
	startSelectTestDB(t)

	seedExportCurrentHost(t, models.Host{
		ID:    2,
		Name:  "nas",
		Mac:   "AA:BB:CC:DD:EE:02",
		IP:    "192.168.1.20",
		Known: 1,
		Now:   1,
	})
	seedExportCurrentHost(t, models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		IP:    "192.168.1.1",
		Known: 1,
		Now:   1,
	})

	hosts, err := ExportCurrentHosts()
	if err != nil {
		t.Fatalf("ExportCurrentHosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("hosts len = %d, want 2: %+v", len(hosts), hosts)
	}
	if hosts[0].ID != 1 || hosts[1].ID != 2 {
		t.Fatalf("host IDs = [%d, %d], want [1, 2]: %+v", hosts[0].ID, hosts[1].ID, hosts)
	}
}

func TestExportCurrentHostsDoesNotRequireHistoryOrEventsTables(t *testing.T) {
	startSelectTestDB(t)

	seedExportCurrentHost(t, models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		IP:    "192.168.1.1",
		Known: 1,
		Now:   1,
	})

	if err := db.Migrator().DropTable("history", "events"); err != nil {
		t.Fatalf("DropTable(history, events): %v", err)
	}

	hosts, err := ExportCurrentHosts()
	if err != nil {
		t.Fatalf("ExportCurrentHosts without history/events tables: %v", err)
	}
	if len(hosts) != 1 || hosts[0].ID != 1 || hosts[0].Name != "router" {
		t.Fatalf("hosts = %+v, want only current router host", hosts)
	}
}

func seedExportCurrentHost(t *testing.T, host models.Host) {
	t.Helper()

	if err := UpdateWithError("now", host); err != nil {
		t.Fatalf("UpdateWithError now: %v", err)
	}
}
