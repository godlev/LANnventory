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
	TimeoutSeconds              int    `gorm:"column:TIMEOUT_SECONDS;not null" json:"timeoutSeconds"`
	AutomaticSync               bool   `gorm:"column:AUTOMATIC_SYNC;not null;default:false" json:"automaticSync"`
	SyncIntervalMinutes         int    `gorm:"column:SYNC_INTERVAL_MINUTES;not null;default:60" json:"syncIntervalMinutes"`
	ConfigRevision              uint64 `gorm:"column:CONFIG_REVISION;not null;default:0" json:"configRevision"`
	LastSyncAttemptAt           string `gorm:"column:LAST_SYNC_ATTEMPT_AT" json:"lastSyncAttemptAt,omitempty"`
	LastSuccessfulCollectionAt  string `gorm:"column:LAST_SUCCESSFUL_COLLECTION_AT" json:"lastSuccessfulCollectionAt,omitempty"`
	NextSyncAt                  string `gorm:"column:NEXT_SYNC_AT" json:"nextSyncAt,omitempty"`
	LastSyncStatus              string `gorm:"column:LAST_SYNC_STATUS" json:"lastSyncStatus,omitempty"`
	LastSyncError               string `gorm:"column:LAST_SYNC_ERROR" json:"lastSyncError,omitempty"`
	LastSyncTrigger             string `gorm:"column:LAST_SYNC_TRIGGER" json:"lastSyncTrigger,omitempty"`
	LastAttemptAt               string `gorm:"column:LAST_ATTEMPT_AT" json:"lastAttemptAt,omitempty"`
	LastSuccessfulSync          string `gorm:"column:LAST_SUCCESSFUL_SYNC" json:"lastSuccessfulSync,omitempty"`
	LastError                   string `gorm:"column:LAST_ERROR" json:"lastError,omitempty"`
	Status                      string `gorm:"column:STATUS" json:"status,omitempty"`
	UpdatedAt                   string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}
