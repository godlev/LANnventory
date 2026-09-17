package routines

import (
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestProcessScanResultRecordsScannerEvidenceForAllMACAddresses(t *testing.T) {
	setupScanRoutineTest(t)
	mac := "AA:BB:CC:DD:EE:41"

	processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.41", Mac: mac, Hw: "Sony", Date: "2026-09-17 14:00:00", Now: 1},
		{Iface: "wifi0", IP: "192.168.1.42", Mac: "aa-bb-cc-dd-ee-41", Hw: "Sony", Date: "2026-09-17 14:00:00", Now: 1},
	}, true)

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}

	vendorByAddress := map[string]bool{}
	for _, row := range rows {
		if row.Source == models.DiscoverySourceScanner && row.Kind == models.DiscoveryKindVendor && row.Value == "Sony" {
			vendorByAddress[row.Address] = row.Active
		}
	}
	if !vendorByAddress["192.168.1.41"] || !vendorByAddress["192.168.1.42"] {
		t.Fatalf("scanner evidence did not preserve both MAC/IP observations: %+v", rows)
	}
}

func TestNewHostPersistsReverseDNSEvidenceWithoutMixingInventoryProvenance(t *testing.T) {
	setupScanRoutineTest(t)
	oldLookupDNS := lookupDNS
	lookupDNS = func(models.Host) (string, string) {
		return "BRAVIA-4K", "bravia.home Sony-TV.local"
	}
	t.Cleanup(func() { lookupDNS = oldLookupDNS })

	mac := "AA:BB:CC:DD:EE:42"
	processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.43", Mac: mac, Hw: "Sony", Date: "2026-09-17 14:05:00", Now: 1},
	}, true)

	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 || hosts[0].Name != "BRAVIA-4K" || hosts[0].DNS != "bravia.home Sony-TV.local" {
		t.Fatalf("new-host compatibility fields = %+v", hosts)
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
		models.DiscoverySourceScanner + "|" + models.DiscoveryKindVendor + "|Sony",
		models.DiscoverySourceReverseDNS + "|" + models.DiscoveryKindHostname + "|bravia.home",
		models.DiscoverySourceReverseDNS + "|" + models.DiscoveryKindHostname + "|Sony-TV.local",
	} {
		if !active[key] {
			t.Fatalf("missing active discovery evidence %q in %+v", key, rows)
		}
	}
}

func TestExistingManualNameIsNotOverwrittenByDiscovery(t *testing.T) {
	setupScanRoutineTest(t)
	lookupCalls := 0
	oldLookupDNS := lookupDNS
	lookupDNS = func(models.Host) (string, string) {
		lookupCalls++
		return "BRAVIA-4K", "bravia.home"
	}
	t.Cleanup(func() { lookupDNS = oldLookupDNS })

	mac := "AA:BB:CC:DD:EE:43"
	gdb.Update("now", models.Host{
		Name: "Living Room TV", Iface: "eth0", IP: "192.168.1.44", Mac: mac,
		Hw: "Sony", Date: "2026-09-17 13:00:00", Known: 1, Now: 1,
	})

	processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.44", Mac: mac, Hw: "Sony Corporation", Date: "2026-09-17 14:10:00", Now: 1},
	}, true)

	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 || hosts[0].Name != "Living Room TV" {
		t.Fatalf("manual name was overwritten: %+v", hosts)
	}
	if lookupCalls != 0 {
		t.Fatalf("existing host triggered %d reverse-DNS compatibility lookups, want 0", lookupCalls)
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	if len(rows) != 1 || rows[0].Source != models.DiscoverySourceScanner || rows[0].Value != "Sony Corporation" || !rows[0].Active {
		t.Fatalf("scanner evidence for existing host = %+v", rows)
	}
}

func TestFailedScanDoesNotMutateDiscoveryEvidence(t *testing.T) {
	setupScanRoutineTest(t)
	mac := "AA:BB:CC:DD:EE:44"

	if err := gdb.RecordHostDiscoveryEvidence(mac, "192.168.1.45", models.DiscoverySourceScanner, models.DiscoveryKindVendor, []string{"Vendor A"}, "2026-09-17 14:00:00"); err != nil {
		t.Fatalf("seed discovery evidence: %v", err)
	}
	if processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.45", Mac: mac, Hw: "Vendor B", Date: "2026-09-17 14:15:00", Now: 1},
	}, false) {
		t.Fatal("failed scan was reported as applied")
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	if len(rows) != 1 || rows[0].Value != "Vendor A" || !rows[0].Active || rows[0].LastSeen != "2026-09-17 14:00:00" {
		t.Fatalf("failed scan mutated discovery evidence: %+v", rows)
	}
}
