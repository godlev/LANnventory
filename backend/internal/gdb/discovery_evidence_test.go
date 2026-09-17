package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestRecordHostDiscoveryEvidenceRetainsSupersededValues(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:31"

	if err := RecordHostDiscoveryEvidence(
		"aa-bb-cc-dd-ee-31",
		"192.168.1.31",
		models.DiscoverySourceReverseDNS,
		models.DiscoveryKindHostname,
		[]string{" BRAVIA-4K ", "Sony-TV.local", "Sony-TV.local"},
		"2026-09-17 10:00:00",
	); err != nil {
		t.Fatalf("RecordHostDiscoveryEvidence first: %v", err)
	}

	if err := RecordHostDiscoveryEvidence(
		mac,
		"192.168.1.31",
		models.DiscoverySourceReverseDNS,
		models.DiscoveryKindHostname,
		[]string{"Sony-TV.local", "Sony-BRAVIA.local"},
		"2026-09-17 10:05:00",
	); err != nil {
		t.Fatalf("RecordHostDiscoveryEvidence second: %v", err)
	}

	rows, err := SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows len = %d, want 3: %+v", len(rows), rows)
	}

	byValue := make(map[string]models.HostDiscoveryEvidence, len(rows))
	for _, row := range rows {
		byValue[row.Value] = row
		if row.Mac != mac || row.Address != "192.168.1.31" || row.Source != models.DiscoverySourceReverseDNS || row.Kind != models.DiscoveryKindHostname {
			t.Fatalf("normalized evidence row = %+v", row)
		}
	}

	superseded := byValue["BRAVIA-4K"]
	if superseded.Active || superseded.FirstSeen != "2026-09-17 10:00:00" || superseded.LastSeen != "2026-09-17 10:00:00" {
		t.Fatalf("superseded evidence = %+v", superseded)
	}
	retained := byValue["Sony-TV.local"]
	if !retained.Active || retained.FirstSeen != "2026-09-17 10:00:00" || retained.LastSeen != "2026-09-17 10:05:00" {
		t.Fatalf("retained evidence = %+v", retained)
	}
	added := byValue["Sony-BRAVIA.local"]
	if !added.Active || added.FirstSeen != "2026-09-17 10:05:00" || added.LastSeen != "2026-09-17 10:05:00" {
		t.Fatalf("new evidence = %+v", added)
	}
}

func TestRecordHostDiscoveryEvidenceScopesSourcesIndependently(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:32"

	if err := RecordHostDiscoveryEvidence(mac, "10.4.1.32", models.DiscoverySourceScanner, models.DiscoveryKindVendor, []string{"Sony"}, "2026-09-17 11:00:00"); err != nil {
		t.Fatalf("record scanner vendor: %v", err)
	}
	if err := RecordHostDiscoveryEvidence(mac, "10.4.1.32", models.DiscoverySourceReverseDNS, models.DiscoveryKindHostname, []string{"tv.home"}, "2026-09-17 11:00:00"); err != nil {
		t.Fatalf("record reverse DNS: %v", err)
	}
	if err := RecordHostDiscoveryEvidence(mac, "10.4.1.32", models.DiscoverySourceScanner, models.DiscoveryKindVendor, []string{"Sony Corporation"}, "2026-09-17 11:05:00"); err != nil {
		t.Fatalf("replace scanner vendor: %v", err)
	}

	rows, err := SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows len = %d, want 3: %+v", len(rows), rows)
	}

	active := make(map[string]bool)
	for _, row := range rows {
		active[row.Source+"|"+row.Value] = row.Active
	}
	if active[models.DiscoverySourceScanner+"|Sony"] {
		t.Fatal("superseded scanner vendor remained active")
	}
	if !active[models.DiscoverySourceScanner+"|Sony Corporation"] {
		t.Fatal("new scanner vendor is not active")
	}
	if !active[models.DiscoverySourceReverseDNS+"|tv.home"] {
		t.Fatal("reverse-DNS evidence was incorrectly deactivated by scanner update")
	}
}

func TestRecordHostDiscoveryEvidenceEmptyValuesDeactivateScope(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:33"

	if err := RecordHostDiscoveryEvidence(mac, "10.4.1.33", models.DiscoverySourceReverseDNS, models.DiscoveryKindHostname, []string{"old.home"}, "2026-09-17 12:00:00"); err != nil {
		t.Fatalf("record evidence: %v", err)
	}
	if err := RecordHostDiscoveryEvidence(mac, "10.4.1.33", models.DiscoverySourceReverseDNS, models.DiscoveryKindHostname, nil, "2026-09-17 12:05:00"); err != nil {
		t.Fatalf("deactivate evidence scope: %v", err)
	}

	rows, err := SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	if len(rows) != 1 || rows[0].Active {
		t.Fatalf("deactivated evidence = %+v, want one retained inactive row", rows)
	}
}

func TestRecordHostDiscoveryEvidenceValidatesIdentityAndScope(t *testing.T) {
	startSelectTestDB(t)

	tests := []struct {
		name      string
		mac       string
		address   string
		source    string
		kind      string
		observed  string
	}{
		{name: "invalid MAC", mac: "not-a-mac", address: "10.4.1.1", source: "scanner", kind: "vendor", observed: "2026-09-17 13:00:00"},
		{name: "invalid address", mac: "AA:BB:CC:DD:EE:34", address: "not-an-ip", source: "scanner", kind: "vendor", observed: "2026-09-17 13:00:00"},
		{name: "missing source", mac: "AA:BB:CC:DD:EE:34", address: "10.4.1.34", source: "", kind: "vendor", observed: "2026-09-17 13:00:00"},
		{name: "missing kind", mac: "AA:BB:CC:DD:EE:34", address: "10.4.1.34", source: "scanner", kind: "", observed: "2026-09-17 13:00:00"},
		{name: "missing timestamp", mac: "AA:BB:CC:DD:EE:34", address: "10.4.1.34", source: "scanner", kind: "vendor", observed: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RecordHostDiscoveryEvidence(tt.mac, tt.address, tt.source, tt.kind, []string{"value"}, tt.observed); err == nil {
				t.Fatal("RecordHostDiscoveryEvidence returned nil error")
			}
		})
	}
}
