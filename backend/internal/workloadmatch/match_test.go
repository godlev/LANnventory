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
