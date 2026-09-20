package models

// InfrastructureWorkloadCandidateRejection records an explicit user decision
// that one current Host suggestion is not the workload represented by a
// Proxmox VM/LXC. The decision is scoped to the evidence fingerprint that was
// visible when the user rejected it; changed evidence is intentionally
// re-evaluated instead of being suppressed forever.
type InfrastructureWorkloadCandidateRejection struct {
	WorkloadID          uint   `gorm:"column:WORKLOAD_ID;primaryKey;autoIncrement:false" json:"workloadId"`
	HostID              int    `gorm:"column:HOST_ID;primaryKey;autoIncrement:false;index" json:"hostId"`
	HostMac             string `gorm:"column:HOST_MAC;not null;index" json:"hostMac"`
	EvidenceFingerprint string `gorm:"column:EVIDENCE_FINGERPRINT;not null" json:"evidenceFingerprint"`
	EvidenceStrength    string `gorm:"column:EVIDENCE_STRENGTH;not null" json:"evidenceStrength"`
	CreatedAt           string `gorm:"column:CREATED_AT" json:"createdAt"`
	UpdatedAt           string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}
