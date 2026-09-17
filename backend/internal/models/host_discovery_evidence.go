package models

const (
	DiscoverySourceScanner        = "scanner"
	DiscoverySourceReverseDNS     = "reverse-dns"
	DiscoverySourceSystemResolver = "system-resolver"
	DiscoverySourceMDNS           = "mdns"
	DiscoverySourceSSDP           = "ssdp"

	DiscoveryKindVendor       = "vendor"
	DiscoveryKindHostname     = "hostname"
	DiscoveryKindFriendlyName = "friendly-name"
	DiscoveryKindManufacturer = "manufacturer"
	DiscoveryKindModel        = "model"
	DiscoveryKindModelNumber  = "model-number"
)

// HostDiscoveryEvidence stores discovered identity data together with its provenance.
// Evidence is observational only and must not overwrite manually maintained inventory fields.
type HostDiscoveryEvidence struct {
	ID        int    `gorm:"column:ID;primaryKey"`
	Mac       string `gorm:"column:MAC;index;uniqueIndex:idx_discovery_evidence_identity,priority:1"`
	Address   string `gorm:"column:ADDRESS;index;uniqueIndex:idx_discovery_evidence_identity,priority:2"`
	Source    string `gorm:"column:SOURCE;index;uniqueIndex:idx_discovery_evidence_identity,priority:3"`
	Kind      string `gorm:"column:KIND;index;uniqueIndex:idx_discovery_evidence_identity,priority:4"`
	Value     string `gorm:"column:VALUE;uniqueIndex:idx_discovery_evidence_identity,priority:5"`
	FirstSeen string `gorm:"column:FIRST_SEEN"`
	LastSeen  string `gorm:"column:LAST_SEEN"`
	Active    bool   `gorm:"column:ACTIVE;index"`
}
