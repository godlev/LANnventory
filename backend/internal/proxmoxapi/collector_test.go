package proxmoxapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

func TestCollectSnapshotMapsAllowlistedStandaloneAndClusterFields(t *testing.T) {
	server := newTestProxmoxServer(t, false)
	defer server.Close()

	client, err := New(Config{
		BaseURL: server.URL,
		TokenID: "lannventory@pve!inventory",
		TokenSecret: "secret",
		VerifyTLS: false,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	snapshot, err := CollectSnapshot(context.Background(), client, time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CollectSnapshot: %v", err)
	}
	if !snapshot.Complete || snapshot.Source != proxmoxsnapshot.SourceProxmoxAPI || snapshot.CollectedAt != "2026-09-21T10:00:00Z" {
		t.Fatalf("snapshot envelope = %+v", snapshot)
	}
	if snapshot.Node.Hostname != "pve-1" || snapshot.Node.ClusterName != "lab" || snapshot.Node.PVEVersion != "pve-manager/9.2.10" {
		t.Fatalf("node = %+v", snapshot.Node)
	}
	if len(snapshot.Workloads) != 2 {
		t.Fatalf("workloads = %+v", snapshot.Workloads)
	}

	container := snapshot.Workloads[0]
	vm := snapshot.Workloads[1]
	if container.WorkloadType != "container" || container.NativeID != "127" || container.NodeName != "pve-2" {
		t.Fatalf("container = %+v", container)
	}
	if len(container.Interfaces) != 1 || container.Interfaces[0].Name != "eth0" ||
		container.Interfaces[0].Mac != "AA:BB:CC:DD:EE:70" ||
		container.Interfaces[0].ConfiguredAddress != "10.4.1.70/24" {
		t.Fatalf("container interfaces = %+v", container.Interfaces)
	}
	if vm.WorkloadType != "vm" || vm.NativeID != "119" || vm.NodeName != "pve-1" || vm.Name != "media" {
		t.Fatalf("vm = %+v", vm)
	}
	if len(vm.Interfaces) != 1 || vm.Interfaces[0].Mac != "BC:24:11:A2:40:12" ||
		vm.Interfaces[0].Bridge != "vmbr0" || vm.Interfaces[0].VLANTag != "20" ||
		vm.Interfaces[0].ConfiguredAddress != "10.4.1.27/24" {
		t.Fatalf("vm interfaces = %+v", vm.Interfaces)
	}
}

func TestNodeSnapshotPrefersClusterLocalNode(t *testing.T) {
	node := nodeSnapshotFromAPI("cluster-api.example.local", "pve-manager/9.2.10", []ClusterStatusEntry{
		{Type: "cluster", Name: "lab"},
		{Type: "node", Name: "pve-1", IP: "10.0.0.1", Online: 1},
		{Type: "node", Name: "pve-2", IP: "10.0.0.2", Online: 1, Local: 1},
	})
	if node.Hostname != "pve-2" || node.ClusterName != "lab" {
		t.Fatalf("cluster local node provenance = %+v", node)
	}
}

func TestCollectSnapshotMarksPartialConfigFailureIncomplete(t *testing.T) {
	server := newTestProxmoxServer(t, true)
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret", VerifyTLS: false, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapshot, err := CollectSnapshot(context.Background(), client, time.Now())
	if err != nil {
		t.Fatalf("partial collection returned fatal error: %v", err)
	}
	if snapshot.Complete || len(snapshot.CollectionErrors) == 0 {
		t.Fatalf("partial snapshot = %+v", snapshot)
	}
	if len(snapshot.Workloads) != 2 {
		t.Fatalf("partial collection lost workload identities: %+v", snapshot.Workloads)
	}
}

func TestConnectionVerifiesInventoryConfigReadPermission(t *testing.T) {
	server := newTestProxmoxServer(t, true)
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret", VerifyTLS: false, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := TestConnection(context.Background(), client)
	if err == nil || KindOf(err) != ErrorPermission {
		t.Fatalf("TestConnection result=%+v err=%v kind=%s", result, err, KindOf(err))
	}
	if !result.ConnectionOK || !result.NodeAccess || result.VMInventory {
		t.Fatalf("permission diagnostics = %+v", result)
	}
}

func newTestProxmoxServer(t *testing.T, denyVMConfig bool) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/version":
			_, _ = w.Write([]byte(`{"data":{"version":"9.2.10","release":"9.2","repoid":"test"}}`))
		case "/api2/json/cluster/status":
			_, _ = w.Write([]byte(`{"data":[{"type":"cluster","name":"lab"},{"type":"node","name":"pve-1","ip":"127.0.0.1","online":1},{"type":"node","name":"pve-2","ip":"10.0.0.2","online":1}]}`))
		case "/api2/json/cluster/resources":
			_, _ = w.Write([]byte(`{"data":[{"type":"qemu","vmid":119,"node":"pve-1","name":"media","status":"running"},{"type":"lxc","vmid":127,"node":"pve-2","name":"yubal","status":"stopped"},{"type":"storage","id":"local"}]}`))
		case "/api2/json/nodes/pve-1/qemu/119/config":
			if denyVMConfig {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"data":null}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"name":"media","net0":"virtio=bc:24:11:a2:40:12,bridge=vmbr0,firewall=1,tag=20","ipconfig0":"ip=10.4.1.27/24,gw=10.4.1.1","scsi0":"local-lvm:vm-119-disk-0"}}`))
		case "/api2/json/nodes/pve-2/lxc/127/config":
			_, _ = w.Write([]byte(`{"data":{"hostname":"yubal","net0":"name=eth0,bridge=vmbr0,gw=10.4.1.1,hwaddr=aa:bb:cc:dd:ee:70,ip=10.4.1.70/24,type=veth,tag=30","mp0":"secret-storage"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}
