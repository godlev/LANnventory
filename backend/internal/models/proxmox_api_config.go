package models

// ProxmoxAPIConfig stores one optional API connection per managed Proxmox
// hypervisor identity. TokenSecret is intentionally write-only at API surfaces
// and excluded from JSON serialization.
type ProxmoxAPIConfig struct {
	HypervisorMac      string `gorm:"column:HYPERVISOR_MAC;primaryKey" json:"hypervisorMac"`
	Enabled            bool   `gorm:"column:ENABLED;not null" json:"enabled"`
	BaseURL            string `gorm:"column:BASE_URL" json:"baseUrl"`
	TokenID            string `gorm:"column:TOKEN_ID" json:"tokenId"`
	TokenSecret        string `gorm:"column:TOKEN_SECRET" json:"-"`
	VerifyTLS          bool   `gorm:"column:VERIFY_TLS;not null" json:"verifyTls"`
	TimeoutSeconds     int    `gorm:"column:TIMEOUT_SECONDS;not null" json:"timeoutSeconds"`
	LastAttemptAt      string `gorm:"column:LAST_ATTEMPT_AT" json:"lastAttemptAt,omitempty"`
	LastSuccessfulSync string `gorm:"column:LAST_SUCCESSFUL_SYNC" json:"lastSuccessfulSync,omitempty"`
	LastError          string `gorm:"column:LAST_ERROR" json:"lastError,omitempty"`
	Status             string `gorm:"column:STATUS" json:"status,omitempty"`
	UpdatedAt          string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}
