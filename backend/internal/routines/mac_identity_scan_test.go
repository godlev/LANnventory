package routines

import (
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestProcessScanResultMatchesEquivalentMACRepresentation(t *testing.T) {
	setupScanRoutineTest(t)

	gdb.Update("now", models.Host{
		Name:  "existing-device",
		Iface: "eth0",
		IP:    "192.168.1.10",
		Mac:   "AA:BB:CC:DD:EE:10",
		Hw:    "Existing Vendor",
		Date:  "2026-09-17 09:00:00",
		Known: 1,
		Now:   1,
	})

	seeded := gdb.SelectByMAC("now", "AA:BB:CC:DD:EE:10")
	if len(seeded) != 1 {
		t.Fatalf("seeded hosts len = %d, want 1", len(seeded))
	}

	processScanResult([]models.Host{
		{
			Iface: "wifi0",
			IP:    "192.168.1.11",
			Mac:   "aa-bb-cc-dd-ee-10",
			Hw:    "Scanned Vendor",
			Date:  "2026-09-17 09:05:00",
			Now:   1,
		},
	}, true)

	hosts, ok := gdb.Select("now")
	if !ok {
		t.Fatal("Select now failed")
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts len = %d, want one existing identity: %+v", len(hosts), hosts)
	}

	updated := hosts[0]
	if updated.ID != seeded[0].ID {
		t.Fatalf("host ID = %d, want existing ID %d", updated.ID, seeded[0].ID)
	}
	if updated.Mac != "AA:BB:CC:DD:EE:10" {
		t.Fatalf("stored legacy MAC changed to %q, want preserved AA:BB:CC:DD:EE:10", updated.Mac)
	}
	if updated.IP != "192.168.1.11" || updated.Iface != "wifi0" || updated.Now != 1 {
		t.Fatalf("scanner fields not refreshed: %+v", updated)
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 0 {
		t.Fatalf("equivalent MAC representation created lifecycle events: %+v", events)
	}
}

func TestProcessScanResultCanonicalizesNewValidMAC(t *testing.T) {
	setupScanRoutineTest(t)

	processScanResult([]models.Host{
		{
			Iface: "eth0",
			IP:    "127.0.0.1",
			Mac:   "00-aa-bb-cc-dd-ee",
			Hw:    "New Device Vendor",
			Date:  "2026-09-17 10:00:00",
			Now:   1,
		},
	}, true)

	hosts, ok := gdb.Select("now")
	if !ok {
		t.Fatal("Select now failed")
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts len = %d, want 1", len(hosts))
	}
	if hosts[0].Mac != "00:AA:BB:CC:DD:EE" {
		t.Fatalf("new host MAC = %q, want canonical 00:AA:BB:CC:DD:EE", hosts[0].Mac)
	}
}
