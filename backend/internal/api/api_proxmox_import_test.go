package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

func TestProxmoxImportPreviewDoesNotMutateInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:90", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	snapshot := apiValidProxmoxSnapshot()
	rec := proxmoxPreviewRequest(t, router, host.ID, snapshot)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var preview proxmoximport.Preview
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}
	if !preview.ApplyAllowed || preview.Summary.Added != 1 || preview.PreviewToken == "" || preview.SnapshotDigest == "" {
		t.Fatalf("preview = %+v", preview)
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectInfrastructureWorkloadsByHypervisorMAC: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("preview mutated workloads: %+v", records)
	}
	if _, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceScriptImport); err != nil || found {
		t.Fatalf("preview persisted source state found=%v err=%v", found, err)
	}
}

func TestProxmoxImportApplyRequiresExplicitConfirmationAndMatchingPreview(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:91", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	snapshot := apiValidProxmoxSnapshot()
	previewRec := proxmoxPreviewRequest(t, router, host.ID, snapshot)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status = %d; body: %s", previewRec.Code, previewRec.Body.String())
	}
	var preview proxmoximport.Preview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}

	rec := proxmoxApplyRequest(t, router, host.ID, ProxmoxImportApplyRequest{
		PreviewToken: preview.PreviewToken,
		Confirmed:    false,
		Snapshot:     snapshot,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed apply status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = proxmoxApplyRequest(t, router, host.ID, ProxmoxImportApplyRequest{
		PreviewToken: strings.Repeat("0", 64),
		Confirmed:    true,
		Snapshot:     snapshot,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("wrong-token apply status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	rec = proxmoxApplyRequest(t, router, host.ID, ProxmoxImportApplyRequest{
		PreviewToken: preview.PreviewToken,
		Confirmed:    true,
		Snapshot:     snapshot,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirmed apply status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var applied ProxmoxImportApplyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &applied); err != nil {
		t.Fatalf("json.Unmarshal apply: %v", err)
	}
	if !applied.Applied || applied.ImportedAt == "" || applied.Summary.Added != 1 {
		t.Fatalf("apply response = %+v", applied)
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil {
		t.Fatalf("load workloads: %v", err)
	}
	if len(records) != 1 || records[0].Workload.Source != models.InfrastructureWorkloadSourceScriptImport ||
		records[0].Workload.NativeID != "119" {
		t.Fatalf("imported workloads = %+v", records)
	}
	state, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceScriptImport)
	if err != nil || !found {
		t.Fatalf("source state found=%v err=%v", found, err)
	}
	if state.NodeHostname != snapshot.Node.Hostname || state.SnapshotDigest != preview.SnapshotDigest {
		t.Fatalf("source state = %+v", state)
	}
}

func TestProxmoxImportApplyRejectsStalePreviewAfterInventoryChange(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:92", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	snapshot := apiValidProxmoxSnapshot()
	previewRec := proxmoxPreviewRequest(t, router, host.ID, snapshot)
	var preview proxmoximport.Preview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}

	rec := workloadRequest(router, http.MethodPost, host.ID, "", `{"nativeId":"555","workloadType":"vm","name":"manual-change","status":"running"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("manual workload change status = %d; body: %s", rec.Code, rec.Body.String())
	}

	rec = proxmoxApplyRequest(t, router, host.ID, ProxmoxImportApplyRequest{
		PreviewToken: preview.PreviewToken,
		Confirmed:    true,
		Snapshot:     snapshot,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale apply status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil {
		t.Fatalf("load workloads: %v", err)
	}
	if len(records) != 1 || records[0].Workload.Source != models.InfrastructureWorkloadSourceManual {
		t.Fatalf("stale apply changed inventory: %+v", records)
	}
}

func TestProxmoxImportPreviewBlocksManualCollisionAndIncompleteSnapshot(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:93", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	rec := workloadRequest(router, http.MethodPost, host.ID, "", `{"nativeId":"119","workloadType":"vm","name":"manual","status":"running"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("manual workload seed status = %d; body: %s", rec.Code, rec.Body.String())
	}

	snapshot := apiValidProxmoxSnapshot()
	previewRec := proxmoxPreviewRequest(t, router, host.ID, snapshot)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("collision preview status = %d; body: %s", previewRec.Code, previewRec.Body.String())
	}
	var preview proxmoximport.Preview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal collision preview: %v", err)
	}
	if preview.ApplyAllowed || preview.Summary.Conflicts != 1 {
		t.Fatalf("collision preview = %+v", preview)
	}

	snapshot.Workloads = []proxmoxsnapshot.WorkloadSnapshot{}
	snapshot.Complete = false
	snapshot.CollectionErrors = []string{"qemu guest list unavailable"}
	previewRec = proxmoxPreviewRequest(t, router, host.ID, snapshot)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("incomplete preview status = %d; body: %s", previewRec.Code, previewRec.Body.String())
	}
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal incomplete preview: %v", err)
	}
	if preview.ApplyAllowed || len(preview.BlockedReasons) == 0 {
		t.Fatalf("incomplete preview = %+v", preview)
	}
}

func TestProxmoxImportKeepsManagedHypervisorFieldsSeparate(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:94", DeviceType: "server"})

	rec := patchProfilePath(router, host.ID, "/hypervisor", `{"platform":"proxmox-ve","version":"manual-version","nodeName":"manual-node","clusterName":"manual-cluster"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable managed hypervisor status = %d; body: %s", rec.Code, rec.Body.String())
	}

	snapshot := apiValidProxmoxSnapshot()
	snapshot.Node.Hostname = "imported-node"
	snapshot.Node.PVEVersion = "pve-manager/9.2.10"
	snapshot.Node.ClusterName = "imported-cluster"

	previewRec := proxmoxPreviewRequest(t, router, host.ID, snapshot)
	var preview proxmoximport.Preview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}
	if len(preview.ManagedConflicts) != 3 || !preview.ApplyAllowed {
		t.Fatalf("managed conflict preview = %+v", preview)
	}

	applyRec := proxmoxApplyRequest(t, router, host.ID, ProxmoxImportApplyRequest{
		PreviewToken: preview.PreviewToken,
		Confirmed:    true,
		Snapshot:     snapshot,
	})
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d; body: %s", applyRec.Code, applyRec.Body.String())
	}

	managed, found, err := gdb.SelectHypervisorProfileByMAC(host.Mac)
	if err != nil || !found {
		t.Fatalf("managed hypervisor found=%v err=%v", found, err)
	}
	if managed.Version != "manual-version" || managed.NodeName != "manual-node" || managed.ClusterName != "manual-cluster" {
		t.Fatalf("managed hypervisor overwritten by import: %+v", managed)
	}
	state, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceScriptImport)
	if err != nil || !found {
		t.Fatalf("imported source state found=%v err=%v", found, err)
	}
	if state.NodeHostname != "imported-node" || state.NodeClusterName != "imported-cluster" {
		t.Fatalf("imported node state = %+v", state)
	}
}

func TestProxmoxImportRequiresProxmoxProfileAndStrictJSON(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "other-hypervisor", Mac: "AA:BB:CC:DD:EE:95", DeviceType: "server"})
	rec := patchProfilePath(router, host.ID, "/hypervisor", `{"platform":"other"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable other hypervisor status = %d; body: %s", rec.Code, rec.Body.String())
	}

	snapshot := apiValidProxmoxSnapshot()
	rec = proxmoxPreviewRequest(t, router, host.ID, snapshot)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-Proxmox preview status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	proxmox := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:96", DeviceType: "server"})
	enableTestHypervisor(t, router, proxmox.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/host/"+itoa(proxmox.ID)+"/proxmox/import/preview", bytes.NewBufferString(`{"schemaVersion":1,"collectorVersion":"1.0.0","source":"script-import","collectedAt":"2026-09-19T09:00:00Z","complete":true,"node":{"hostname":"pve","pveVersion":"pve-manager/9.2.10","status":"online","secret":"nope"},"workloads":[]}`))
	req.Header.Set("Content-Type", "application/json")
	strictRec := httptest.NewRecorder()
	router.ServeHTTP(strictRec, req)
	if strictRec.Code != http.StatusBadRequest {
		t.Fatalf("unknown-field preview status = %d, want %d; body: %s", strictRec.Code, http.StatusBadRequest, strictRec.Body.String())
	}
}

func apiValidProxmoxSnapshot() proxmoxsnapshot.Snapshot {
	return proxmoxsnapshot.Snapshot{
		SchemaVersion:    proxmoxsnapshot.SchemaVersion,
		CollectorVersion: proxmoxsnapshot.CollectorVersion,
		Source:           proxmoxsnapshot.SourceScriptImport,
		CollectedAt:      "2026-09-19T09:00:00Z",
		Complete:         true,
		Node: proxmoxsnapshot.NodeSnapshot{
			Hostname:   "pve",
			PVEVersion: "pve-manager/9.2.10",
			Status:     "online",
		},
		Workloads: []proxmoxsnapshot.WorkloadSnapshot{{
			NativeID:     "119",
			WorkloadType: "vm",
			Name:         "media",
			Status:       "running",
			Interfaces: []proxmoxsnapshot.InterfaceSnapshot{{
				Name:              "net0",
				Mac:               "AA:BB:CC:00:01:19",
				Bridge:            "vmbr0",
				ConfiguredAddress: "10.4.1.19/24",
				ConfiguredNetwork: "10.4.1.0/24",
			}},
		}},
	}
}

func proxmoxPreviewRequest(t *testing.T, router *gin.Engine, hostID int, snapshot proxmoxsnapshot.Snapshot) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal snapshot: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/host/"+itoa(hostID)+"/proxmox/import/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func proxmoxApplyRequest(t *testing.T, router *gin.Engine, hostID int, payload ProxmoxImportApplyRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal apply payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/host/"+itoa(hostID)+"/proxmox/import/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
