package backup

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

const (
	Format        = "lannventory-backup"
	FormatVersion = 5
)

// Document is the stable, versioned logical backup format.
type Document struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"formatVersion"`
	CreatedAt     string `json:"createdAt"`
	AppVersion    string `json:"appVersion"`
	Data          Data   `json:"data"`
}

// Data contains persisted application data only. It intentionally excludes
// configuration and secrets so exports are portable across database backends.
type Data struct {
	CurrentHosts          []Host                 `json:"currentHosts"`
	History               []Host                 `json:"history"`
	Events                []Event                `json:"events"`
	HostMetadata          []HostMetadata         `json:"hostMetadata"`
	HostLifecycle         []HostLifecycle        `json:"hostLifecycle"`
	DeviceProfiles        []DeviceProfile        `json:"deviceProfiles"`
	NetworkDeviceProfiles []NetworkDeviceProfile `json:"networkDeviceProfiles"`
	SystemDeviceProfiles  []SystemDeviceProfile  `json:"systemDeviceProfiles"`
	HypervisorProfiles             []HypervisorProfile             `json:"hypervisorProfiles"`
	InfrastructureWorkloads        []InfrastructureWorkload        `json:"infrastructureWorkloads"`
	InfrastructureWorkloadInterfaces []InfrastructureWorkloadInterface `json:"infrastructureWorkloadInterfaces"`
	InfrastructureWorkloadHostLinks []InfrastructureWorkloadHostLink `json:"infrastructureWorkloadHostLinks"`
}

// Host mirrors the currently persisted host columns in the now/history tables.
type Host struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	DNS        string `json:"dns"`
	Iface      string `json:"iface"`
	IP         string `json:"ip"`
	Mac        string `json:"mac"`
	Hw         string `json:"hw"`
	Date       string `json:"date"`
	Known      int    `json:"known"`
	Now        int    `json:"now"`
	DeviceType string `json:"deviceType"`
}

// Event mirrors the currently persisted event columns. DateUTC is intentionally
// omitted because it is derived display data, not persisted source data.
type Event struct {
	ID         int    `json:"id"`
	HostID     int    `json:"hostId"`
	Mac        string `json:"mac"`
	Name       string `json:"name"`
	EventType  string `json:"eventType"`
	Date       string `json:"date"`
	IP         string `json:"ip"`
	Iface      string `json:"iface"`
	DeviceType string `json:"deviceType"`
	OldValue   string `json:"oldValue"`
	NewValue   string `json:"newValue"`
}

// HostMetadata is the portable metadata backup representation.
type HostMetadata struct {
	Mac      string   `json:"mac"`
	Owner    string   `json:"owner"`
	Location string   `json:"location"`
	Notes    string   `json:"notes"`
	Tags     []string `json:"tags"`
	Pinned   bool     `json:"pinned"`
}

// DeviceProfile is the portable managed device-profile backup representation.
type DeviceProfile struct {
	Mac               string `json:"mac"`
	Manufacturer      string `json:"manufacturer"`
	Model             string `json:"model"`
	ManagementAddress string `json:"managementAddress"`
	UpdatedAt         string `json:"updatedAt"`
}

// NetworkDeviceProfile is the portable managed network specialization.
type NetworkDeviceProfile struct {
	Mac                 string `json:"mac"`
	ManagementMode      string `json:"managementMode"`
	PhysicalPortCount   int    `json:"physicalPortCount"`
	PortCapabilityNotes string `json:"portCapabilityNotes"`
	UpdatedAt           string `json:"updatedAt"`
}

// SystemDeviceProfile is the portable managed system specialization.
type SystemDeviceProfile struct {
	Mac             string `json:"mac"`
	Role            string `json:"role"`
	OperatingSystem string `json:"operatingSystem"`
	Version         string `json:"version"`
	UpdatedAt       string `json:"updatedAt"`
}

// HypervisorProfile is the portable managed hypervisor specialization.
type HypervisorProfile struct {
	Mac         string `json:"mac"`
	Platform    string `json:"platform"`
	Version     string `json:"version"`
	NodeName    string `json:"nodeName"`
	ClusterName string `json:"clusterName"`
	UpdatedAt   string `json:"updatedAt"`
}

// InfrastructureWorkload is the portable hypervisor workload representation.
type InfrastructureWorkload struct {
	ID            uint   `json:"id"`
	HypervisorMac string `json:"hypervisorMac"`
	NativeID      string `json:"nativeId"`
	WorkloadType  string `json:"workloadType"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Source        string `json:"source"`
	FirstSeen     string `json:"firstSeen"`
	LastSeen      string `json:"lastSeen"`
	RetiredAt     string `json:"retiredAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// InfrastructureWorkloadInterface is the portable allowlisted workload network representation.
type InfrastructureWorkloadInterface struct {
	ID                uint   `json:"id"`
	WorkloadID        uint   `json:"workloadId"`
	Name              string `json:"name"`
	Mac               string `json:"mac"`
	Bridge            string `json:"bridge"`
	VLANTag           string `json:"vlanTag"`
	ConfiguredAddress string `json:"configuredAddress"`
	ConfiguredNetwork string `json:"configuredNetwork"`
	UpdatedAt         string `json:"updatedAt"`
}

// InfrastructureWorkloadHostLink is the portable logical MATCHES relation.
type InfrastructureWorkloadHostLink struct {
	WorkloadID uint   `json:"workloadId"`
	HostID     int    `json:"hostId"`
	HostMac    string `json:"hostMac"`
	LinkSource string `json:"linkSource"`
	LinkedAt   string `json:"linkedAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// HostLifecycle is the portable lifecycle backup representation.
type HostLifecycle struct {
	Mac                string `json:"mac"`
	FirstSeen          string `json:"firstSeen"`
	LastSeen           string `json:"lastSeen"`
	FirstSeenEstimated bool   `json:"firstSeenEstimated"`
}

// InventoryHost is the current-inventory CSV representation.
type InventoryHost struct {
	Host
	Owner              string
	Location           string
	Notes              string
	Tags               []string
	Pinned             bool
	FirstSeen          string
	FirstSeenEstimated bool
	LastSeen           string
}

var InventoryCSVHeader = []string{
	"ID",
	"Name",
	"DNS",
	"Iface",
	"IP",
	"Mac",
	"Hw",
	"Date",
	"Known",
	"Now",
	"DeviceType",
	"Owner",
	"Location",
	"Notes",
	"Tags",
	"Pinned",
	"FirstSeen",
	"FirstSeenEstimated",
	"LastSeen",
}

func NewDocument(data Data, appVersion string, createdAt time.Time) Document {
	return Document{
		Format:        Format,
		FormatVersion: FormatVersion,
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		AppVersion:    appVersion,
		Data:          normalizeData(data),
	}
}

func DataFromModels(currentHosts, history []models.Host, events []models.HostEvent, hostMetadata []models.HostMetadata, hostLifecycle []models.HostLifecycle, deviceProfiles []models.DeviceProfile, networkProfiles []models.NetworkDeviceProfile, systemProfiles []models.SystemDeviceProfile, hypervisorProfiles []models.HypervisorProfile, workloads []models.InfrastructureWorkload, workloadInterfaces []models.InfrastructureWorkloadInterface, workloadLinks []models.InfrastructureWorkloadHostLink) Data {
	data := Data{
		CurrentHosts:          make([]Host, 0, len(currentHosts)),
		History:               make([]Host, 0, len(history)),
		Events:                make([]Event, 0, len(events)),
		HostMetadata:          make([]HostMetadata, 0, len(hostMetadata)),
		HostLifecycle:         make([]HostLifecycle, 0, len(hostLifecycle)),
		DeviceProfiles:        make([]DeviceProfile, 0, len(deviceProfiles)),
		NetworkDeviceProfiles: make([]NetworkDeviceProfile, 0, len(networkProfiles)),
		SystemDeviceProfiles:  make([]SystemDeviceProfile, 0, len(systemProfiles)),
		HypervisorProfiles:               make([]HypervisorProfile, 0, len(hypervisorProfiles)),
		InfrastructureWorkloads:          make([]InfrastructureWorkload, 0, len(workloads)),
		InfrastructureWorkloadInterfaces: make([]InfrastructureWorkloadInterface, 0, len(workloadInterfaces)),
		InfrastructureWorkloadHostLinks:  make([]InfrastructureWorkloadHostLink, 0, len(workloadLinks)),
	}

	for _, host := range currentHosts {
		data.CurrentHosts = append(data.CurrentHosts, HostFromModel(host))
	}
	for _, host := range history {
		data.History = append(data.History, HostFromModel(host))
	}
	for _, event := range events {
		data.Events = append(data.Events, EventFromModel(event))
	}
	for _, metadata := range hostMetadata {
		data.HostMetadata = append(data.HostMetadata, HostMetadataFromModel(metadata))
	}
	for _, lifecycle := range hostLifecycle {
		data.HostLifecycle = append(data.HostLifecycle, HostLifecycleFromModel(lifecycle))
	}
	for _, profile := range deviceProfiles {
		data.DeviceProfiles = append(data.DeviceProfiles, DeviceProfileFromModel(profile))
	}
	for _, profile := range networkProfiles {
		data.NetworkDeviceProfiles = append(data.NetworkDeviceProfiles, NetworkDeviceProfileFromModel(profile))
	}
	for _, profile := range systemProfiles {
		data.SystemDeviceProfiles = append(data.SystemDeviceProfiles, SystemDeviceProfileFromModel(profile))
	}
	for _, profile := range hypervisorProfiles {
		data.HypervisorProfiles = append(data.HypervisorProfiles, HypervisorProfileFromModel(profile))
	}
	for _, workload := range workloads {
		data.InfrastructureWorkloads = append(data.InfrastructureWorkloads, InfrastructureWorkloadFromModel(workload))
	}
	for _, iface := range workloadInterfaces {
		data.InfrastructureWorkloadInterfaces = append(data.InfrastructureWorkloadInterfaces, InfrastructureWorkloadInterfaceFromModel(iface))
	}
	for _, link := range workloadLinks {
		data.InfrastructureWorkloadHostLinks = append(data.InfrastructureWorkloadHostLinks, InfrastructureWorkloadHostLinkFromModel(link))
	}

	return data
}

func HostFromModel(host models.Host) Host {
	return Host{
		ID:         host.ID,
		Name:       host.Name,
		DNS:        host.DNS,
		Iface:      host.Iface,
		IP:         host.IP,
		Mac:        host.Mac,
		Hw:         host.Hw,
		Date:       host.Date,
		Known:      host.Known,
		Now:        host.Now,
		DeviceType: host.DeviceType,
	}
}

func EventFromModel(event models.HostEvent) Event {
	return Event{
		ID:         event.ID,
		HostID:     event.HostID,
		Mac:        event.Mac,
		Name:       event.Name,
		EventType:  event.EventType,
		Date:       event.Date,
		IP:         event.IP,
		Iface:      event.Iface,
		DeviceType: event.DeviceType,
		OldValue:   event.OldValue,
		NewValue:   event.NewValue,
	}
}

func HostMetadataFromModel(metadata models.HostMetadata) HostMetadata {
	return HostMetadata{
		Mac:      metadata.Mac,
		Owner:    metadata.Owner,
		Location: metadata.Location,
		Notes:    metadata.Notes,
		Tags:     models.DecodeMetadataTags(metadata.TagsJSON),
		Pinned:   metadata.Pinned,
	}
}

func DeviceProfileFromModel(profile models.DeviceProfile) DeviceProfile {
	return DeviceProfile{
		Mac:               profile.Mac,
		Manufacturer:      profile.Manufacturer,
		Model:             profile.Model,
		ManagementAddress: profile.ManagementAddress,
		UpdatedAt:         profile.UpdatedAt,
	}
}

func NetworkDeviceProfileFromModel(profile models.NetworkDeviceProfile) NetworkDeviceProfile {
	return NetworkDeviceProfile{
		Mac:                 profile.Mac,
		ManagementMode:      profile.ManagementMode,
		PhysicalPortCount:   profile.PhysicalPortCount,
		PortCapabilityNotes: profile.PortCapabilityNotes,
		UpdatedAt:           profile.UpdatedAt,
	}
}

func SystemDeviceProfileFromModel(profile models.SystemDeviceProfile) SystemDeviceProfile {
	return SystemDeviceProfile{
		Mac:             profile.Mac,
		Role:            profile.Role,
		OperatingSystem: profile.OperatingSystem,
		Version:         profile.Version,
		UpdatedAt:       profile.UpdatedAt,
	}
}

func HypervisorProfileFromModel(profile models.HypervisorProfile) HypervisorProfile {
	return HypervisorProfile{
		Mac:         profile.Mac,
		Platform:    profile.Platform,
		Version:     profile.Version,
		NodeName:    profile.NodeName,
		ClusterName: profile.ClusterName,
		UpdatedAt:   profile.UpdatedAt,
	}
}

func InfrastructureWorkloadFromModel(workload models.InfrastructureWorkload) InfrastructureWorkload {
	return InfrastructureWorkload{
		ID:            workload.ID,
		HypervisorMac: workload.HypervisorMac,
		NativeID:      workload.NativeID,
		WorkloadType:  workload.WorkloadType,
		Name:          workload.Name,
		Status:        workload.Status,
		Source:        workload.Source,
		FirstSeen:     workload.FirstSeen,
		LastSeen:      workload.LastSeen,
		RetiredAt:     workload.RetiredAt,
		UpdatedAt:     workload.UpdatedAt,
	}
}

func InfrastructureWorkloadInterfaceFromModel(iface models.InfrastructureWorkloadInterface) InfrastructureWorkloadInterface {
	return InfrastructureWorkloadInterface{
		ID:                iface.ID,
		WorkloadID:        iface.WorkloadID,
		Name:              iface.Name,
		Mac:               iface.Mac,
		Bridge:            iface.Bridge,
		VLANTag:           iface.VLANTag,
		ConfiguredAddress: iface.ConfiguredAddress,
		ConfiguredNetwork: iface.ConfiguredNetwork,
		UpdatedAt:         iface.UpdatedAt,
	}
}

func InfrastructureWorkloadHostLinkFromModel(link models.InfrastructureWorkloadHostLink) InfrastructureWorkloadHostLink {
	return InfrastructureWorkloadHostLink{
		WorkloadID: link.WorkloadID,
		HostID:     link.HostID,
		HostMac:    link.HostMac,
		LinkSource: link.LinkSource,
		LinkedAt:   link.LinkedAt,
		UpdatedAt:  link.UpdatedAt,
	}
}

func HostLifecycleFromModel(lifecycle models.HostLifecycle) HostLifecycle {
	return HostLifecycle{
		Mac:                lifecycle.Mac,
		FirstSeen:          lifecycle.FirstSeen,
		LastSeen:           lifecycle.LastSeen,
		FirstSeenEstimated: lifecycle.FirstSeenEstimated,
	}
}

func InventoryHostFromModel(host models.Host) InventoryHost {
	return InventoryHost{
		Host:               HostFromModel(host),
		Owner:              host.Owner,
		Location:           host.Location,
		Notes:              host.Notes,
		Tags:               host.Tags,
		Pinned:             host.Pinned,
		FirstSeen:          host.FirstSeen,
		FirstSeenEstimated: host.FirstSeenEstimated,
		LastSeen:           host.LastSeen,
	}
}

func WriteInventoryCSV(writer io.Writer, hosts []InventoryHost) error {
	csvWriter := csv.NewWriter(writer)

	if err := csvWriter.Write(InventoryCSVHeader); err != nil {
		return err
	}
	for _, host := range hosts {
		if err := csvWriter.Write([]string{
			strconv.Itoa(host.ID),
			host.Name,
			host.DNS,
			host.Iface,
			host.IP,
			host.Mac,
			host.Hw,
			host.Date,
			strconv.Itoa(host.Known),
			strconv.Itoa(host.Now),
			host.DeviceType,
			host.Owner,
			host.Location,
			host.Notes,
			strings.Join(host.Tags, "; "),
			strconv.FormatBool(host.Pinned),
			host.FirstSeen,
			strconv.FormatBool(host.FirstSeenEstimated),
			host.LastSeen,
		}); err != nil {
			return err
		}
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

func normalizeData(data Data) Data {
	if data.CurrentHosts == nil {
		data.CurrentHosts = []Host{}
	}
	if data.History == nil {
		data.History = []Host{}
	}
	if data.Events == nil {
		data.Events = []Event{}
	}
	if data.HostMetadata == nil {
		data.HostMetadata = []HostMetadata{}
	}
	if data.HostLifecycle == nil {
		data.HostLifecycle = []HostLifecycle{}
	}
	if data.DeviceProfiles == nil {
		data.DeviceProfiles = []DeviceProfile{}
	}
	if data.NetworkDeviceProfiles == nil {
		data.NetworkDeviceProfiles = []NetworkDeviceProfile{}
	}
	if data.SystemDeviceProfiles == nil {
		data.SystemDeviceProfiles = []SystemDeviceProfile{}
	}
	if data.HypervisorProfiles == nil {
		data.HypervisorProfiles = []HypervisorProfile{}
	}
	if data.InfrastructureWorkloads == nil {
		data.InfrastructureWorkloads = []InfrastructureWorkload{}
	}
	if data.InfrastructureWorkloadInterfaces == nil {
		data.InfrastructureWorkloadInterfaces = []InfrastructureWorkloadInterface{}
	}
	if data.InfrastructureWorkloadHostLinks == nil {
		data.InfrastructureWorkloadHostLinks = []InfrastructureWorkloadHostLink{}
	}

	return data
}
