package workloadmatch

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestMatchDeterministicExactMACWinsAndExcludesHypervisor(t *testing.T) {
	record := models.InfrastructureWorkloadRecord{
		Workload: models.InfrastructureWorkload{ID: 10, NativeID: "119", WorkloadType: "vm", Name: "media"},
		Interfaces: []models.InfrastructureWorkloadInterface{
			{Name: "net0", Mac: "aa:bb:cc:dd:ee:19", ConfiguredAddress: "10.4.1.19/24"},
			{Name: "net1", Mac: "aa:bb:cc:dd:ee:80", ConfiguredAddress: "10.4.2.19/24"},
		},
	}
	hosts := []models.Host{
		{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:80", IP: "10.4.1.6", DeviceType: "server", Now: 1},
		{ID: 2, Name: "media", Mac: "AA:BB:CC:DD:EE:19", IP: "10.4.1.19", DeviceType: "server", Now: 1},
	}

	result := Match(record, "AA:BB:CC:DD:EE:80", hosts, nil, nil)
	if result.DeterministicExactHostID != 2 || result.ExactAmbiguous {
		t.Fatalf("exact result = %+v", result)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].HostID != 2 || result.Candidates[0].Strength != StrengthExactMAC {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	for _, candidate := range result.Candidates {
		if candidate.HostID == 1 {
			t.Fatalf("source hypervisor appeared as candidate: %+v", candidate)
		}
	}
}

func TestMatchMultipleExactMACHostsIsAmbiguous(t *testing.T) {
	record := models.InfrastructureWorkloadRecord{
		Workload: models.InfrastructureWorkload{Name: "multi-nic"},
		Interfaces: []models.InfrastructureWorkloadInterface{
			{Name: "net0", Mac: "AA:BB:CC:DD:EE:10"},
			{Name: "net1", Mac: "AA:BB:CC:DD:EE:11"},
		},
	}
	hosts := []models.Host{
		{ID: 10, Name: "host-a", Mac: "AA:BB:CC:DD:EE:10"},
		{ID: 11, Name: "host-b", Mac: "AA:BB:CC:DD:EE:11"},
	}

	result := Match(record, "AA:BB:CC:DD:EE:99", hosts, nil, nil)
	if result.DeterministicExactHostID != 0 || !result.ExactAmbiguous {
		t.Fatalf("ambiguous exact result = %+v", result)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2: %+v", len(result.Candidates), result.Candidates)
	}
}

func TestMatchAddressAndNameAreSuggestionOnly(t *testing.T) {
	record := models.InfrastructureWorkloadRecord{
		Workload: models.InfrastructureWorkload{Name: "guest.local"},
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			ConfiguredAddress: "10.4.1.44/24",
		}},
	}
	hosts := []models.Host{
		{ID: 20, Name: "other", DNS: "guest.local.", Mac: "AA:BB:CC:DD:EE:20", IP: "10.4.1.200", Now: 1},
		{ID: 21, Name: "guest", Mac: "AA:BB:CC:DD:EE:21", IP: "10.4.1.21", Now: 0},
	}
	addresses := []models.HostAddress{
		{Mac: "AA:BB:CC:DD:EE:21", Address: "10.4.1.44", Active: false},
	}
	discovery := []models.HostDiscoveryEvidence{
		{Mac: "AA:BB:CC:DD:EE:21", Kind: models.DiscoveryKindHostname, Value: "guest.local", Active: true},
	}

	result := Match(record, "AA:BB:CC:DD:EE:99", hosts, addresses, discovery)
	if result.DeterministicExactHostID != 0 || result.ExactAmbiguous {
		t.Fatalf("weak evidence became exact: %+v", result)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	if result.Candidates[0].HostID != 21 || result.Candidates[0].Strength != StrengthAddress {
		t.Fatalf("address candidate ordering = %+v", result.Candidates)
	}
	if result.Candidates[1].HostID != 20 || result.Candidates[1].Strength != StrengthName {
		t.Fatalf("name candidate ordering = %+v", result.Candidates)
	}
}

func TestMatchCurrentAddressIsAddressEvidenceNotExact(t *testing.T) {
	record := models.InfrastructureWorkloadRecord{
		Workload: models.InfrastructureWorkload{Name: "guest"},
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			ConfiguredAddress: "2001:db8::44/64",
		}},
	}
	hosts := []models.Host{{
		ID: 30, Name: "different", Mac: "AA:BB:CC:DD:EE:30", IP: "2001:db8::44", Now: 1,
	}}

	result := Match(record, "", hosts, nil, nil)
	if result.DeterministicExactHostID != 0 || len(result.Candidates) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.Candidates[0].Strength != StrengthAddress {
		t.Fatalf("strength = %q, want address", result.Candidates[0].Strength)
	}
}

func TestMatchDeduplicatesEvidencePerHost(t *testing.T) {
	record := models.InfrastructureWorkloadRecord{
		Workload: models.InfrastructureWorkload{Name: "guest"},
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			Mac:               "AA:BB:CC:DD:EE:40",
			ConfiguredAddress: "10.4.1.40/24",
		}},
	}
	hosts := []models.Host{{ID: 40, Name: "guest", DNS: "guest", Mac: "AA:BB:CC:DD:EE:40", IP: "10.4.1.40", Now: 1}}
	addresses := []models.HostAddress{{Mac: "AA:BB:CC:DD:EE:40", Address: "10.4.1.40", Active: true}}
	discovery := []models.HostDiscoveryEvidence{{Mac: "AA:BB:CC:DD:EE:40", Kind: models.DiscoveryKindFriendlyName, Value: "guest", Active: true}}

	result := Match(record, "", hosts, addresses, discovery)
	if len(result.Candidates) != 1 || result.Candidates[0].Strength != StrengthExactMAC {
		t.Fatalf("candidate = %+v", result.Candidates)
	}
	if len(result.Candidates[0].Evidence) < 4 {
		t.Fatalf("expected explainable multi-source evidence, got %+v", result.Candidates[0].Evidence)
	}
}


func TestMatchClassifiesSameAddressDifferentMACAsPossibleIPConflict(t *testing.T) {
	record := models.InfrastructureWorkloadRecord{
		Workload: models.InfrastructureWorkload{ID: 104, NativeID: "104", WorkloadType: "container", Name: "actualbudget"},
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			Mac:               "BC:24:11:14:02:A5",
			ConfiguredAddress: "10.4.1.67/24",
		}},
	}
	hosts := []models.Host{{
		ID: 67, Name: "IR + RF - Tuya", Mac: "FC:67:1F:26:20:A5", IP: "10.4.1.67", Now: 1,
	}}

	result := Match(record, "AA:BB:CC:DD:EE:99", hosts, nil, nil)
	if result.DeterministicExactHostID != 0 || len(result.Candidates) != 1 {
		t.Fatalf("result = %+v", result)
	}
	candidate := result.Candidates[0]
	if candidate.Strength != StrengthAddress || candidate.Assessment != "possible-ip-conflict" || !candidate.PossibleIPConflict {
		t.Fatalf("candidate classification = %+v", candidate)
	}
	if len(candidate.MatchedAddresses) != 1 || candidate.MatchedAddresses[0] != "10.4.1.67" {
		t.Fatalf("matched addresses = %+v", candidate.MatchedAddresses)
	}
	if len(candidate.WorkloadMACs) != 1 || candidate.WorkloadMACs[0] != "BC:24:11:14:02:A5" {
		t.Fatalf("workload MACs = %+v", candidate.WorkloadMACs)
	}
	if candidate.EvidenceFingerprint == "" {
		t.Fatal("evidence fingerprint is empty")
	}
}

func TestMatchEvidenceFingerprintIgnoresObservationTimestampsButChangesWithMaterialEvidence(t *testing.T) {
	record := models.InfrastructureWorkloadRecord{
		Workload: models.InfrastructureWorkload{ID: 122, NativeID: "122", WorkloadType: "container", Name: "frigate"},
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			Mac:               "BC:24:11:F8:D8:48",
			ConfiguredAddress: "10.4.1.17/24",
		}},
	}
	host := models.Host{ID: 17, Name: "KIVI Kids TV", Mac: "54:67:E6:E7:6F:CB", IP: "10.4.1.200"}
	addresses := []models.HostAddress{{
		Mac: host.Mac, Address: "10.4.1.17", Active: true,
		FirstSeen: "2026-09-19T10:00:00Z", LastSeen: "2026-09-19T11:00:00Z",
	}}

	first := Match(record, "", []models.Host{host}, addresses, nil)
	addresses[0].LastSeen = "2026-09-20T22:00:00Z"
	second := Match(record, "", []models.Host{host}, addresses, nil)
	if len(first.Candidates) != 1 || len(second.Candidates) != 1 {
		t.Fatalf("candidates first=%+v second=%+v", first.Candidates, second.Candidates)
	}
	if first.Candidates[0].EvidenceFingerprint != second.Candidates[0].EvidenceFingerprint {
		t.Fatalf("timestamp-only refresh changed fingerprint: %q != %q", first.Candidates[0].EvidenceFingerprint, second.Candidates[0].EvidenceFingerprint)
	}

	host.Now = 1
	addresses[0].Active = false
	thirdLiveness := Match(record, "", []models.Host{host}, addresses, nil)
	if len(thirdLiveness.Candidates) != 1 || thirdLiveness.Candidates[0].EvidenceFingerprint != first.Candidates[0].EvidenceFingerprint {
		t.Fatalf("liveness-only change altered identity fingerprint: first=%+v liveness=%+v", first.Candidates, thirdLiveness.Candidates)
	}

	host.Mac = "54:67:E6:E7:6F:CC"
	addresses[0].Mac = host.Mac
	third := Match(record, "", []models.Host{host}, addresses, nil)
	if len(third.Candidates) != 1 || third.Candidates[0].EvidenceFingerprint == first.Candidates[0].EvidenceFingerprint {
		t.Fatalf("material MAC change did not change fingerprint: first=%+v third=%+v", first.Candidates, third.Candidates)
	}

	host.Mac = "BC:24:11:F8:D8:48"
	fourth := Match(record, "", []models.Host{host}, nil, nil)
	if fourth.DeterministicExactHostID != host.ID || len(fourth.Candidates) != 1 || fourth.Candidates[0].Assessment != "exact-mac" {
		t.Fatalf("stronger exact-MAC evidence did not supersede weak evidence: %+v", fourth)
	}
}
