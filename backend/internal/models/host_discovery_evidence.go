package models

const (
	DiscoverySourceScanner    = "scanner"
	DiscoverySourceReverseDNS = "reverse-dns"

	DiscoveryKindVendor   = "vendor"
	DiscoveryKindHostname = "hostname"
)

// HostDiscoveryEvidence stores discovered identity data together with its provenance.
// Evidence is observational only and must not overwrite manually maintained inventory fields.
type HostDiscoveryEvidence struct {
	ID        int    `gorm:"column:ID;primaryKey"`
	Mac       string `gorm:"column:MAC;index"`
	Address   string `gorm:"column:ADDRESS;index"`
	Source    string `gorm:"column:SOURCE;index"`
	Kind      string `gorm:"column:KIND;index"`
	Value     string `gorm:"column:VALUE"`
	FirstSeen string `gorm:"column:FIRST_SEEN"`
	LastSeen  string `gorm:"column:LAST_SEEN"`
	Active    bool   `gorm:"column:ACTIVE;index"`
}
