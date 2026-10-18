package relationships

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxManualNoteRunes = 500

var (
	ErrInvalidEntityType  = errors.New("invalid relationship entity type")
	ErrInvalidEndpoint    = errors.New("invalid relationship endpoint")
	ErrInvalidType        = errors.New("invalid relationship type")
	ErrEndpointCombination = errors.New("relationship endpoint types are not allowed")
	ErrSelfRelationship   = errors.New("self relationship is not allowed")
	ErrManualNotAllowed   = errors.New("relationship type is not available through manual CRUD")
	ErrInvalidProvenance  = errors.New("invalid relationship provenance")
	ErrInvalidNote        = errors.New("invalid relationship note")
	ErrNoteTooLong        = errors.New("relationship note exceeds maximum length")
	ErrResolverRequired   = errors.New("relationship endpoint resolver is required")
	ErrEntityNotFound     = errors.New("relationship endpoint not found")
	ErrEndpointNotIncident = errors.New("endpoint is not part of relationship")
)

// EndpointExists is implemented by persistence against CURRENT inventory,
// never retained scanner/address history. Resolve inside the write transaction
// in Phase 38A.2 to prevent time-of-check/time-of-use races.
type EndpointExists func(endpoint Endpoint) (bool, error)

// ManualCreateInput intentionally has NO provenance, source instance or edge ID.
// API decoding must disallow unknown fields so clients cannot spoof ownership.
type ManualCreateInput struct {
	Type   Type     `json:"type"`
	Source Endpoint `json:"source"`
	Target Endpoint `json:"target"`
	Note   string   `json:"note,omitempty"`
}

// ValidatePair validates registry rules and returns canonical endpoints.
// Inverse storage is never required. Symmetric normalization is available for
// future predicates, but NO Phase 38A.1 registry type is symmetric.
func ValidatePair(kind Type, source, target Endpoint) (Endpoint, Endpoint, error) {
	if !IsValidEntityType(source.EntityType) || !IsValidEntityType(target.EntityType) {
		return Endpoint{}, Endpoint{}, ErrInvalidEntityType
	}
	if source.EntityID == 0 || target.EntityID == 0 {
		return Endpoint{}, Endpoint{}, ErrInvalidEndpoint
	}
	if source == target {
		return Endpoint{}, Endpoint{}, ErrSelfRelationship
	}
	def, ok := LookupType(kind)
	if !ok {
		return Endpoint{}, Endpoint{}, fmt.Errorf("%w: %q", ErrInvalidType, kind)
	}
	if !containsEntityType(def.SourceTypes, source.EntityType) ||
		!containsEntityType(def.TargetTypes, target.EntityType) {
		return Endpoint{}, Endpoint{}, fmt.Errorf("%w: %q (%s -> %s)", ErrEndpointCombination, kind, source.EntityType, target.EntityType)
	}
	return normalizeEndpoints(def.Directionality, source, target)
}

// normalizeEndpoints is deliberately directionality-aware. An actual symmetric
// type may only be added after defining its endpoint restrictions in registry.
func normalizeEndpoints(direction Directionality, source, target Endpoint) (Endpoint, Endpoint, error) {
	switch direction {
	case Directed:
		return source, target, nil
	case Symmetric:
		if source.EntityType > target.EntityType ||
			(source.EntityType == target.EntityType && source.EntityID > target.EntityID) {
			return target, source, nil
		}
		return source, target, nil
	default:
		return Endpoint{}, Endpoint{}, errors.New("invalid relationship directionality")
	}
}

// NewManualAssertion validates a manual relationship and sets its trusted
// provenance internally. Duplicate detection is delegated to the transactional
// uniqueness constraint in Phase 38A.2, using Assertion.IdentityKey semantics.
func NewManualAssertion(input ManualCreateInput, exists EndpointExists) (Assertion, error) {
	source, target, err := ValidatePair(input.Type, input.Source, input.Target)
	if err != nil {
		return Assertion{}, err
	}
	def, _ := LookupType(input.Type)
	if !def.ManualAllowed {
		return Assertion{}, fmt.Errorf("%w: %q", ErrManualNotAllowed, input.Type)
	}
	note := strings.TrimSpace(input.Note)
	if !utf8.ValidString(note) {
		return Assertion{}, ErrInvalidNote
	}
	if utf8.RuneCountInString(note) > MaxManualNoteRunes {
		return Assertion{}, ErrNoteTooLong
	}
	for _, r := range note {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return Assertion{}, ErrInvalidNote
		}
	}
	if exists == nil {
		return Assertion{}, ErrResolverRequired
	}
	for _, endpoint := range []Endpoint{source, target} {
		found, err := exists(endpoint)
		if err != nil {
			return Assertion{}, err
		}
		if !found {
			return Assertion{}, fmt.Errorf("%w: %s", ErrEntityNotFound, endpoint.Key())
		}
	}
	return Assertion{
		Type: input.Type, Source: source, Target: target,
		Provenance: ProvenanceManual, Note: note,
	}, nil
}
