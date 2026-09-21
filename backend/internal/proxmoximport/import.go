package proxmoximport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

const (
	maxCollectorVersionRunes = 64
	maxNodeTextRunes         = 255
	maxWorkloads             = 10000
	maxInterfacesPerWorkload = 64
	maxCollectionErrors      = 100
	maxCollectionErrorRunes  = 512
	maxNativeID              = 999999
	maxNameRunes             = 255
	maxInterfaceNameRunes    = 128
	maxNetworkTextRunes      = 255
)

type PreviewSummary struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Retired   int `json:"retired"`
	Conflicts int `json:"conflicts"`
}

type FieldConflict struct {
	Field    string `json:"field"`
	Managed  string `json:"managed"`
	Imported string `json:"imported"`
}

type NodeView struct {
	Hostname    string `json:"hostname"`
	PVEVersion  string `json:"pveVersion"`
	ClusterName string `json:"clusterName"`
	Status      string `json:"status"`
}

type NodeDiff struct {
	Action string    `json:"action"`
	Before *NodeView `json:"before"`
	After  NodeView  `json:"after"`
}

type InterfaceView struct {
	Name              string `json:"name"`
	Mac               string `json:"mac"`
	Bridge            string `json:"bridge"`
	VLANTag           string `json:"vlanTag"`
	ConfiguredAddress string `json:"configuredAddress"`
	ConfiguredNetwork string `json:"configuredNetwork"`
}

type WorkloadView struct {
	ID           uint            `json:"id,omitempty"`
	NativeID     string          `json:"nativeId"`
	WorkloadType string          `json:"workloadType"`
	NodeName     string          `json:"nodeName,omitempty"`
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	Source       string          `json:"source"`
	RetiredAt    string          `json:"retiredAt,omitempty"`
	Interfaces   []InterfaceView `json:"interfaces"`
}

type WorkloadDiff struct {
	Action  string        `json:"action"`
	Key     string        `json:"key"`
	Changes []string      `json:"changes"`
	Before  *WorkloadView `json:"before"`
	After   *WorkloadView `json:"after"`
}

type Preview struct {
	PreviewToken     string          `json:"previewToken"`
	SnapshotDigest   string          `json:"snapshotDigest"`
	Source           string          `json:"source"`
	CollectedAt      string          `json:"collectedAt"`
	Complete         bool            `json:"complete"`
	ApplyAllowed     bool            `json:"applyAllowed"`
	BlockedReasons   []string        `json:"blockedReasons"`
	Warnings         []string        `json:"warnings"`
	Summary          PreviewSummary  `json:"summary"`
	Node             NodeDiff        `json:"node"`
	ManagedConflicts []FieldConflict `json:"managedConflicts"`
	Workloads        []WorkloadDiff  `json:"workloads"`
}

type CurrentState struct {
	SourceState       *models.ProxmoxSourceState
	ManagedHypervisor *models.HypervisorProfile
	Workloads         []models.InfrastructureWorkloadRecord
}

// ValidateAndNormalize validates the allowlisted snapshot contract and returns
// deterministic, normalized data suitable for preview hashing and persistence.
func ValidateAndNormalize(input proxmoxsnapshot.Snapshot) (proxmoxsnapshot.Snapshot, error) {
	snapshot := input
	if snapshot.SchemaVersion != proxmoxsnapshot.SchemaVersion {
		return snapshot, fmt.Errorf("unsupported schemaVersion %d", snapshot.SchemaVersion)
	}
	snapshot.CollectorVersion = strings.TrimSpace(snapshot.CollectorVersion)
	if snapshot.CollectorVersion == "" || utf8.RuneCountInString(snapshot.CollectorVersion) > maxCollectorVersionRunes || hasUnsafeControl(snapshot.CollectorVersion) {
		return snapshot, errors.New("invalid collectorVersion")
	}
	snapshot.Source = strings.TrimSpace(snapshot.Source)
	switch snapshot.Source {
	case proxmoxsnapshot.SourceScriptImport, proxmoxsnapshot.SourceProxmoxAPI:
	default:
		return snapshot, errors.New("source must be script-import or proxmox-api")
	}

	collectedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(snapshot.CollectedAt))
	if err != nil {
		return snapshot, errors.New("collectedAt must be RFC3339")
	}
	snapshot.CollectedAt = collectedAt.UTC().Format(time.RFC3339)

	if len(snapshot.CollectionErrors) > maxCollectionErrors {
		return snapshot, errors.New("too many collectionErrors")
	}
	for i := range snapshot.CollectionErrors {
		snapshot.CollectionErrors[i] = strings.TrimSpace(snapshot.CollectionErrors[i])
		if snapshot.CollectionErrors[i] == "" ||
			utf8.RuneCountInString(snapshot.CollectionErrors[i]) > maxCollectionErrorRunes ||
			hasUnsafeControl(snapshot.CollectionErrors[i]) {
			return snapshot, fmt.Errorf("invalid collectionErrors[%d]", i)
		}
	}
	if snapshot.Complete && len(snapshot.CollectionErrors) > 0 {
		return snapshot, errors.New("complete snapshot cannot contain collectionErrors")
	}

	node, err := normalizeNode(snapshot.Node)
	if err != nil {
		return snapshot, err
	}
	snapshot.Node = node

	if len(snapshot.Workloads) > maxWorkloads {
		return snapshot, errors.New("too many workloads")
	}
	seenWorkloads := make(map[string]struct{}, len(snapshot.Workloads))
	for i := range snapshot.Workloads {
		workload, err := normalizeWorkload(snapshot.Workloads[i])
		if err != nil {
			return snapshot, fmt.Errorf("workloads[%d]: %w", i, err)
		}
		key := workloadKey(workload.WorkloadType, workload.NativeID)
		if _, exists := seenWorkloads[key]; exists {
			return snapshot, fmt.Errorf("duplicate workload %s", key)
		}
		seenWorkloads[key] = struct{}{}
		snapshot.Workloads[i] = workload
	}
	sort.Slice(snapshot.Workloads, func(i, j int) bool {
		left, right := snapshot.Workloads[i], snapshot.Workloads[j]
		if left.WorkloadType != right.WorkloadType {
			return left.WorkloadType < right.WorkloadType
		}
		li, _ := strconv.Atoi(left.NativeID)
		ri, _ := strconv.Atoi(right.NativeID)
		if li != ri {
			return li < ri
		}
		return left.NativeID < right.NativeID
	})

	if snapshot.Workloads == nil {
		snapshot.Workloads = []proxmoxsnapshot.WorkloadSnapshot{}
	}
	if snapshot.CollectionErrors == nil {
		snapshot.CollectionErrors = []string{}
	}
	return snapshot, nil
}

func BuildPreview(hypervisorMac string, snapshot proxmoxsnapshot.Snapshot, current CurrentState) (Preview, error) {
	canonicalMac, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return Preview{}, err
	}
	snapshot, err = ValidateAndNormalize(snapshot)
	if err != nil {
		return Preview{}, err
	}

	preview := Preview{
		Source:           snapshot.Source,
		CollectedAt:      snapshot.CollectedAt,
		Complete:         snapshot.Complete,
		ApplyAllowed:     true,
		BlockedReasons:   []string{},
		Warnings:         []string{},
		ManagedConflicts: []FieldConflict{},
		Workloads:        []WorkloadDiff{},
		Node: NodeDiff{
			Action: "add",
			After: NodeView{
				Hostname:    snapshot.Node.Hostname,
				PVEVersion:  snapshot.Node.PVEVersion,
				ClusterName: snapshot.Node.ClusterName,
				Status:      snapshot.Node.Status,
			},
		},
	}

	if !snapshot.Complete {
		preview.ApplyAllowed = false
		preview.BlockedReasons = append(preview.BlockedReasons, "snapshot is incomplete; collect a complete snapshot before applying")
	}
	if len(snapshot.CollectionErrors) > 0 {
		preview.Warnings = append(preview.Warnings, snapshot.CollectionErrors...)
	}

	if current.SourceState != nil {
		before := nodeViewFromSourceState(*current.SourceState)
		preview.Node.Before = &before
		if nodeViewsEqual(before, preview.Node.After) {
			preview.Node.Action = "unchanged"
		} else {
			preview.Node.Action = "update"
		}
	}

	if current.ManagedHypervisor != nil {
		preview.ManagedConflicts = managedNodeConflicts(*current.ManagedHypervisor, preview.Node.After)
		if len(preview.ManagedConflicts) > 0 {
			preview.Warnings = append(preview.Warnings, "imported node data differs from managed hypervisor fields; managed values will not be overwritten")
		}
	}

	currentByKey := make(map[string]models.InfrastructureWorkloadRecord, len(current.Workloads))
	for _, record := range current.Workloads {
		key := workloadKey(record.Workload.WorkloadType, record.Workload.NativeID)
		currentByKey[key] = record
	}

	incoming := make(map[string]struct{}, len(snapshot.Workloads))
	for _, workload := range snapshot.Workloads {
		key := workloadKey(workload.WorkloadType, workload.NativeID)
		incoming[key] = struct{}{}
		after := workloadViewFromSnapshot(workload, snapshot.Source)

		record, exists := currentByKey[key]
		if !exists {
			preview.Summary.Added++
			preview.Workloads = append(preview.Workloads, WorkloadDiff{
				Action:  "add",
				Key:     key,
				Changes: []string{"workload"},
				After:   &after,
			})
			continue
		}

		before := workloadViewFromRecord(record)
		if !isManagedProxmoxSource(record.Workload.Source) {
			preview.Summary.Conflicts++
			preview.ApplyAllowed = false
			preview.Workloads = append(preview.Workloads, WorkloadDiff{
				Action:  "conflict",
				Key:     key,
				Changes: []string{"source"},
				Before:  &before,
				After:   &after,
			})
			continue
		}

		changes := workloadChanges(before, after)
		if record.Workload.RetiredAt != "" {
			changes = append(changes, "retired")
		}
		if len(changes) == 0 {
			preview.Summary.Unchanged++
			preview.Workloads = append(preview.Workloads, WorkloadDiff{
				Action:  "unchanged",
				Key:     key,
				Changes: []string{},
				Before:  &before,
				After:   &after,
			})
			continue
		}

		preview.Summary.Updated++
		preview.Workloads = append(preview.Workloads, WorkloadDiff{
			Action:  "update",
			Key:     key,
			Changes: changes,
			Before:  &before,
			After:   &after,
		})
	}

	for key, record := range currentByKey {
		if !isManagedProxmoxSource(record.Workload.Source) ||
			record.Workload.RetiredAt != "" {
			continue
		}
		if _, exists := incoming[key]; exists {
			continue
		}
		before := workloadViewFromRecord(record)
		preview.Summary.Retired++
		preview.Workloads = append(preview.Workloads, WorkloadDiff{
			Action:  "retire",
			Key:     key,
			Changes: []string{"retired"},
			Before:  &before,
		})
	}

	if preview.Summary.Conflicts > 0 {
		preview.BlockedReasons = append(preview.BlockedReasons, "manual or other-source workload conflicts must be resolved before applying")
	}

	sort.Slice(preview.Workloads, func(i, j int) bool {
		if preview.Workloads[i].Key != preview.Workloads[j].Key {
			return preview.Workloads[i].Key < preview.Workloads[j].Key
		}
		return preview.Workloads[i].Action < preview.Workloads[j].Action
	})
	sort.Slice(preview.ManagedConflicts, func(i, j int) bool {
		return preview.ManagedConflicts[i].Field < preview.ManagedConflicts[j].Field
	})

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return Preview{}, err
	}
	digest := sha256.Sum256(snapshotJSON)
	preview.SnapshotDigest = hex.EncodeToString(digest[:])

	tokenPayload := struct {
		HypervisorMac string
		Snapshot      proxmoxsnapshot.Snapshot
		Current       CurrentState
	}{
		HypervisorMac: canonicalMac,
		Snapshot:      snapshot,
		Current:       normalizeCurrentStateForToken(current),
	}
	tokenJSON, err := json.Marshal(tokenPayload)
	if err != nil {
		return Preview{}, err
	}
	token := sha256.Sum256(tokenJSON)
	preview.PreviewToken = hex.EncodeToString(token[:])

	return preview, nil
}

func SnapshotWorkloadInputs(snapshot proxmoxsnapshot.Snapshot) ([]models.InfrastructureWorkloadUpsert, error) {
	snapshot, err := ValidateAndNormalize(snapshot)
	if err != nil {
		return nil, err
	}
	source, err := workloadSourceForSnapshotSource(snapshot.Source)
	if err != nil {
		return nil, err
	}
	inputs := make([]models.InfrastructureWorkloadUpsert, 0, len(snapshot.Workloads))
	for _, workload := range snapshot.Workloads {
		interfaces := make([]models.InfrastructureWorkloadInterface, 0, len(workload.Interfaces))
		for _, iface := range workload.Interfaces {
			interfaces = append(interfaces, models.InfrastructureWorkloadInterface{
				Name:              iface.Name,
				Mac:               iface.Mac,
				Bridge:            iface.Bridge,
				VLANTag:           iface.VLANTag,
				ConfiguredAddress: iface.ConfiguredAddress,
				ConfiguredNetwork: iface.ConfiguredNetwork,
			})
		}
		inputs = append(inputs, models.InfrastructureWorkloadUpsert{
			NativeID:     workload.NativeID,
			WorkloadType: workload.WorkloadType,
			NodeName:     workload.NodeName,
			Name:         workload.Name,
			Status:       workload.Status,
			Source:       source,
			Interfaces:   interfaces,
		})
	}
	return inputs, nil
}

func SourceStateFromSnapshot(hypervisorMac string, snapshot proxmoxsnapshot.Snapshot, digest, importedAt string) (models.ProxmoxSourceState, error) {
	canonicalMac, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return models.ProxmoxSourceState{}, err
	}
	snapshot, err = ValidateAndNormalize(snapshot)
	if err != nil {
		return models.ProxmoxSourceState{}, err
	}
	source, err := workloadSourceForSnapshotSource(snapshot.Source)
	if err != nil {
		return models.ProxmoxSourceState{}, err
	}
	return models.ProxmoxSourceState{
		HypervisorMac:    canonicalMac,
		Source:           source,
		SchemaVersion:    snapshot.SchemaVersion,
		CollectorVersion: snapshot.CollectorVersion,
		CollectedAt:      snapshot.CollectedAt,
		Complete:         snapshot.Complete,
		NodeHostname:     snapshot.Node.Hostname,
		NodePVEVersion:   snapshot.Node.PVEVersion,
		NodeClusterName:  snapshot.Node.ClusterName,
		NodeStatus:       snapshot.Node.Status,
		SnapshotDigest:   strings.TrimSpace(digest),
		ImportedAt:       strings.TrimSpace(importedAt),
	}, nil
}

func normalizeNode(node proxmoxsnapshot.NodeSnapshot) (proxmoxsnapshot.NodeSnapshot, error) {
	var err error
	node.Hostname, err = safeText("node.hostname", node.Hostname, maxNodeTextRunes, true)
	if err != nil {
		return node, err
	}
	node.PVEVersion, err = safeText("node.pveVersion", node.PVEVersion, maxNodeTextRunes, true)
	if err != nil {
		return node, err
	}
	node.ClusterName, err = safeText("node.clusterName", node.ClusterName, maxNodeTextRunes, false)
	if err != nil {
		return node, err
	}
	node.Status = strings.ToLower(strings.TrimSpace(node.Status))
	switch node.Status {
	case "online", "offline", "unknown":
	default:
		return node, errors.New("node.status must be online, offline, or unknown")
	}
	return node, nil
}

func normalizeWorkload(workload proxmoxsnapshot.WorkloadSnapshot) (proxmoxsnapshot.WorkloadSnapshot, error) {
	workload.NativeID = strings.TrimSpace(workload.NativeID)
	id, err := strconv.Atoi(workload.NativeID)
	if err != nil || id < 1 || id > maxNativeID || strconv.Itoa(id) != workload.NativeID {
		return workload, errors.New("nativeId must be an integer from 1 to 999999")
	}
	workload.WorkloadType = strings.ToLower(strings.TrimSpace(workload.WorkloadType))
	if workload.WorkloadType != models.InfrastructureWorkloadTypeVM &&
		workload.WorkloadType != models.InfrastructureWorkloadTypeContainer {
		return workload, errors.New("workloadType must be vm or container")
	}
	workload.Name, err = safeText("name", workload.Name, maxNameRunes, false)
	if err != nil {
		return workload, err
	}
	workload.Status = strings.ToLower(strings.TrimSpace(workload.Status))
	switch workload.Status {
	case models.InfrastructureWorkloadStatusRunning, models.InfrastructureWorkloadStatusStopped, models.InfrastructureWorkloadStatusUnknown:
	default:
		return workload, errors.New("status must be running, stopped, or unknown")
	}
	if len(workload.Interfaces) > maxInterfacesPerWorkload {
		return workload, errors.New("too many interfaces")
	}

	seenNames := make(map[string]struct{}, len(workload.Interfaces))
	seenMACs := make(map[string]struct{}, len(workload.Interfaces))
	for i := range workload.Interfaces {
		iface, err := normalizeInterface(workload.Interfaces[i])
		if err != nil {
			return workload, fmt.Errorf("interfaces[%d]: %w", i, err)
		}
		if _, exists := seenNames[iface.Name]; exists {
			return workload, fmt.Errorf("duplicate interface name %q", iface.Name)
		}
		seenNames[iface.Name] = struct{}{}
		if iface.Mac != "" {
			if _, exists := seenMACs[iface.Mac]; exists {
				return workload, fmt.Errorf("duplicate interface MAC %q", iface.Mac)
			}
			seenMACs[iface.Mac] = struct{}{}
		}
		workload.Interfaces[i] = iface
	}
	sort.Slice(workload.Interfaces, func(i, j int) bool {
		return workload.Interfaces[i].Name < workload.Interfaces[j].Name
	})
	if workload.Interfaces == nil {
		workload.Interfaces = []proxmoxsnapshot.InterfaceSnapshot{}
	}
	return workload, nil
}

func normalizeInterface(iface proxmoxsnapshot.InterfaceSnapshot) (proxmoxsnapshot.InterfaceSnapshot, error) {
	var err error
	iface.Name, err = safeText("name", iface.Name, maxInterfaceNameRunes, true)
	if err != nil {
		return iface, err
	}
	if strings.TrimSpace(iface.Mac) != "" {
		iface.Mac, err = identity.NormalizeMAC(iface.Mac)
		if err != nil {
			return iface, errors.New("invalid MAC")
		}
	}
	iface.Bridge, err = safeText("bridge", iface.Bridge, maxNetworkTextRunes, false)
	if err != nil {
		return iface, err
	}
	iface.VLANTag = strings.TrimSpace(iface.VLANTag)
	if iface.VLANTag != "" {
		tag, err := strconv.Atoi(iface.VLANTag)
		if err != nil || tag < 0 || tag > 4094 || strconv.Itoa(tag) != iface.VLANTag {
			return iface, errors.New("vlanTag must be an integer from 0 to 4094")
		}
	}
	iface.ConfiguredAddress, err = normalizeAddress(iface.ConfiguredAddress)
	if err != nil {
		return iface, err
	}
	iface.ConfiguredNetwork, err = normalizeNetwork(iface.ConfiguredNetwork)
	if err != nil {
		return iface, err
	}
	if iface.ConfiguredAddress != "" {
		derived := networkFromAddress(iface.ConfiguredAddress)
		if iface.ConfiguredNetwork == "" {
			iface.ConfiguredNetwork = derived
		} else if derived != "" && iface.ConfiguredNetwork != derived {
			return iface, errors.New("configuredNetwork does not match configuredAddress")
		}
	}
	return iface, nil
}

func normalizeAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if ip := net.ParseIP(value); ip != nil {
		return ip.String(), nil
	}
	ip, network, err := net.ParseCIDR(value)
	if err != nil {
		return "", errors.New("configuredAddress must be an IP address or CIDR")
	}
	ones, _ := network.Mask.Size()
	return fmt.Sprintf("%s/%d", ip.String(), ones), nil
}

func normalizeNetwork(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return "", errors.New("configuredNetwork must be CIDR")
	}
	return network.String(), nil
}

func networkFromAddress(value string) string {
	if !strings.Contains(value, "/") {
		return ""
	}
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return ""
	}
	return network.String()
}

func safeText(field, value string, maxRunes int, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if utf8.RuneCountInString(value) > maxRunes || hasUnsafeControl(value) {
		return "", fmt.Errorf("invalid %s", field)
	}
	return value, nil
}

func hasUnsafeControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func workloadKey(workloadType, nativeID string) string {
	return workloadType + ":" + nativeID
}

func isManagedProxmoxSource(source string) bool {
	switch strings.TrimSpace(source) {
	case models.InfrastructureWorkloadSourceScriptImport, models.InfrastructureWorkloadSourceProxmoxAPI:
		return true
	default:
		return false
	}
}

func workloadSourceForSnapshotSource(source string) (string, error) {
	switch strings.TrimSpace(source) {
	case proxmoxsnapshot.SourceScriptImport:
		return models.InfrastructureWorkloadSourceScriptImport, nil
	case proxmoxsnapshot.SourceProxmoxAPI:
		return models.InfrastructureWorkloadSourceProxmoxAPI, nil
	default:
		return "", errors.New("unsupported Proxmox snapshot source")
	}
}

func workloadViewFromSnapshot(workload proxmoxsnapshot.WorkloadSnapshot, source string) WorkloadView {
	view := WorkloadView{
		NativeID:     workload.NativeID,
		WorkloadType: workload.WorkloadType,
		NodeName:     workload.NodeName,
		Name:         workload.Name,
		Status:       workload.Status,
		Source:       source,
		Interfaces:   make([]InterfaceView, 0, len(workload.Interfaces)),
	}
	for _, iface := range workload.Interfaces {
		view.Interfaces = append(view.Interfaces, InterfaceView{
			Name:              iface.Name,
			Mac:               iface.Mac,
			Bridge:            iface.Bridge,
			VLANTag:           iface.VLANTag,
			ConfiguredAddress: iface.ConfiguredAddress,
			ConfiguredNetwork: iface.ConfiguredNetwork,
		})
	}
	return view
}

func workloadViewFromRecord(record models.InfrastructureWorkloadRecord) WorkloadView {
	view := WorkloadView{
		ID:           record.Workload.ID,
		NativeID:     record.Workload.NativeID,
		WorkloadType: record.Workload.WorkloadType,
		NodeName:     record.Workload.NodeName,
		Name:         record.Workload.Name,
		Status:       record.Workload.Status,
		Source:       record.Workload.Source,
		RetiredAt:    record.Workload.RetiredAt,
		Interfaces:   make([]InterfaceView, 0, len(record.Interfaces)),
	}
	for _, iface := range record.Interfaces {
		view.Interfaces = append(view.Interfaces, InterfaceView{
			Name:              iface.Name,
			Mac:               iface.Mac,
			Bridge:            iface.Bridge,
			VLANTag:           iface.VLANTag,
			ConfiguredAddress: iface.ConfiguredAddress,
			ConfiguredNetwork: iface.ConfiguredNetwork,
		})
	}
	sort.Slice(view.Interfaces, func(i, j int) bool {
		return view.Interfaces[i].Name < view.Interfaces[j].Name
	})
	return view
}

func workloadChanges(before, after WorkloadView) []string {
	changes := []string{}
	if before.NodeName != after.NodeName {
		changes = append(changes, "nodeName")
	}
	if before.Name != after.Name {
		changes = append(changes, "name")
	}
	if before.Status != after.Status {
		changes = append(changes, "status")
	}
	if before.Source != after.Source {
		changes = append(changes, "source")
	}
	if !interfacesEqual(before.Interfaces, after.Interfaces) {
		changes = append(changes, "interfaces")
	}
	return changes
}

func interfacesEqual(left, right []InterfaceView) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func nodeViewFromSourceState(state models.ProxmoxSourceState) NodeView {
	return NodeView{
		Hostname:    state.NodeHostname,
		PVEVersion:  state.NodePVEVersion,
		ClusterName: state.NodeClusterName,
		Status:      state.NodeStatus,
	}
}

func nodeViewsEqual(left, right NodeView) bool {
	return left == right
}

func managedNodeConflicts(managed models.HypervisorProfile, imported NodeView) []FieldConflict {
	conflicts := []FieldConflict{}
	for _, item := range []struct {
		field    string
		managed  string
		imported string
	}{
		{"version", managed.Version, imported.PVEVersion},
		{"nodeName", managed.NodeName, imported.Hostname},
		{"clusterName", managed.ClusterName, imported.ClusterName},
	} {
		if strings.TrimSpace(item.managed) != "" &&
			strings.TrimSpace(item.imported) != "" &&
			strings.TrimSpace(item.managed) != strings.TrimSpace(item.imported) {
			conflicts = append(conflicts, FieldConflict{
				Field:    item.field,
				Managed:  item.managed,
				Imported: item.imported,
			})
		}
	}
	return conflicts
}

func normalizeCurrentStateForToken(current CurrentState) CurrentState {
	result := CurrentState{
		SourceState:       current.SourceState,
		ManagedHypervisor: current.ManagedHypervisor,
		Workloads:         append([]models.InfrastructureWorkloadRecord(nil), current.Workloads...),
	}
	sort.Slice(result.Workloads, func(i, j int) bool {
		left, right := result.Workloads[i].Workload, result.Workloads[j].Workload
		if left.WorkloadType != right.WorkloadType {
			return left.WorkloadType < right.WorkloadType
		}
		if left.NativeID != right.NativeID {
			return left.NativeID < right.NativeID
		}
		return left.ID < right.ID
	})
	for i := range result.Workloads {
		sort.Slice(result.Workloads[i].Interfaces, func(a, b int) bool {
			left, right := result.Workloads[i].Interfaces[a], result.Workloads[i].Interfaces[b]
			if left.Name != right.Name {
				return left.Name < right.Name
			}
			return left.ID < right.ID
		})
	}
	return result
}
