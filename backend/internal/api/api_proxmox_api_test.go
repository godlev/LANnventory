package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
	"github.com/godlev/LANnventory/internal/proxmoxsync"
)

func TestProxmoxAPITestConnectionNeverMutatesInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-test", Mac: "AA:BB:CC:DD:EE:B1", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, false, false)
	defer server.Close()

	if err := gdb.UpsertProxmoxAPIConfig(models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5,
	}); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	if _, err := gdb.UpsertInfrastructureWorkload(host.Mac, models.InfrastructureWorkloadUpsert{
		NativeID: "555", WorkloadType: models.InfrastructureWorkloadTypeVM, Name: "manual",
		Status: models.InfrastructureWorkloadStatusRunning, Source: models.InfrastructureWorkloadSourceManual,
	}, "2026-09-21T08:00:00Z"); err != nil {
		t.Fatalf("seed manual workload: %v", err)
	}

	rec := postEmpty(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api/test")
	if rec.Code != http.StatusOK {
		t.Fatalf("test connection status = %d; body: %s", rec.Code, rec.Body.String())
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 1 || records[0].Workload.Source != models.InfrastructureWorkloadSourceManual {
		t.Fatalf("Test Connection mutated workloads: records=%+v err=%v", records, err)
	}
	if _, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI); err != nil || found {
		t.Fatalf("Test Connection mutated source state found=%v err=%v", found, err)
	}
	config, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found || config.Status != "connected" || config.LastAttemptAt == "" || config.LastSuccessfulSync != "" {
		t.Fatalf("connection status config found=%v err=%v config=%+v", found, err, config)
	}
}

func TestProxmoxAPISyncRequiresPreviewThenExplicitApply(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-sync", Mac: "AA:BB:CC:DD:EE:B2", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, false, false)
	defer server.Close()

	if err := gdb.UpsertProxmoxAPIConfig(models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: false, BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5,
	}); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}

	previewRec := postEmpty(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api/sync-preview")
	if previewRec.Code != http.StatusOK {
		t.Fatalf("sync preview status = %d; body: %s", previewRec.Code, previewRec.Body.String())
	}
	var response ProxmoxAPISyncPreviewResponse
	if err := json.Unmarshal(previewRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}
	if response.Snapshot.Source != proxmoxsnapshot.SourceProxmoxAPI || !response.Snapshot.Complete ||
		!response.Preview.ApplyAllowed || response.Preview.Summary.Added != 2 {
		t.Fatalf("sync preview response = %+v", response)
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 0 {
		t.Fatalf("preview mutated inventory records=%+v err=%v", records, err)
	}
	if _, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI); err != nil || found {
		t.Fatalf("preview persisted source state found=%v err=%v", found, err)
	}

	unconfirmed := ProxmoxImportApplyRequest{
		PreviewToken: response.Preview.PreviewToken,
		Confirmed: false,
		Snapshot: response.Snapshot,
	}
	rec := postJSON(t, router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api/sync-apply", unconfirmed)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed apply status = %d; body: %s", rec.Code, rec.Body.String())
	}

	confirmed := unconfirmed
	confirmed.Confirmed = true
	rec = postJSON(t, router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api/sync-apply", confirmed)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirmed apply status = %d; body: %s", rec.Code, rec.Body.String())
	}

	records, err = gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 2 {
		t.Fatalf("applied workloads records=%+v err=%v", records, err)
	}
	for _, record := range records {
		if record.Workload.Source != models.InfrastructureWorkloadSourceProxmoxAPI || record.Workload.NodeName == "" {
			t.Fatalf("API provenance workload = %+v", record.Workload)
		}
	}
	state, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil || !found || state.Source != models.InfrastructureWorkloadSourceProxmoxAPI {
		t.Fatalf("API source state found=%v err=%v state=%+v", found, err, state)
	}
	config, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found || !config.Enabled || config.LastSuccessfulSync == "" || config.Status != "connected" {
		t.Fatalf("API sync status found=%v err=%v config=%+v", found, err, config)
	}
}

func TestProxmoxAPIPartialSyncPreservesLastGoodInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-partial", Mac: "AA:BB:CC:DD:EE:B3", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, true, false)
	defer server.Close()

	if err := gdb.UpsertProxmoxAPIConfig(models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5,
	}); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	seedGoodAPIWorkload(t, host.Mac)

	previewRec := postEmpty(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api/sync-preview")
	if previewRec.Code != http.StatusOK {
		t.Fatalf("partial sync preview status = %d; body: %s", previewRec.Code, previewRec.Body.String())
	}
	var response ProxmoxAPISyncPreviewResponse
	if err := json.Unmarshal(previewRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if response.Snapshot.Complete || response.Preview.ApplyAllowed {
		t.Fatalf("partial snapshot unexpectedly applyable: %+v", response)
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 1 || records[0].Workload.NativeID != "900" || records[0].Workload.RetiredAt != "" {
		t.Fatalf("partial sync changed last good inventory records=%+v err=%v", records, err)
	}
	config, _, _ := gdb.SelectProxmoxAPIConfig(host.Mac)
	if config.Status != "degraded" || config.LastError == "" {
		t.Fatalf("partial sync status = %+v", config)
	}
}

func TestProxmoxAPIAuthFailurePreservesLastGoodInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-auth", Mac: "AA:BB:CC:DD:EE:B4", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, false, true)
	defer server.Close()

	if err := gdb.UpsertProxmoxAPIConfig(models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5,
	}); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	seedGoodAPIWorkload(t, host.Mac)

	rec := postEmpty(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api/sync-preview")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("auth failure status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("secret")) {
		t.Fatalf("auth failure response leaked secret: %s", rec.Body.String())
	}
	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 1 || records[0].Workload.NativeID != "900" || records[0].Workload.RetiredAt != "" {
		t.Fatalf("auth failure changed inventory records=%+v err=%v", records, err)
	}
	config, _, _ := gdb.SelectProxmoxAPIConfig(host.Mac)
	if config.Status != "error" || config.LastError != "authentication failed" {
		t.Fatalf("auth failure status = %+v", config)
	}
}

func seedGoodAPIWorkload(t *testing.T, mac string) {
	t.Helper()
	state := models.ProxmoxSourceState{
		HypervisorMac: mac,
		Source: models.InfrastructureWorkloadSourceProxmoxAPI,
		SchemaVersion: proxmoxsnapshot.SchemaVersion,
		CollectorVersion: proxmoxsnapshot.CollectorVersion,
		CollectedAt: "2026-09-21T08:00:00Z",
		Complete: true,
		NodeHostname: "pve-old",
		NodePVEVersion: "pve-manager/9.2.9",
		NodeStatus: "online",
		SnapshotDigest: "last-good-digest",
		ImportedAt: "2026-09-21T08:01:00Z",
	}
	err := gdb.ApplyProxmoxImport(mac, state, []models.InfrastructureWorkloadUpsert{{
		NativeID: "900", WorkloadType: models.InfrastructureWorkloadTypeVM, NodeName: "pve-old",
		Name: "last-good", Status: models.InfrastructureWorkloadStatusRunning,
		Source: models.InfrastructureWorkloadSourceProxmoxAPI,
	}})
	if err != nil {
		t.Fatalf("seed API inventory: %v", err)
	}
}

func newAPIIntegrationTestServer(t *testing.T, denyVMConfig, authFail bool) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if authFail {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null}`))
			return
		}
		switch r.URL.Path {
		case "/api2/json/version":
			_, _ = w.Write([]byte(`{"data":{"version":"9.2.10","release":"9.2"}}`))
		case "/api2/json/cluster/status":
			_, _ = w.Write([]byte(`{"data":[{"type":"cluster","name":"lab"},{"type":"node","name":"pve-1","online":1}]}`))
		case "/api2/json/cluster/resources":
			_, _ = w.Write([]byte(`{"data":[{"type":"qemu","vmid":119,"node":"pve-1","name":"media","status":"running"},{"type":"lxc","vmid":127,"node":"pve-1","name":"yubal","status":"stopped"}]}`))
		case "/api2/json/nodes/pve-1/qemu/119/config":
			if denyVMConfig {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"data":null}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"name":"media","net0":"virtio=BC:24:11:A2:40:12,bridge=vmbr0","ipconfig0":"ip=10.4.1.27/24"}}`))
		case "/api2/json/nodes/pve-1/lxc/127/config":
			_, _ = w.Write([]byte(`{"data":{"hostname":"yubal","net0":"name=eth0,bridge=vmbr0,hwaddr=AA:BB:CC:DD:EE:70,ip=10.4.1.70/24,type=veth"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func postEmpty(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func postJSON(t *testing.T, router http.Handler, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}



func TestProxmoxAutomaticSyncSafeSnapshotApplies(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-safe", Mac: "AA:BB:CC:DD:EE:E1", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, false, false)
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 3,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	if err := proxmoxSyncService.RunAutomatic(context.Background(), config); err != nil {
		t.Fatalf("RunAutomatic: %v", err)
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 2 {
		t.Fatalf("automatic apply records=%+v err=%v", records, err)
	}
	stored, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectProxmoxAPIConfig found=%v err=%v", found, err)
	}
	if stored.LastSyncStatus != "healthy" || stored.LastSyncAttemptAt == "" ||
		stored.LastSuccessfulCollectionAt == "" || stored.LastSyncTrigger != "automatic" ||
		stored.LastSyncError != "" {
		t.Fatalf("automatic runtime state = %+v", stored)
	}
	if _, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI); err != nil || !found {
		t.Fatalf("automatic apply source state found=%v err=%v", found, err)
	}
}

func TestProxmoxAutomaticSyncIPConflictRequiresReviewAndPreservesInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-review", Mac: "AA:BB:CC:DD:EE:E2", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	seedHost(t, models.Host{Name: "reused-ip", Mac: "AA:BB:CC:DD:EE:99", IP: "10.4.1.27", Now: 1, DeviceType: "server"})
	server := newAPIIntegrationTestServer(t, false, false)
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 4,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	if err := proxmoxSyncService.RunAutomatic(context.Background(), config); err != nil {
		t.Fatalf("review-required automatic run returned error: %v", err)
	}
	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 0 {
		t.Fatalf("review-required run changed inventory records=%+v err=%v", records, err)
	}
	stored, _, _ := gdb.SelectProxmoxAPIConfig(host.Mac)
	if stored.LastSyncStatus != "review-required" ||
		stored.LastSuccessfulCollectionAt == "" ||
		stored.LastSuccessfulSync != "" {
		t.Fatalf("review-required runtime state = %+v", stored)
	}
}

func TestProxmoxAutomaticPartialSnapshotNeverRetiresLastGood(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-partial", Mac: "AA:BB:CC:DD:EE:E3", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, true, false)
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 5,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	seedGoodAPIWorkload(t, host.Mac)

	if err := proxmoxSyncService.RunAutomatic(context.Background(), config); err == nil {
		t.Fatal("partial automatic sync unexpectedly reported success")
	}
	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 1 || records[0].Workload.NativeID != "900" || records[0].Workload.RetiredAt != "" {
		t.Fatalf("partial automatic sync changed last-good inventory records=%+v err=%v", records, err)
	}
	stored, _, _ := gdb.SelectProxmoxAPIConfig(host.Mac)
	if stored.LastSyncStatus != "error" || stored.LastSuccessfulCollectionAt != "" {
		t.Fatalf("partial automatic runtime state = %+v", stored)
	}
}


func TestProxmoxAutomaticSyncRejectsConfigChangedDuringCollection(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-config-race", Mac: "AA:BB:CC:DD:EE:EA", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	reached := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/version":
			_, _ = w.Write([]byte(`{"data":{"version":"9.2.10","release":"9.2"}}`))
		case "/api2/json/cluster/status":
			_, _ = w.Write([]byte(`{"data":[{"type":"cluster","name":"lab"},{"type":"node","name":"pve-1","online":1}]}`))
		case "/api2/json/cluster/resources":
			select {
			case reached <- struct{}{}:
			default:
			}
			<-release
			_, _ = w.Write([]byte(`{"data":[{"type":"qemu","vmid":119,"node":"pve-1","name":"media","status":"running"}]}`))
		case "/api2/json/nodes/pve-1/qemu/119/config":
			_, _ = w.Write([]byte(`{"data":{"name":"media","net0":"virtio=BC:24:11:A2:40:12,bridge=vmbr0","ipconfig0":"ip=10.4.1.27/24"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 3,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- proxmoxSyncService.RunAutomatic(context.Background(), config)
	}()
	select {
	case <-reached:
	case <-time.After(2 * time.Second):
		t.Fatal("automatic collection did not reach blocking API call")
	}

	config.TokenSecret = "replacement-secret"
	config.ConfigRevision = 4
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("replace config during collection: %v", err)
	}
	close(release)

	select {
	case err := <-done:
		if proxmoxsync.KindOf(err) != proxmoxsync.ErrorStaleConfig {
			t.Fatalf("automatic run error = %v kind=%q, want stale-config", err, proxmoxsync.KindOf(err))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("automatic run did not finish after releasing API call")
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil || len(records) != 0 {
		t.Fatalf("stale automatic run changed inventory records=%+v err=%v", records, err)
	}
	if _, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI); err != nil || found {
		t.Fatalf("stale automatic run persisted source state found=%v err=%v", found, err)
	}
	stored, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found || stored.ConfigRevision != 4 || stored.TokenSecret != "replacement-secret" {
		t.Fatalf("current config was not preserved found=%v err=%v config=%+v", found, err, stored)
	}
}
