package models

const (
	IdentityCorrelationConfirmed = "confirmed"
	IdentityCorrelationRejected  = "rejected"
)

// IdentityCorrelationDecision stores an explicit user decision about whether
// two MAC identities belong to the same physical device. It does not merge or
// rewrite either identity's observations, events, presence, or lifecycle.
type IdentityCorrelationDecision struct {
	ID        int    `gorm:"column:ID;primaryKey"`
	MacA      string `gorm:"column:MAC_A;index;uniqueIndex:idx_identity_correlation_pair,priority:1"`
	MacB      string `gorm:"column:MAC_B;index;uniqueIndex:idx_identity_correlation_pair,priority:2"`
	Decision  string `gorm:"column:DECISION;index"`
	CreatedAt string `gorm:"column:CREATED_AT"`
	UpdatedAt string `gorm:"column:UPDATED_AT"`
}
