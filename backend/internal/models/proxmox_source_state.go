package models

// ProxmoxSourceState stores imported/discovered Proxmox node state separately
// from the manually managed HypervisorProfile.
type ProxmoxSourceState struct {
	HypervisorMac   string `gorm:"column:HYPERVISOR_MAC;primaryKey" json:"hypervisorMac"`
	Source          string `gorm:"column:SOURCE;primaryKey" json:"source"`
	SchemaVersion   int    `gorm:"column:SCHEMA_VERSION" json:"schemaVersion"`
	CollectorVersion string `gorm:"column:COLLECTOR_VERSION" json:"collectorVersion"`
	CollectedAt     string `gorm:"column:COLLECTED_AT;index" json:"collectedAt"`
	Complete        bool   `gorm:"column:COMPLETE" json:"complete"`
	NodeHostname    string `gorm:"column:NODE_HOSTNAME" json:"nodeHostname"`
	NodePVEVersion  string `gorm:"column:NODE_PVE_VERSION" json:"nodePveVersion"`
	NodeClusterName string `gorm:"column:NODE_CLUSTER_NAME" json:"nodeClusterName"`
	NodeStatus      string `gorm:"column:NODE_STATUS" json:"nodeStatus"`
	SnapshotDigest  string `gorm:"column:SNAPSHOT_DIGEST" json:"snapshotDigest"`
	ImportedAt      string `gorm:"column:IMPORTED_AT" json:"importedAt"`
}
