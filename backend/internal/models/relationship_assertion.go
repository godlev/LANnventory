package models

// RelationshipAssertion persists a source-owned assertion between existing
// inventory entities. HOSTS/MATCHES_HOST are derived from existing Phase 35
// tables, never copied into this writable table by the Proxmox reconciliation.
//
// The composite unique index prevents duplicate claims by the same source
// instance. Claims from different provenance/instances stay independent.
type RelationshipAssertion struct {
	ID               uint64 `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	RelationshipType string `gorm:"column:RELATIONSHIP_TYPE;not null;uniqueIndex:idx_relationship_assertion_identity,priority:1;index:idx_relationship_type" json:"type"`
	SourceEntityType string `gorm:"column:SOURCE_ENTITY_TYPE;not null;uniqueIndex:idx_relationship_assertion_identity,priority:2;index:idx_relationship_source,priority:1" json:"sourceEntityType"`
	SourceEntityID   uint64 `gorm:"column:SOURCE_ENTITY_ID;not null;uniqueIndex:idx_relationship_assertion_identity,priority:3;index:idx_relationship_source,priority:2" json:"sourceEntityId"`
	TargetEntityType string `gorm:"column:TARGET_ENTITY_TYPE;not null;uniqueIndex:idx_relationship_assertion_identity,priority:4;index:idx_relationship_target,priority:1" json:"targetEntityType"`
	TargetEntityID   uint64 `gorm:"column:TARGET_ENTITY_ID;not null;uniqueIndex:idx_relationship_assertion_identity,priority:5;index:idx_relationship_target,priority:2" json:"targetEntityId"`
	Provenance       string `gorm:"column:PROVENANCE;not null;uniqueIndex:idx_relationship_assertion_identity,priority:6" json:"provenance"`
	SourceInstance   string `gorm:"column:SOURCE_INSTANCE;not null;uniqueIndex:idx_relationship_assertion_identity,priority:7" json:"sourceInstance,omitempty"`
	SourceDetail     string `gorm:"column:SOURCE_DETAIL" json:"sourceDetail,omitempty"`
	Note             string `gorm:"column:NOTE" json:"note,omitempty"`
	CreatedAt        string `gorm:"column:CREATED_AT;not null" json:"createdAt"`
	UpdatedAt        string `gorm:"column:UPDATED_AT;not null" json:"updatedAt"`
}
