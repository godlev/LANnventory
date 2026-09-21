package proxmoxapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

type ConnectionTestResult struct {
	ConnectionOK bool   `json:"connectionOk"`
	PVEVersion   string `json:"pveVersion"`
	NodeAccess   bool   `json:"nodeAccess"`
	VMInventory  bool   `json:"vmInventory"`
	LXCInventory bool   `json:"lxcInventory"`
	NodeCount    int    `json:"nodeCount"`
	VMCount      int    `json:"vmCount"`
	LXCCount     int    `json:"lxcCount"`
}

func TestConnection(ctx context.Context, client *Client) (ConnectionTestResult, error) {
	result := ConnectionTestResult{}

	version, err := client.Version(ctx)
	if err != nil {
		return result, err
	}
	result.ConnectionOK = true
	result.PVEVersion = NormalizeVersion(version.Version)

	status, err := client.ClusterStatus(ctx)
	if err != nil {
		return result, err
	}
	result.NodeAccess = true
	for _, entry := range status {
		if strings.EqualFold(strings.TrimSpace(entry.Type), "node") {
			result.NodeCount++
		}
	}

	resources, err := client.ClusterResources(ctx)
	if err != nil {
		return result, err
	}
	var firstVM, firstLXC *Resource
	for i := range resources {
		resource := &resources[i]
		switch normalizedResourceType(resource.Type) {
		case "vm":
			result.VMCount++
			if firstVM == nil {
				firstVM = resource
			}
		case "container":
			result.LXCCount++
			if firstLXC == nil {
				firstLXC = resource
			}
		}
	}

	result.VMInventory = true
	if firstVM != nil {
		vmid, parseErr := resourceVMID(*firstVM)
		if parseErr != nil {
			return result, parseErr
		}
		if _, err := client.GuestConfig(ctx, firstVM.Node, "vm", vmid); err != nil {
			result.VMInventory = false
			return result, err
		}
	}

	result.LXCInventory = true
	if firstLXC != nil {
		vmid, parseErr := resourceVMID(*firstLXC)
		if parseErr != nil {
			return result, parseErr
		}
		if _, err := client.GuestConfig(ctx, firstLXC.Node, "container", vmid); err != nil {
			result.LXCInventory = false
			return result, err
		}
	}

	return result, nil
}

func CollectSnapshot(ctx context.Context, client *Client, collectedAt time.Time) (proxmoxsnapshot.Snapshot, error) {
	if client == nil {
		return proxmoxsnapshot.Snapshot{}, &APIError{Kind: ErrorInvalidConfig, Operation: "collect"}
	}

	version, err := client.Version(ctx)
	if err != nil {
		return proxmoxsnapshot.Snapshot{}, err
	}
	statusEntries, err := client.ClusterStatus(ctx)
	if err != nil {
		return proxmoxsnapshot.Snapshot{}, err
	}
	resources, err := client.ClusterResources(ctx)
	if err != nil {
		return proxmoxsnapshot.Snapshot{}, err
	}

	node := nodeSnapshotFromAPI(client.EndpointHost(), NormalizeVersion(version.Version), statusEntries)
	if node.Hostname == "" {
		return proxmoxsnapshot.Snapshot{}, &APIError{Kind: ErrorMalformed, Operation: "cluster-status"}
	}

	snapshot := proxmoxsnapshot.Snapshot{
		SchemaVersion:    proxmoxsnapshot.SchemaVersion,
		CollectorVersion: proxmoxsnapshot.CollectorVersion,
		Source:           proxmoxsnapshot.SourceProxmoxAPI,
		CollectedAt:      collectedAt.UTC().Format(time.RFC3339),
		Complete:         true,
		CollectionErrors: []string{},
		Node:             node,
		Workloads:        []proxmoxsnapshot.WorkloadSnapshot{},
	}

	sort.Slice(resources, func(i, j int) bool {
		leftType, rightType := normalizedResourceType(resources[i].Type), normalizedResourceType(resources[j].Type)
		if leftType != rightType {
			return leftType < rightType
		}
		leftID, _ := resourceVMID(resources[i])
		rightID, _ := resourceVMID(resources[j])
		if leftID != rightID {
			return leftID < rightID
		}
		return resources[i].Node < resources[j].Node
	})

	for _, resource := range resources {
		workloadType := normalizedResourceType(resource.Type)
		if workloadType == "" {
			continue
		}
		vmid, err := resourceVMID(resource)
		if err != nil || strings.TrimSpace(resource.Node) == "" {
			snapshot.Complete = false
			snapshot.CollectionErrors = append(snapshot.CollectionErrors, "workload inventory contains invalid identity")
			continue
		}

		config, configErr := client.GuestConfig(ctx, resource.Node, workloadType, vmid)
		interfaces := []proxmoxsnapshot.InterfaceSnapshot{}
		name := strings.TrimSpace(resource.Name)
		if configErr != nil {
			snapshot.Complete = false
			snapshot.CollectionErrors = append(snapshot.CollectionErrors, fmt.Sprintf("%s %d: network metadata unavailable", workloadType, vmid))
		} else {
			switch workloadType {
			case "vm":
				interfaces = parseQEMUInterfaces(config)
				if name == "" {
					name = stringValue(config["name"])
				}
			case "container":
				interfaces = parseLXCInterfaces(config)
				if name == "" {
					name = stringValue(config["hostname"])
				}
			}
		}

		snapshot.Workloads = append(snapshot.Workloads, proxmoxsnapshot.WorkloadSnapshot{
			NativeID:     strconv.Itoa(vmid),
			WorkloadType: workloadType,
			NodeName:     strings.TrimSpace(resource.Node),
			Name:         name,
			Status:       normalizeWorkloadStatus(resource.Status),
			Interfaces:   interfaces,
		})
	}

	if !snapshot.Complete && len(snapshot.CollectionErrors) == 0 {
		snapshot.CollectionErrors = []string{"Proxmox API collection incomplete"}
	}
	return snapshot, nil
}

func nodeSnapshotFromAPI(endpointHost, pveVersion string, entries []ClusterStatusEntry) proxmoxsnapshot.NodeSnapshot {
	clusterName := ""
	type nodeInfo struct {
		name  string
		ip    string
		local bool
	}
	nodes := []nodeInfo{}
	for _, entry := range entries {
		switch strings.ToLower(strings.TrimSpace(entry.Type)) {
		case "cluster":
			if clusterName == "" {
				clusterName = strings.TrimSpace(entry.Name)
			}
		case "node":
			name := strings.TrimSpace(entry.Name)
			if name == "" {
				name = strings.TrimSpace(stringValue(entry.NodeID))
			}
			if name != "" {
				nodes = append(nodes, nodeInfo{name: name, ip: strings.TrimSpace(entry.IP), local: entry.Local == 1})
			}
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].name < nodes[j].name })

	host := strings.TrimSpace(endpointHost)
	if len(nodes) == 1 {
		host = nodes[0].name
	} else {
		for _, node := range nodes {
			if node.local {
				host = node.name
				break
			}
		}
		if host == strings.TrimSpace(endpointHost) {
			for _, node := range nodes {
				if strings.EqualFold(host, node.name) || (node.ip != "" && host == node.ip) {
					host = node.name
					break
				}
			}
		}
	}
	if host == "" && clusterName != "" {
		host = clusterName
	}

	return proxmoxsnapshot.NodeSnapshot{
		Hostname:    host,
		PVEVersion:  pveVersion,
		ClusterName: clusterName,
		Status:      "online",
	}
}

func normalizedResourceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "qemu", "vm":
		return "vm"
	case "lxc", "container":
		return "container"
	default:
		return ""
	}
}

func resourceVMID(resource Resource) (int, error) {
	raw := strings.TrimSpace(resource.VMID.String())
	if raw == "" {
		return 0, &APIError{Kind: ErrorMalformed, Operation: "cluster-resources"}
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 {
		return 0, &APIError{Kind: ErrorMalformed, Operation: "cluster-resources"}
	}
	return id, nil
}

func normalizeWorkloadStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "running":
		return "running"
	case "stopped":
		return "stopped"
	default:
		return "unknown"
	}
}

func parseQEMUInterfaces(config map[string]any) []proxmoxsnapshot.InterfaceSnapshot {
	indexes := networkIndexes(config, "net")
	result := make([]proxmoxsnapshot.InterfaceSnapshot, 0, len(indexes))
	for _, index := range indexes {
		key := "net" + strconv.Itoa(index)
		values := commaValues(stringValue(config[key]))
		ipValues := commaValues(stringValue(config["ipconfig"+strconv.Itoa(index)]))
		result = append(result, proxmoxsnapshot.InterfaceSnapshot{
			Name:              key,
			Mac:               findMAC(values),
			Bridge:            strings.TrimSpace(values["bridge"]),
			VLANTag:           strings.TrimSpace(values["tag"]),
			ConfiguredAddress: configuredAddress(ipValues["ip"]),
		})
	}
	return result
}

func parseLXCInterfaces(config map[string]any) []proxmoxsnapshot.InterfaceSnapshot {
	indexes := networkIndexes(config, "net")
	result := make([]proxmoxsnapshot.InterfaceSnapshot, 0, len(indexes))
	for _, index := range indexes {
		key := "net" + strconv.Itoa(index)
		values := commaValues(stringValue(config[key]))
		name := strings.TrimSpace(values["name"])
		if name == "" {
			name = key
		}
		result = append(result, proxmoxsnapshot.InterfaceSnapshot{
			Name:              name,
			Mac:               normalizedMAC(values["hwaddr"]),
			Bridge:            strings.TrimSpace(values["bridge"]),
			VLANTag:           strings.TrimSpace(values["tag"]),
			ConfiguredAddress: configuredAddress(values["ip"]),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func networkIndexes(config map[string]any, prefix string) []int {
	indexes := []int{}
	for key := range config {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		raw := strings.TrimPrefix(key, prefix)
		if raw == "" {
			continue
		}
		index, err := strconv.Atoi(raw)
		if err == nil && index >= 0 {
			indexes = append(indexes, index)
		}
	}
	sort.Ints(indexes)
	return indexes
}

func commaValues(value string) map[string]string {
	result := map[string]string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key, val, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return result
}

func findMAC(values map[string]string) string {
	if mac := normalizedMAC(values["hwaddr"]); mac != "" {
		return mac
	}
	for key, value := range values {
		switch strings.ToLower(key) {
		case "bridge", "tag", "firewall", "link_down", "queues", "rate", "trunks", "mtu":
			continue
		}
		if mac := normalizedMAC(value); mac != "" {
			return mac
		}
	}
	return ""
}

func normalizedMAC(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	mac, err := identity.NormalizeMAC(value)
	if err != nil {
		return ""
	}
	return mac
}

func configuredAddress(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "dhcp", "manual", "auto":
		return ""
	default:
		return value
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func IsPartialCollectionError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.Kind == ErrorMalformed || apiErr.Kind == ErrorPermission || apiErr.Kind == ErrorAPI)
}
