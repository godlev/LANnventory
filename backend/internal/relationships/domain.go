// Package relationships defines canonical relationship semantics without
// coupling inventory ownership to persistence or a topology visualization.
//
// Proxmox HOSTS and MATCHES_HOST relationships remain owned by their existing
// Phase 35 tables; this package does not write or reconcile those records.
package relationships

import (
	"fmt"
	"strconv"
)

// EntityType identifies an existing first-class inventory entity.
// A hypervisor is a Host with a hypervisor profile, not a separate entity.
type EntityType string

const (
	EntityHost     EntityType = "host"
	EntityWorkload EntityType = "workload"
)

// Endpoint refers to an existing inventory entity by its internal primary key.
// Workloads must not be projected into fake LAN Host records.
type Endpoint struct {
	EntityType EntityType `json:"entityType"`
	EntityID   uint64     `json:"entityId"`
}

// Key is stable for the lifetime of this inventory record. Deleted/recreated
// Hosts are not automatically rebound to the old record's relationships.
func (e Endpoint) Key() string {
	return string(e.EntityType) + ":" + strconv.FormatUint(e.EntityID, 10)
}

func IsValidEntityType(value EntityType) bool {
	return value == EntityHost || value == EntityWorkload
}

// Type is a canonical relationship predicate, independent of display labels.
type Type string

const (
	TypeHosts       Type = "hosts"
	TypeMatchesHost Type = "matches_host"
	TypeConnectedTo Type = "connected_to"
	TypeManages     Type = "manages"
	TypeDependsOn   Type = "depends_on"
)

// Directionality is part of the domain contract, not a UI interpretation.
// There are no enabled symmetric predicates in Phase 38A.1.
type Directionality string

const (
	Directed  Directionality = "directed"
	Symmetric Directionality = "symmetric"
)

// Provenance identifies who asserts a relationship. SourceDetail can identify
// a specific collection mechanism such as proxmox-api or exact-mac.
type Provenance string

const (
	ProvenanceManual  Provenance = "manual"
	ProvenanceProxmox Provenance = "proxmox"
	ProvenanceSystem  Provenance = "system"
)

func IsValidProvenance(value Provenance) bool {
	switch value {
	case ProvenanceManual, ProvenanceProxmox, ProvenanceSystem:
		return true
	default:
		return false
	}
}

// Assertion is a source-owned relationship claim, not a reconciled effective
// physical topology. SourceInstance is reserved for a specific future source
// instance (for example, one router); no caller controls it through manual CRUD.
// IDs and timestamps are assigned by persistence in Phase 38A.2.
type Assertion struct {
	ID             uint64     `json:"id,omitempty"`
	Type           Type       `json:"type"`
	Source         Endpoint   `json:"source"`
	Target         Endpoint   `json:"target"`
	Provenance     Provenance `json:"provenance"`
	SourceDetail   string     `json:"sourceDetail,omitempty"`
	SourceInstance string     `json:"sourceInstance,omitempty"`
	Note           string     `json:"note,omitempty"`
	CreatedAt      string     `json:"createdAt,omitempty"`
	UpdatedAt      string     `json:"updatedAt,omitempty"`
}

// IdentityKey is a canonical collision key for one source assertion.
// It excludes notes/timestamps and never stores a duplicate inverse edge.
// Database uniqueness and concurrent-write handling belong to Phase 38A.2.
func (a Assertion) IdentityKey() (string, error) {
	source, target, err := ValidatePair(a.Type, a.Source, a.Target)
	if err != nil {
		return "", err
	}
	if !IsValidProvenance(a.Provenance) {
		return "", fmt.Errorf("%w: %q", ErrInvalidProvenance, a.Provenance)
	}
	def, _ := LookupType(a.Type)
	if a.Provenance == ProvenanceManual && !def.ManualAllowed {
		return "", fmt.Errorf("%w: %q", ErrManualNotAllowed, a.Type)
	}
	return fmt.Sprintf("%q|%q|%q|%q|%q", a.Type, source.Key(), target.Key(), a.Provenance, a.SourceInstance), nil
}
