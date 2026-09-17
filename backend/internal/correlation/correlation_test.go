package correlation

import "testing"

func TestCandidatesRejectSharedVendorOrAddressAlone(t *testing.T) {
	target := IdentityObservation{
		Mac: "00:11:22:33:44:55",
		Addresses: []AddressObservation{{Address: "10.4.1.20"}},
		Evidence: []EvidenceObservation{{Kind: "vendor", Value: "Acme"}},
	}
	others := []IdentityObservation{{
		Mac: "00:11:22:33:44:66",
		Addresses: []AddressObservation{{Address: "10.4.1.20"}},
		Evidence: []EvidenceObservation{{Kind: "vendor", Value: "Acme"}},
	}}

	if got := Candidates(target, others); len(got) != 0 {
		t.Fatalf("expected no candidate for shared vendor/IP alone, got %#v", got)
	}
}

func TestCandidatesUseSharedCurrentHostname(t *testing.T) {
	target := IdentityObservation{
		Mac: "02:11:22:33:44:55",
		Evidence: []EvidenceObservation{{Kind: "hostname", Value: "Phone.local.", Active: true}},
	}
	others := []IdentityObservation{{
		Mac: "06:11:22:33:44:66",
		Evidence: []EvidenceObservation{{Kind: "hostname", Value: "phone.local", Active: true}},
	}}

	got := Candidates(target, others)
	if len(got) != 1 {
		t.Fatalf("expected one candidate, got %#v", got)
	}
	if got[0].Confidence != ConfidenceMedium {
		t.Fatalf("expected medium confidence, got %q score=%d", got[0].Confidence, got[0].Score)
	}
	if got[0].Score != 55 {
		t.Fatalf("expected score 55 (hostname + local support), got %d", got[0].Score)
	}
}

func TestCandidatesCanReachHighConfidenceWithIndependentSignals(t *testing.T) {
	target := IdentityObservation{
		Mac: "02:11:22:33:44:55",
		Evidence: []EvidenceObservation{
			{Kind: "hostname", Value: "Living-Room.local", Active: true},
			{Kind: "friendly-name", Value: "Living Room TV", Active: true},
		},
	}
	others := []IdentityObservation{{
		Mac: "06:11:22:33:44:66",
		Evidence: []EvidenceObservation{
			{Kind: "hostname", Value: "living-room.local", Active: true},
			{Kind: "friendly-name", Value: "living room tv", Active: true},
		},
	}}

	got := Candidates(target, others)
	if len(got) != 1 || got[0].Confidence != ConfidenceHigh {
		t.Fatalf("expected one high-confidence candidate, got %#v", got)
	}
}

func TestCandidatesAllowWeakAddressHandoffOnlyWithLocalMAC(t *testing.T) {
	target := IdentityObservation{
		Mac:       "02:11:22:33:44:55",
		Addresses: []AddressObservation{{Address: "10.4.1.50", Active: false}},
	}
	others := []IdentityObservation{{
		Mac:       "00:11:22:33:44:66",
		Addresses: []AddressObservation{{Address: "10.4.1.50", Active: true}},
	}}

	got := Candidates(target, others)
	if len(got) != 1 {
		t.Fatalf("expected weak candidate, got %#v", got)
	}
	if got[0].Confidence != ConfidenceLow || got[0].Score != 15 {
		t.Fatalf("expected low confidence score 15, got %q/%d", got[0].Confidence, got[0].Score)
	}
}

func TestCandidatesPenalizeConcurrentActivity(t *testing.T) {
	target := IdentityObservation{
		Mac:    "02:11:22:33:44:55",
		Active: true,
		Evidence: []EvidenceObservation{{Kind: "hostname", Value: "device.local", Active: true}},
	}
	others := []IdentityObservation{{
		Mac:    "06:11:22:33:44:66",
		Active: true,
		Evidence: []EvidenceObservation{{Kind: "hostname", Value: "device.local", Active: true}},
	}}

	got := Candidates(target, others)
	if len(got) != 1 {
		t.Fatalf("expected candidate retained with negative evidence, got %#v", got)
	}
	if got[0].Confidence != ConfidenceLow || got[0].Score != 20 {
		t.Fatalf("expected low confidence score 20 after concurrent penalty, got %q/%d", got[0].Confidence, got[0].Score)
	}
	foundNegative := false
	for _, reason := range got[0].Reasons {
		if reason.Code == "concurrent-active" && reason.Weight < 0 {
			foundNegative = true
		}
	}
	if !foundNegative {
		t.Fatalf("expected concurrent-active negative reason, got %#v", got[0].Reasons)
	}
}

func TestCandidatesDeterministicOrder(t *testing.T) {
	target := IdentityObservation{
		Mac: "02:11:22:33:44:55",
		Evidence: []EvidenceObservation{{Kind: "hostname", Value: "same.local", Active: true}},
	}
	others := []IdentityObservation{
		{Mac: "06:11:22:33:44:77", Evidence: []EvidenceObservation{{Kind: "hostname", Value: "same.local", Active: true}}},
		{Mac: "06:11:22:33:44:66", Evidence: []EvidenceObservation{{Kind: "hostname", Value: "same.local", Active: true}}},
	}

	got := Candidates(target, others)
	if len(got) != 2 {
		t.Fatalf("expected two candidates, got %#v", got)
	}
	if got[0].Mac != "06:11:22:33:44:66" || got[1].Mac != "06:11:22:33:44:77" {
		t.Fatalf("expected deterministic MAC ordering for equal scores, got %#v", got)
	}
}

func TestCandidatesUseSharedManufacturerAndModelNumberDescriptor(t *testing.T) {
	target := IdentityObservation{
		Mac: "00:11:22:33:44:55",
		Evidence: []EvidenceObservation{
			{Kind: "manufacturer", Value: "Sony", Active: true},
			{Kind: "model-number", Value: "XR-55A95L", Active: true},
		},
	}
	others := []IdentityObservation{{
		Mac: "00:11:22:33:44:66",
		Evidence: []EvidenceObservation{
			{Kind: "manufacturer", Value: "sony", Active: true},
			{Kind: "model-number", Value: "XR-55A95L", Active: true},
		},
	}}

	got := Candidates(target, others)
	if len(got) != 1 {
		t.Fatalf("expected descriptor candidate, got %#v", got)
	}
	if got[0].Score != 25 || got[0].Confidence != ConfidenceLow {
		t.Fatalf("expected conservative low-confidence descriptor score 25, got %q/%d", got[0].Confidence, got[0].Score)
	}
}
