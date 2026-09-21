package models

// NetworkManagementMode describes whether the device exposes managed network functionality.
type NetworkManagementMode string

const (
	NetworkManagementModeUnspecified NetworkManagementMode = ""
	NetworkManagementModeManaged     NetworkManagementMode = "managed"
	NetworkManagementModeUnmanaged   NetworkManagementMode = "unmanaged"
)

// IsValidNetworkManagementMode reports whether value can be persisted.
func IsValidNetworkManagementMode(value string) bool {
	return value == string(NetworkManagementModeUnspecified) ||
		value == string(NetworkManagementModeManaged) ||
		value == string(NetworkManagementModeUnmanaged)
}

// NetworkDeviceProfile stores manually managed network-device-specific inventory.
type NetworkDeviceProfile struct {
	Mac                 string `gorm:"column:MAC;primaryKey" json:"mac"`
	ManagementMode      string `gorm:"column:MANAGEMENT_MODE" json:"managementMode"`
	PhysicalPortCount   int    `gorm:"column:PHYSICAL_PORT_COUNT" json:"physicalPortCount"`
	PortCapabilityNotes string `gorm:"column:PORT_CAPABILITY_NOTES;type:text" json:"portCapabilityNotes"`
	UpdatedAt           string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}

// NetworkDeviceProfileUpdate describes a partial managed network profile update.
// PhysicalPortCount uses 0 for "not set"; positive values are explicit counts.
type NetworkDeviceProfileUpdate struct {
	ManagementMode      *string
	PhysicalPortCount   *int
	PortCapabilityNotes *string
}

// SystemDeviceProfile stores manually managed Server/NAS system inventory.
// Role remains intentionally lightweight; DeviceType continues to classify the Host.
type SystemDeviceProfile struct {
	Mac             string `gorm:"column:MAC;primaryKey" json:"mac"`
	Role            string `gorm:"column:ROLE" json:"role"`
	OperatingSystem string `gorm:"column:OPERATING_SYSTEM" json:"operatingSystem"`
	Version         string `gorm:"column:VERSION" json:"version"`
	UpdatedAt       string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}

// SystemDeviceProfileUpdate describes a partial managed system profile update.
type SystemDeviceProfileUpdate struct {
	Role            *string
	OperatingSystem *string
	Version         *string
}

// HypervisorPlatform is a manually selected hypervisor family.
type HypervisorPlatform string

const (
	HypervisorPlatformProxmoxVE  HypervisorPlatform = "proxmox-ve"
	HypervisorPlatformVMwareESXi HypervisorPlatform = "vmware-esxi"
	HypervisorPlatformHyperV     HypervisorPlatform = "hyper-v"
	HypervisorPlatformOther      HypervisorPlatform = "other"
)

// IsValidHypervisorPlatform reports whether value is supported by the managed profile.
func IsValidHypervisorPlatform(value string) bool {
	switch HypervisorPlatform(value) {
	case HypervisorPlatformProxmoxVE, HypervisorPlatformVMwareESXi, HypervisorPlatformHyperV, HypervisorPlatformOther:
		return true
	default:
		return false
	}
}

// HypervisorProfile stores manually managed hypervisor identity.
// Imported Proxmox state belongs to a separate integration/source model.
type HypervisorProfile struct {
	Mac         string `gorm:"column:MAC;primaryKey" json:"mac"`
	Platform    string `gorm:"column:PLATFORM;not null" json:"platform"`
	Version     string `gorm:"column:VERSION" json:"version"`
	NodeName    string `gorm:"column:NODE_NAME" json:"nodeName"`
	ClusterName string `gorm:"column:CLUSTER_NAME" json:"clusterName"`
	UpdatedAt   string `gorm:"column:UPDATED_AT" json:"updatedAt"`
}

// HypervisorProfileUpdate describes a partial managed hypervisor profile update.
type HypervisorProfileUpdate struct {
	Platform    *string
	Version     *string
	NodeName    *string
	ClusterName *string
}
