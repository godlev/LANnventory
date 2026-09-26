package identification

import (
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC)

func TestAssessExactCurrentWorkloadIsHighConfidenceInfrastructure(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.27",
		Workloads: []WorkloadEvidence{{
			Name:         "ubuntu-plex-immich",
			WorkloadType: "vm",
			Source:       "proxmox-api",
			ExactMAC:     true,
			Current:      true,
		}},
		Services: []ServiceEvidence{{
			Address: "10.4.1.27", Protocol: "tcp", Port: 554, State: "open", Hint: "RTSP", LastChecked: testNow.Add(-time.Hour),
		}},
	})

	if got.SuggestedName == nil || got.SuggestedName.Value != "ubuntu-plex-immich" || got.SuggestedName.Confidence != ConfidenceHigh {
		t.Fatalf("name suggestion = %+v", got.SuggestedName)
	}
	if got.SuggestedDeviceType == nil || got.SuggestedDeviceType.Value != "virtual-machine" || got.SuggestedDeviceType.Confidence != ConfidenceHigh {
		t.Fatalf("type suggestion = %+v", got.SuggestedDeviceType)
	}
	if len(got.Cautions) == 0 || !strings.Contains(got.Cautions[0], "application/service context") {
		t.Fatalf("camera-like service should remain context, cautions=%+v", got.Cautions)
	}
}

func TestAssessMembershipWithoutExactMACCannotCreateWorkloadSuggestion(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.27",
		Workloads: []WorkloadEvidence{{
			Name: "media", WorkloadType: "vm", Source: "proxmox-api", Current: true, ExactMAC: false,
		}},
	})
	if got.SuggestedName != nil || got.SuggestedDeviceType != nil {
		t.Fatalf("non-exact membership created suggestion: %+v", got)
	}
}

func TestAssessOneSpecificOpenPortIsOnlyLowConfidence(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.50",
		Services: []ServiceEvidence{{
			Address: "10.4.1.50", Protocol: "tcp", Port: 554, State: "open", Hint: "RTSP", LastChecked: testNow.Add(-30 * time.Minute),
		}},
	})
	if got.SuggestedDeviceType == nil || got.SuggestedDeviceType.Value != "camera" || got.SuggestedDeviceType.Confidence != ConfidenceLow {
		t.Fatalf("single service suggestion = %+v", got.SuggestedDeviceType)
	}
}

func TestAssessServiceAndDescriptorAgreementRaisesOnlyToMedium(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.51",
		Discovery: []DiscoveryEvidence{{
			Source: "ssdp", Kind: "friendly-name", Value: "Garage Camera", Address: "10.4.1.51", Active: true, LastSeen: testNow.Add(-time.Hour),
		}},
		Services: []ServiceEvidence{{
			Address: "10.4.1.51", Protocol: "tcp", Port: 554, State: "open", Hint: "RTSP", LastChecked: testNow.Add(-time.Hour),
		}},
	})
	if got.SuggestedDeviceType == nil || got.SuggestedDeviceType.Value != "camera" || got.SuggestedDeviceType.Confidence != ConfidenceMedium {
		t.Fatalf("combined clue suggestion = %+v", got.SuggestedDeviceType)
	}
	if got.SuggestedDeviceType.Confidence == ConfidenceHigh {
		t.Fatal("service/descriptor clues must never become High")
	}
}

func TestAssessHistoricalAndStaleEvidenceDoNotCreateCurrentSuggestions(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.60",
		FreshWithin:    6 * time.Hour,
		Discovery: []DiscoveryEvidence{
			{Source: "mdns", Kind: "hostname", Value: "old-cam.local", Address: "10.4.1.60", Active: false, LastSeen: testNow.Add(-time.Hour)},
			{Source: "ssdp", Kind: "friendly-name", Value: "Camera", Address: "10.4.1.60", Active: true, LastSeen: testNow.Add(-12 * time.Hour)},
		},
		Services: []ServiceEvidence{
			{Address: "10.4.1.60", Protocol: "tcp", Port: 554, State: "open", LastChecked: testNow.Add(-12 * time.Hour)},
			{Address: "10.4.1.99", Protocol: "tcp", Port: 631, State: "open", LastChecked: testNow.Add(-time.Hour)},
		},
	})
	if got.SuggestedName != nil || got.SuggestedDeviceType != nil || got.ClueCount != 0 {
		t.Fatalf("stale/historical evidence created current assessment: %+v", got)
	}

	counts := map[Freshness]int{}
	for _, evidence := range got.Evidence {
		counts[evidence.Freshness]++
	}
	if counts[FreshnessStale] != 2 || counts[FreshnessHistorical] != 2 {
		t.Fatalf("freshness counts = %+v, evidence=%+v", counts, got.Evidence)
	}
}

func TestAssessEqualPriorityNameConflictProducesNoSuggestion(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.70",
		Discovery: []DiscoveryEvidence{
			{Source: "mdns", Kind: "hostname", Value: "alpha.local", Address: "10.4.1.70", Active: true, LastSeen: testNow.Add(-time.Hour)},
			{Source: "mdns", Kind: "hostname", Value: "beta.local", Address: "10.4.1.70", Active: true, LastSeen: testNow.Add(-time.Hour)},
		},
	})
	if got.SuggestedName != nil || len(got.Conflicts) == 0 {
		t.Fatalf("conflicting mDNS names = %+v", got)
	}
}

func TestAssessNamePriorityAndGenericFiltering(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.80",
		Discovery: []DiscoveryEvidence{
			{Source: "system-resolver", Kind: "hostname", Value: "resolver-name", Address: "10.4.1.80", Active: true, LastSeen: testNow.Add(-time.Hour)},
			{Source: "reverse-dns", Kind: "hostname", Value: "10.4.1.80", Address: "10.4.1.80", Active: true, LastSeen: testNow.Add(-time.Hour)},
			{Source: "ssdp", Kind: "friendly-name", Value: "device", Address: "10.4.1.80", Active: true, LastSeen: testNow.Add(-time.Hour)},
			{Source: "mdns", Kind: "hostname", Value: "living-room.local.", Address: "10.4.1.80", Active: true, LastSeen: testNow.Add(-time.Hour)},
		},
	})
	if got.SuggestedName == nil || got.SuggestedName.Value != "living-room.local" || got.SuggestedName.Source != "mdns" {
		t.Fatalf("name suggestion = %+v", got.SuggestedName)
	}
}

func TestAssessResolverDuplicateCountsAsOneIndependentClue(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.90",
		Discovery: []DiscoveryEvidence{
			{Source: "reverse-dns", Kind: "hostname", Value: "nas.home", Address: "10.4.1.90", Active: true, LastSeen: testNow.Add(-time.Hour)},
			{Source: "system-resolver", Kind: "hostname", Value: "nas.home.", Address: "10.4.1.90", Active: true, LastSeen: testNow.Add(-time.Hour)},
		},
	})
	if got.ClueCount != 1 {
		t.Fatalf("resolver duplicates counted independently: clueCount=%d", got.ClueCount)
	}
	if got.SuggestedName == nil || got.SuggestedName.Confidence != ConfidenceLow || got.SuggestedName.Source != "reverse-dns" {
		t.Fatalf("resolver suggestion = %+v", got.SuggestedName)
	}
}

func TestAssessGenericWebAndSSHDoNotInventDeviceType(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.100",
		Services: []ServiceEvidence{
			{Address: "10.4.1.100", Protocol: "tcp", Port: 22, State: "open", Hint: "SSH", LastChecked: testNow.Add(-time.Hour)},
			{Address: "10.4.1.100", Protocol: "tcp", Port: 443, State: "open", Hint: "HTTPS", LastChecked: testNow.Add(-time.Hour)},
		},
	})
	if got.SuggestedDeviceType != nil {
		t.Fatalf("generic services invented type: %+v", got.SuggestedDeviceType)
	}
}

func TestAssessVendorOnlyDoesNotInventType(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.110",
		Discovery: []DiscoveryEvidence{{
			Source: "scanner", Kind: "vendor", Value: "Example Networks", Address: "10.4.1.110", Active: true, LastSeen: testNow.Add(-time.Hour),
		}},
	})
	if got.SuggestedDeviceType != nil {
		t.Fatalf("vendor-only evidence invented type: %+v", got.SuggestedDeviceType)
	}
}

func TestAssessConflictingCurrentTypeCluesProduceNoSuggestion(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.120",
		Discovery: []DiscoveryEvidence{{
			Source: "ssdp", Kind: "friendly-name", Value: "Office Printer", Address: "10.4.1.120", Active: true, LastSeen: testNow.Add(-time.Hour),
		}},
		Services: []ServiceEvidence{{
			Address: "10.4.1.120", Protocol: "tcp", Port: 554, State: "open", Hint: "RTSP", LastChecked: testNow.Add(-time.Hour),
		}},
	})
	if got.SuggestedDeviceType != nil || len(got.Conflicts) == 0 {
		t.Fatalf("conflicting type clues = %+v", got)
	}
}

func TestAssessClosedServiceIsNotAClue(t *testing.T) {
	got := Assess(Input{
		Now:            testNow,
		CurrentAddress: "10.4.1.130",
		Services: []ServiceEvidence{{
			Address: "10.4.1.130", Protocol: "tcp", Port: 631, State: "closed", Hint: "IPP", LastChecked: testNow.Add(-time.Hour),
		}},
	})
	if got.SuggestedDeviceType != nil || got.ClueCount != 0 {
		t.Fatalf("closed service became a clue: %+v", got)
	}
}
