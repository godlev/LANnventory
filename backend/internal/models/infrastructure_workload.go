package models

const (
	InfrastructureWorkloadTypeVM        = "vm"
	InfrastructureWorkloadTypeContainer = "container"

	InfrastructureWorkloadStatusUnknown = "unknown"
	InfrastructureWorkloadStatusRunning = "running"
	InfrastructureWorkloadStatusStopped = "stopped"

	InfrastructureWorkloadSourceManual       = "manual"
	InfrastructureWorkloadSourceScriptImport = "script-import"

	InfrastructureWorkloadLinkSourceManual   = "manual"
	InfrastructureWorkloadLinkSourceExactMAC = "exact-mac"
)

// InfrastructureWorkload represents a hypervisor-owned VM or container.
// Workloads are inventory objects, not LANnventory Hosts.
type InfrastructureWorkload struct {
	ID            uint   `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	HypervisorMac string `gorm:"column:HYPERVISOR_MAC;not null;index;uniqueIndex:idx_infrastructure_workload_identity,priority:1" json:"hypervisorMac"`
	NativeID      string `gorm:"column:NATIVE_ID;not null;uniqueIndex:idx_infrastructure_workload_identity,priority:2" json:"nativeId"`
	WorkloadType  string `gorm:"column:WORKLOAD_TYPE;not null;uniqueIndex:idx_infrastructure_workload_identity,priority:3" json:"workloadType"`
	Name          string `gorm:"column:NAME" json:"name"`
	Status        string `gorm:"column:STATUS;index" json:"status"`
	Source        string `gorm:"column:SOURCE;index" json:"source"`
	FirstSeen     string `gorm:"column:FIRST_SEEN" json:"firstSeen"`
	LastSeen      string `gorm:"column:LAST_SEEN" json:"lastSeen"`
	RetiredAt     string `gorm:"column:RETIRED_AT;index" json:"retiredAt"`
	UpdatedAt     string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}

// InfrastructureWorkloadInterface stores allowlisted workload network identity.
// It deliberately excludes raw guest configuration.
type InfrastructureWorkloadInterface struct {
	ID                uint   `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	WorkloadID        uint   `gorm:"column:WORKLOAD_ID;not null;index;uniqueIndex:idx_infrastructure_workload_interface,priority:1" json:"workloadId"`
	Name              string `gorm:"column:NAME;not null;uniqueIndex:idx_infrastructure_workload_interface,priority:2" json:"name"`
	Mac               string `gorm:"column:MAC;index" json:"mac"`
	Bridge            string `gorm:"column:BRIDGE" json:"bridge"`
	VLANTag           string `gorm:"column:VLAN_TAG" json:"vlanTag"`
	ConfiguredAddress string `gorm:"column:CONFIGURED_ADDRESS" json:"configuredAddress"`
	ConfiguredNetwork string `gorm:"column:CONFIGURED_NETWORK" json:"configuredNetwork"`
	UpdatedAt         string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}

// InfrastructureWorkloadHostLink is the explicit logical MATCHES relation.
// A workload can match at most one current LANnventory Host at a time.
type InfrastructureWorkloadHostLink struct {
	WorkloadID uint   `gorm:"column:WORKLOAD_ID;primaryKey" json:"workloadId"`
	HostID     int    `gorm:"column:HOST_ID;index" json:"hostId"`
	HostMac    string `gorm:"column:HOST_MAC;index" json:"hostMac"`
	LinkSource string `gorm:"column:LINK_SOURCE;index" json:"linkSource"`
	LinkedAt   string `gorm:"column:LINKED_AT" json:"linkedAt"`
	UpdatedAt  string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}

// InfrastructureWorkloadRecord is the aggregate persistence read model.
type InfrastructureWorkloadRecord struct {
	Workload   InfrastructureWorkload
	Interfaces []InfrastructureWorkloadInterface
	Link       *InfrastructureWorkloadHostLink
}

// InfrastructureWorkloadUpsert is a normalized write model used by manual and
// future import sources.
type InfrastructureWorkloadUpsert struct {
	NativeID     string
	WorkloadType string
	Name         string
	Status       string
	Source       string
	Interfaces   []InfrastructureWorkloadInterface
}


// InfrastructureWorkloadMembershipRecord is a read-only reverse projection of
// an existing workload MATCHES relation. It does not create a Host hierarchy or
// change either the Host or workload record.
type InfrastructureWorkloadMembershipRecord struct {
	Workload InfrastructureWorkload
	Link     InfrastructureWorkloadHostLink
}
