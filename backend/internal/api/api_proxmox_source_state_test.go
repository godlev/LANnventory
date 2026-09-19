package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoximport"
)

func TestProxmoxSourceStateEndpointIsEmptyBeforeImportAndPopulatedAfterApply(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:F0:10", DeviceType: "server", Now: 1})
	enableTestHypervisor(t, router, host.ID)

	rec := getPath(router, "/api/host/"+itoa(host.ID)+"/proxmox/source-state")
	if rec.Code != http.StatusOK {
		t.Fatalf("empty source-state status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if string(rec.Body.Bytes()) != "null" {
		t.Fatalf("empty source-state body = %q, want null", rec.Body.String())
	}

	snapshot := apiValidProxmoxSnapshot()
	snapshot.Node.Hostname = "pve-imported"
	snapshot.Node.ClusterName = "home-cluster"

	previewRec := proxmoxPreviewRequest(t, router, host.ID, snapshot)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status = %d; body: %s", previewRec.Code, previewRec.Body.String())
	}
	var preview proxmoximport.Preview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}
	applyRec := proxmoxApplyRequest(t, router, host.ID, ProxmoxImportApplyRequest{
		PreviewToken: preview.PreviewToken,
		Confirmed:    true,
		Snapshot:     snapshot,
	})
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d; body: %s", applyRec.Code, applyRec.Body.String())
	}

	rec = getPath(router, "/api/host/"+itoa(host.ID)+"/proxmox/source-state")
	if rec.Code != http.StatusOK {
		t.Fatalf("source-state status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var state models.ProxmoxSourceState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("json.Unmarshal source state: %v", err)
	}
	if state.NodeHostname != "pve-imported" ||
		state.NodeClusterName != "home-cluster" ||
		state.Source != models.InfrastructureWorkloadSourceScriptImport ||
		state.ImportedAt == "" {
		t.Fatalf("source state = %+v", state)
	}

	profile, found, err := gdb.SelectHypervisorProfileByMAC(host.Mac)
	if err != nil || !found {
		t.Fatalf("managed profile found=%v err=%v", found, err)
	}
	if profile.NodeName != "" || profile.ClusterName != "" || profile.Version != "" {
		t.Fatalf("imported source state leaked into managed profile: %+v", profile)
	}
}

func TestProxmoxSourceStateEndpointRejectsNonProxmoxHypervisor(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "other", Mac: "AA:BB:CC:DD:F0:11", DeviceType: "server"})
	rec := patchProfilePath(router, host.ID, "/hypervisor", `{"platform":"other"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable other hypervisor status = %d; body: %s", rec.Code, rec.Body.String())
	}

	rec = getPath(router, "/api/host/"+itoa(host.ID)+"/proxmox/source-state")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-Proxmox source-state status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
