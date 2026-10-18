package relationships

import (
	"errors"
	"strings"
	"testing"
)

func endpoint(kind EntityType, id uint64) Endpoint {
	return Endpoint{EntityType: kind, EntityID: id}
}

func currentHosts(endpoints ...Endpoint) EndpointExists {
	set := make(map[Endpoint]bool, len(endpoints))
	for _, e := range endpoints {
		set[e] = true
	}
	return func(e Endpoint) (bool, error) { return set[e], nil }
}

func TestRegistryIsSmallExplicitAndImmutable(t *testing.T) {
	all := Definitions()
	if len(all) != 5 {
		t.Fatalf("registry has %d entries; want five", len(all))
	}
	seen := map[Type]bool{}
	for _, def := range all {
		if seen[def.Type] || def.SourceLabel == "" || def.InverseLabel == "" ||
			len(def.SourceTypes) == 0 || len(def.TargetTypes) == 0 {
			t.Fatalf("invalid registry entry: %+v", def)
		}
		seen[def.Type] = true
		if def.Hierarchical || def.Directionality != Directed {
			t.Fatalf("unexpected hierarchy/symmetry in 38A.1: %+v", def)
		}
	}
	for _, kind := range []Type{TypeHosts, TypeMatchesHost} {
		def, ok := LookupType(kind)
		if !ok || def.ManualAllowed != false {
			t.Fatalf("proxmox-derived relationship may not be generic-manual: %+v", def)
		}
	}
	for _, kind := range []Type{TypeConnectedTo, TypeManages, TypeDependsOn} {
		def, ok := LookupType(kind)
		if !ok || !def.ManualAllowed {
			t.Fatalf("missing manual relationship type: %q", kind)
		}
	}
	all[0].SourceLabel = "CHANGED"
	all[0].SourceTypes[0] = EntityWorkload
	original, _ := LookupType(TypeHosts)
	if original.SourceLabel != "Hosts" || original.SourceTypes[0] != EntityHost {
		t.Fatal("registry is externally mutable")
	}
}

func TestManualRelationshipValidation(t *testing.T) {
	a, b := endpoint(EntityHost, 10), endpoint(EntityHost, 20)
	w := endpoint(EntityWorkload, 119)
	exists := currentHosts(a, b, w)
	tests := []struct {
		name   string
		input  ManualCreateInput
		want   error
	}{
		{"connected", ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: b}, nil},
		{"manages", ManualCreateInput{Type: TypeManages, Source: a, Target: b}, nil},
		{"depends on", ManualCreateInput{Type: TypeDependsOn, Source: a, Target: b}, nil},
		{"self", ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: a}, ErrSelfRelationship},
		{"missing source ID", ManualCreateInput{Type: TypeConnectedTo, Source: endpoint(EntityHost, 0), Target: b}, ErrInvalidEndpoint},
		{"missing target ID", ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: endpoint(EntityHost, 0)}, ErrInvalidEndpoint},
		{"invalid source entity", ManualCreateInput{Type: TypeConnectedTo, Source: endpoint("service", 1), Target: b}, ErrInvalidEntityType},
		{"invalid target entity", ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: endpoint("interface", 1)}, ErrInvalidEntityType},
		{"invalid predicate", ManualCreateInput{Type: "parent_of", Source: a, Target: b}, ErrInvalidType},
		{"invalid endpoint combination", ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: w}, ErrEndpointCombination},
		{"cannot create HOSTS", ManualCreateInput{Type: TypeHosts, Source: a, Target: w}, ErrManualNotAllowed},
		{"cannot create MATCHES", ManualCreateInput{Type: TypeMatchesHost, Source: w, Target: b}, ErrManualNotAllowed},
		{"too long note", ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: b, Note: strings.Repeat("✓", MaxManualNoteRunes+1)}, ErrNoteTooLong},
		{"invalid note control", ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: b, Note: "a\x00b"}, ErrInvalidNote},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertion, err := NewManualAssertion(tc.input, exists)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if tc.want == nil && (assertion.Provenance != ProvenanceManual || assertion.SourceInstance != "" || assertion.SourceDetail != "") {
				t.Fatalf("spoofable/unexpected manual provenance: %+v", assertion)
			}
		})
	}
}

func TestEndpointExistenceIsMandatory(t *testing.T) {
	a, b := endpoint(EntityHost, 4), endpoint(EntityHost, 5)
	input := ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: b}
	if _, err := NewManualAssertion(input, nil); !errors.Is(err, ErrResolverRequired) {
		t.Fatalf("missing resolver error = %v", err)
	}
	if _, err := NewManualAssertion(input, currentHosts(b)); !errors.Is(err, ErrEntityNotFound) {
		t.Fatalf("missing source error = %v", err)
	}
	if _, err := NewManualAssertion(input, currentHosts(a)); !errors.Is(err, ErrEntityNotFound) {
		t.Fatalf("missing target error = %v", err)
	}
	boom := errors.New("database failed")
	if _, err := NewManualAssertion(input, func(Endpoint) (bool, error) { return false, boom }); !errors.Is(err, boom) {
		t.Fatalf("resolver error = %v", err)
	}
}

func TestDirectedIdentityKeyAndInverseLabels(t *testing.T) {
	a, b := endpoint(EntityHost, 1), endpoint(EntityHost, 2)
	exists := currentHosts(a, b)
	input := ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: b, Note: " port 8 "}
	first, err := NewManualAssertion(input, exists)
	if err != nil {
		t.Fatal(err)
	}
	if first.Note != "port 8" {
		t.Fatalf("trimmed note = %q", first.Note)
	}
	duplicate, err := NewManualAssertion(ManualCreateInput{Type: TypeConnectedTo, Source: a, Target: b, Note: "different note"}, exists)
	if err != nil {
		t.Fatal(err)
	}
	forwardKey, err := first.IdentityKey()
	if err != nil {
		t.Fatal(err)
	}
	duplicateKey, err := duplicate.IdentityKey()
	if err != nil || forwardKey != duplicateKey {
		t.Fatalf("identical endpoint pair must have identical key: %q, %q (%v)", forwardKey, duplicateKey, err)
	}
	reverse, err := NewManualAssertion(ManualCreateInput{Type: TypeConnectedTo, Source: b, Target: a}, exists)
	if err != nil {
		t.Fatal(err)
	}
	reverseKey, err := reverse.IdentityKey()
	if err != nil || reverseKey == forwardKey {
		t.Fatalf("directed reversal must differ: forward=%q reverse=%q", forwardKey, reverseKey)
	}
	sourceLabel, err := LabelForEndpoint(first, a)
	if err != nil || sourceLabel != "Connected to" {
		t.Fatalf("source label = %q, %v", sourceLabel, err)
	}
	inverseLabel, err := LabelForEndpoint(first, b)
	if err != nil || inverseLabel != "Connected devices" {
		t.Fatalf("target label = %q, %v", inverseLabel, err)
	}
	if _, err := LabelForEndpoint(first, endpoint(EntityHost, 9)); !errors.Is(err, ErrEndpointNotIncident) {
		t.Fatalf("nonparticipant lookup = %v", err)
	}
}

func TestSymmetricNormalizationHelperFutureProof(t *testing.T) {
	a, b := endpoint(EntityHost, 3), endpoint(EntityHost, 12)
	x1, y1, err := normalizeEndpoints(Symmetric, a, b)
	if err != nil { t.Fatal(err) }
	x2, y2, err := normalizeEndpoints(Symmetric, b, a)
	if err != nil || x1 != x2 || y1 != y2 {
		t.Fatalf("symmetric pair not normalized: %v/%v vs %v/%v", x1, y1, x2, y2)
	}
	x3, y3, err := normalizeEndpoints(Directed, b, a)
	if err != nil || x3 != b || y3 != a {
		t.Fatalf("directed pair reordered: %v %v", x3, y3)
	}
	if _, _, err := normalizeEndpoints("mystery", a, b); err == nil {
		t.Fatal("invalid directionality accepted")
	}
}

func TestNonHierarchicalCyclesAllowed(t *testing.T) {
	a, b, c := endpoint(EntityHost, 1), endpoint(EntityHost, 2), endpoint(EntityHost, 3)
	exists := currentHosts(a, b, c)
	for _, pair := range [][2]Endpoint{{a,b},{b,c},{c,a}} {
		if _, err := NewManualAssertion(ManualCreateInput{Type: TypeDependsOn, Source: pair[0], Target: pair[1]}, exists); err != nil {
			t.Fatalf("nonhierarchical cycle rejected: %v", err)
		}
	}
}

func TestAssertionIdentityRejectsInvalidProvenanceAndReservedWrites(t *testing.T) {
	a, b := endpoint(EntityHost, 1), endpoint(EntityHost, 2)
	manual, err := NewManualAssertion(ManualCreateInput{Type: TypeManages, Source: a, Target: b}, currentHosts(a,b))
	if err != nil { t.Fatal(err) }
	invalid := manual
	invalid.Provenance = "mikrotik"
	if _, err := invalid.IdentityKey(); !errors.Is(err, ErrInvalidProvenance) {
		t.Fatalf("unknown provenance = %v", err)
	}
	reserved := Assertion{Type: TypeMatchesHost, Source: endpoint(EntityWorkload, 12), Target: a, Provenance: ProvenanceManual}
	if _, err := reserved.IdentityKey(); !errors.Is(err, ErrManualNotAllowed) {
		t.Fatalf("reserved manual edge = %v", err)
	}
	projected := reserved
	projected.Provenance = ProvenanceSystem
	if _, err := projected.IdentityKey(); err != nil {
		t.Fatalf("derived link disallowed: %v", err)
	}
}
