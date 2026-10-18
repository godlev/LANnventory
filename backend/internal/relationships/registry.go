package relationships

import "fmt"

// Definition is the single authoritative registry entry for a predicate.
// AutomaticAllowed means future source assertions may be represented; it does
// not enable automatic discovery or grant an ingestion API in Phase 38A.
type Definition struct {
	Type             Type           `json:"type"`
	SourceTypes      []EntityType   `json:"sourceTypes"`
	TargetTypes      []EntityType   `json:"targetTypes"`
	Directionality   Directionality `json:"directionality"`
	SourceLabel      string         `json:"sourceLabel"`
	InverseLabel     string         `json:"inverseLabel"`
	ManualAllowed    bool           `json:"manualAllowed"`
	Hierarchical     bool           `json:"hierarchical"`
	AutomaticAllowed bool           `json:"automaticAllowed"`
}

// These five types reflect the current Phase 35 inventory. Do not introduce
// uplink_to/parent_of until an authoritative semantic and cycle policy exists.
// "hosts" and "matches_host" MUST be projected from their existing owners and
// are deliberately disabled for generic manual relationship writes.
var definitions = [...]Definition{
	{
		Type: TypeHosts, SourceTypes: []EntityType{EntityHost},
		TargetTypes: []EntityType{EntityWorkload}, Directionality: Directed,
		SourceLabel: "Hosts", InverseLabel: "Runs on",
		AutomaticAllowed: true,
	},
	{
		Type: TypeMatchesHost, SourceTypes: []EntityType{EntityWorkload},
		TargetTypes: []EntityType{EntityHost}, Directionality: Directed,
		SourceLabel: "Matches Host", InverseLabel: "Represented by workload",
		AutomaticAllowed: true,
	},
	{
		Type: TypeConnectedTo, SourceTypes: []EntityType{EntityHost},
		TargetTypes: []EntityType{EntityHost}, Directionality: Directed,
		SourceLabel: "Connected to", InverseLabel: "Connected devices",
		ManualAllowed: true, AutomaticAllowed: true,
	},
	{
		Type: TypeManages, SourceTypes: []EntityType{EntityHost},
		TargetTypes: []EntityType{EntityHost}, Directionality: Directed,
		SourceLabel: "Manages", InverseLabel: "Managed by",
		ManualAllowed: true, AutomaticAllowed: true,
	},
	{
		Type: TypeDependsOn, SourceTypes: []EntityType{EntityHost},
		TargetTypes: []EntityType{EntityHost}, Directionality: Directed,
		SourceLabel: "Depends on", InverseLabel: "Required by",
		ManualAllowed: true,
	},
}

// Definitions returns a defensive copy, so callers (including API layers)
// cannot mutate the shared registry through exported slices.
func Definitions() []Definition {
	out := make([]Definition, 0, len(definitions))
	for _, def := range definitions {
		out = append(out, cloneDefinition(def))
	}
	return out
}

func LookupType(value Type) (Definition, bool) {
	for _, def := range definitions {
		if def.Type == value {
			return cloneDefinition(def), true
		}
	}
	return Definition{}, false
}

func cloneDefinition(def Definition) Definition {
	def.SourceTypes = append([]EntityType(nil), def.SourceTypes...)
	def.TargetTypes = append([]EntityType(nil), def.TargetTypes...)
	return def
}

func containsEntityType(allowed []EntityType, entityType EntityType) bool {
	for _, candidate := range allowed {
		if candidate == entityType {
			return true
		}
	}
	return false
}

// LabelForEndpoint returns the applicable source/inverse text for a participant.
// It does not invert or persist a second assertion.
func LabelForEndpoint(assertion Assertion, endpoint Endpoint) (string, error) {
	source, target, err := ValidatePair(assertion.Type, assertion.Source, assertion.Target)
	if err != nil {
		return "", err
	}
	def, _ := LookupType(assertion.Type)
	switch endpoint {
	case source:
		return def.SourceLabel, nil
	case target:
		return def.InverseLabel, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrEndpointNotIncident, endpoint.Key())
	}
}
