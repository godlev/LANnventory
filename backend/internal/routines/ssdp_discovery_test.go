package routines

import (
	"context"
	"testing"

	"github.com/godlev/LANnventory/internal/discovery"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestProcessScanResultPersistsSSDPEvidenceWithoutOverwritingManualName(t *testing.T) {
	setupScanRoutineTest(t)
	oldDiscovery := ssdpDiscovery
	calls := 0
	ssdpDiscovery = func(_ context.Context, addresses []string) []discovery.SSDPObservation {
		calls++
		if len(addresses) != 1 || addresses[0] != "192.168.1.80" {
			t.Fatalf("SSDP addresses = %v, want [192.168.1.80]", addresses)
		}
		return []discovery.SSDPObservation{
			{Address: "192.168.1.80", Kind: models.DiscoveryKindFriendlyName, Values: []string{"Sony BRAVIA XR"}},
			{Address: "192.168.1.80", Kind: models.DiscoveryKindManufacturer, Values: []string{"Sony"}},
			{Address: "192.168.1.80", Kind: models.DiscoveryKindModel, Values: []string{"BRAVIA XR"}},
			{Address: "192.168.1.80", Kind: models.DiscoveryKindModelNumber, Values: []string{"XR-55X90L"}},
		}
	}
	t.Cleanup(func() { ssdpDiscovery = oldDiscovery })

	mac := "AA:BB:CC:DD:EE:80"
	gdb.Update("now", models.Host{
		Name:  "Living Room TV",
		Iface: "eth0",
		IP:    "192.168.1.80",
		Mac:   mac,
		Hw:    "Sony",
		Date:  "2026-09-17 15:00:00",
		Known: 1,
		Now:   1,
	})

	if !processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.80", Mac: mac, Hw: "Sony", Date: "2026-09-17 15:30:00", Now: 1},
	}, true) {
		t.Fatal("successful scan was not applied")
	}
	if calls != 1 {
		t.Fatalf("SSDP discovery calls = %d, want 1", calls)
	}

	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 || hosts[0].Name != "Living Room TV" {
		t.Fatalf("manual inventory name was changed by SSDP discovery: %+v", hosts)
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	active := make(map[string]bool)
	for _, row := range rows {
		if row.Active {
			active[row.Source+"|"+row.Kind+"|"+row.Value] = true
		}
	}
	for _, key := range []string{
		models.DiscoverySourceSSDP + "|" + models.DiscoveryKindFriendlyName + "|Sony BRAVIA XR",
		models.DiscoverySourceSSDP + "|" + models.DiscoveryKindManufacturer + "|Sony",
		models.DiscoverySourceSSDP + "|" + models.DiscoveryKindModel + "|BRAVIA XR",
		models.DiscoverySourceSSDP + "|" + models.DiscoveryKindModelNumber + "|XR-55X90L",
	} {
		if !active[key] {
			t.Fatalf("missing active SSDP evidence %q in %+v", key, rows)
		}
	}
}

func TestSSDPDiscoveryRunsOncePerSuccessfulScan(t *testing.T) {
	setupScanRoutineTest(t)
	oldDiscovery := ssdpDiscovery
	calls := 0
	ssdpDiscovery = func(_ context.Context, addresses []string) []discovery.SSDPObservation {
		calls++
		seen := make(map[string]bool, len(addresses))
		for _, address := range addresses {
			seen[address] = true
		}
		if len(seen) != 2 || !seen["192.168.1.81"] || !seen["192.168.1.82"] {
			t.Fatalf("SSDP addresses = %v, want both scanner-observed addresses", addresses)
		}
		return nil
	}
	t.Cleanup(func() { ssdpDiscovery = oldDiscovery })

	if !processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.81", Mac: "AA:BB:CC:DD:EE:81", Hw: "Vendor A", Date: "2026-09-17 15:31:00", Now: 1},
		{Iface: "eth0", IP: "192.168.1.82", Mac: "AA:BB:CC:DD:EE:82", Hw: "Vendor B", Date: "2026-09-17 15:31:00", Now: 1},
	}, true) {
		t.Fatal("successful scan was not applied")
	}
	if calls != 1 {
		t.Fatalf("SSDP discovery calls = %d, want exactly one per successful scan", calls)
	}
}

func TestFailedScanDoesNotRunSSDPDiscovery(t *testing.T) {
	setupScanRoutineTest(t)
	oldDiscovery := ssdpDiscovery
	calls := 0
	ssdpDiscovery = func(context.Context, []string) []discovery.SSDPObservation {
		calls++
		return nil
	}
	t.Cleanup(func() { ssdpDiscovery = oldDiscovery })

	if processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.83", Mac: "AA:BB:CC:DD:EE:83", Hw: "Vendor", Date: "2026-09-17 15:32:00", Now: 1},
	}, false) {
		t.Fatal("failed scan was reported as applied")
	}
	if calls != 0 {
		t.Fatalf("SSDP discovery calls after failed scan = %d, want 0", calls)
	}
}

func TestSSDPEvidenceSkipsAmbiguousAddressOwnership(t *testing.T) {
	setupScanRoutineTest(t)
	oldDiscovery := ssdpDiscovery
	ssdpDiscovery = func(context.Context, []string) []discovery.SSDPObservation {
		return []discovery.SSDPObservation{
			{Address: "192.168.1.84", Kind: models.DiscoveryKindFriendlyName, Values: []string{"Ambiguous Device"}},
		}
	}
	t.Cleanup(func() { ssdpDiscovery = oldDiscovery })

	macA := "AA:BB:CC:DD:EE:84"
	macB := "AA:BB:CC:DD:EE:85"
	if !processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.84", Mac: macA, Hw: "Vendor A", Date: "2026-09-17 15:33:00", Now: 1},
		{Iface: "eth0", IP: "192.168.1.84", Mac: macB, Hw: "Vendor B", Date: "2026-09-17 15:33:00", Now: 1},
	}, true) {
		t.Fatal("successful scan was not applied")
	}

	for _, mac := range []string{macA, macB} {
		rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
		if err != nil {
			t.Fatalf("SelectHostDiscoveryEvidenceByMAC(%s): %v", mac, err)
		}
		for _, row := range rows {
			if row.Source == models.DiscoverySourceSSDP {
				t.Fatalf("ambiguous SSDP evidence was attributed to %s: %+v", mac, rows)
			}
		}
	}
}
