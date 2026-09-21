package proxmoxsnapshot

const (
	SchemaVersion     = 1
	CollectorVersion  = "1.0.0"
	SourceScriptImport = "script-import"
	SourceProxmoxAPI  = "proxmox-api"
)

// Snapshot is the normalized contract shared by the read-only script collector
// and future Proxmox API collection. Validation and preview/diff are layered on
// top of this contract in the following Phase 35 checkpoints.
type Snapshot struct {
	SchemaVersion     int                `json:"schemaVersion"`
	CollectorVersion  string             `json:"collectorVersion"`
	Source            string             `json:"source"`
	CollectedAt       string             `json:"collectedAt"`
	Complete          bool               `json:"complete"`
	CollectionErrors  []string           `json:"collectionErrors,omitempty"`
	Node              NodeSnapshot       `json:"node"`
	Workloads         []WorkloadSnapshot `json:"workloads"`
}

// NodeSnapshot contains basic Proxmox node identity only.
type NodeSnapshot struct {
	Hostname    string `json:"hostname"`
	PVEVersion  string `json:"pveVersion"`
	ClusterName string `json:"clusterName,omitempty"`
	Status      string `json:"status"`
}

// WorkloadSnapshot is a VM/LXC inventory object, not a LANnventory Host.
type WorkloadSnapshot struct {
	NativeID     string              `json:"nativeId"`
	WorkloadType string              `json:"workloadType"`
	NodeName     string              `json:"nodeName,omitempty"`
	Name         string              `json:"name"`
	Status       string              `json:"status"`
	Interfaces   []InterfaceSnapshot `json:"interfaces"`
}

// InterfaceSnapshot contains only explicitly allowlisted network identity.
type InterfaceSnapshot struct {
	Name              string `json:"name"`
	Mac               string `json:"mac,omitempty"`
	Bridge            string `json:"bridge,omitempty"`
	VLANTag           string `json:"vlanTag,omitempty"`
	ConfiguredAddress string `json:"configuredAddress,omitempty"`
	ConfiguredNetwork string `json:"configuredNetwork,omitempty"`
}
