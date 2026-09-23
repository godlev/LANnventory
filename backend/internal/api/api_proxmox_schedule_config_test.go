package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestProxmoxAPIScheduledSyncConfigDefaultsOff(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-default", Mac: "AA:BB:CC:DD:EE:B1", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	rec := getPath(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api-config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got ProxmoxAPIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.AutomaticSync {
		t.Fatal("automatic sync must default to off")
	}
	if got.SyncIntervalMinutes != defaultProxmoxAPISyncIntervalMinutes {
		t.Fatalf("sync interval = %d, want %d", got.SyncIntervalMinutes, defaultProxmoxAPISyncIntervalMinutes)
	}
	if got.ConfigRevision != 0 || got.Syncing {
		t.Fatalf("unexpected default runtime contract: %+v", got)
	}
	if got.SyncStatus != "disabled" {
		t.Fatalf("sync status = %q, want disabled", got.SyncStatus)
	}
}

func TestProxmoxAPIScheduledSyncIntervalValidationAndRevision(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-config", Mac: "AA:BB:CC:DD:EE:B2", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	rec := patchProxmoxAPIConfig(t, router, host.ID, `{
		"baseUrl":"https://10.4.1.6:8006",
		"tokenId":"lannventory@pve!inventory",
		"tokenSecret":"scheduled-sync-secret",
		"syncIntervalMinutes":60
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("initial config status = %d; body: %s", rec.Code, rec.Body.String())
	}

	var initial ProxmoxAPIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("json.Unmarshal initial: %v", err)
	}
	if initial.ConfigRevision != 1 {
		t.Fatalf("initial config revision = %d, want 1", initial.ConfigRevision)
	}

	rec = patchProxmoxAPIConfig(t, router, host.ID, `{"syncIntervalMinutes":60}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("no-op config status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var noOp ProxmoxAPIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &noOp); err != nil {
		t.Fatalf("json.Unmarshal no-op: %v", err)
	}
	if noOp.ConfigRevision != initial.ConfigRevision {
		t.Fatalf("no-op revision = %d, want %d", noOp.ConfigRevision, initial.ConfigRevision)
	}

	rec = patchProxmoxAPIConfig(t, router, host.ID, `{"automaticSync":true,"syncIntervalMinutes":15}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("scheduled config status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var changed ProxmoxAPIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &changed); err != nil {
		t.Fatalf("json.Unmarshal changed: %v", err)
	}
	if !changed.AutomaticSync || changed.SyncIntervalMinutes != 15 {
		t.Fatalf("scheduled config = %+v", changed)
	}
	if changed.ConfigRevision != initial.ConfigRevision+1 {
		t.Fatalf("changed revision = %d, want %d", changed.ConfigRevision, initial.ConfigRevision+1)
	}

	for _, interval := range []int{30, 60, 360, 720, 1440} {
		rec = patchProxmoxAPIConfig(t, router, host.ID, `{"syncIntervalMinutes":`+itoaHostID(interval)+`}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("supported interval %d status = %d; body: %s", interval, rec.Code, rec.Body.String())
		}
	}

	rec = patchProxmoxAPIConfig(t, router, host.ID, `{"syncIntervalMinutes":10}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported interval status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	stored, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectProxmoxAPIConfig found=%v err=%v", found, err)
	}
	if stored.SyncIntervalMinutes != 1440 {
		t.Fatalf("invalid patch changed stored interval to %d", stored.SyncIntervalMinutes)
	}
}

func TestProxmoxAPIScheduledSyncResponseDerivesLegacyAppliedState(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-legacy", Mac: "AA:BB:CC:DD:EE:B3", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	if err := gdb.UpsertProxmoxAPIConfig(models.ProxmoxAPIConfig{
		HypervisorMac:      host.Mac,
		Enabled:            true,
		BaseURL:            "https://10.4.1.6:8006",
		TokenID:            "lannventory@pve!inventory",
		TokenSecret:        "legacy-secret",
		VerifyTLS:          true,
		TimeoutSeconds:     10,
		LastAttemptAt:      "2026-09-23T16:40:00Z",
		LastSuccessfulSync: "2026-09-23T16:41:00Z",
		Status:             "connected",
	}); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}

	if err := gdb.ApplyProxmoxImport(host.Mac, models.ProxmoxSourceState{
		HypervisorMac:    host.Mac,
		Source:           models.InfrastructureWorkloadSourceProxmoxAPI,
		SchemaVersion:    1,
		CollectorVersion: "1.0.0",
		CollectedAt:      "2026-09-23T16:40:30Z",
		Complete:         true,
		NodeHostname:     "pve",
		NodePVEVersion:   "9.2.10",
		NodeStatus:       "online",
		SnapshotDigest:   "legacy-digest",
		ImportedAt:       "2026-09-23T16:41:00Z",
	}, nil); err != nil {
		t.Fatalf("ApplyProxmoxImport: %v", err)
	}

	rec := getPath(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api-config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var got ProxmoxAPIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.LastSyncAttemptAt != "2026-09-23T16:40:00Z" {
		t.Fatalf("last sync attempt = %q", got.LastSyncAttemptAt)
	}
	if got.LastSuccessfulCollectionAt != "2026-09-23T16:40:30Z" {
		t.Fatalf("last successful collection = %q", got.LastSuccessfulCollectionAt)
	}
	if got.LastAppliedAt != "2026-09-23T16:41:00Z" {
		t.Fatalf("last applied = %q", got.LastAppliedAt)
	}
	if got.SyncStatus != "healthy" {
		t.Fatalf("sync status = %q, want healthy", got.SyncStatus)
	}
}
