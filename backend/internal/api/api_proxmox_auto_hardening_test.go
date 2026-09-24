package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoxapi"
)

func TestProxmoxAutomaticAuthFailurePreservesLastGoodInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-auth", Mac: "AA:BB:CC:DD:EE:EB", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, false, true)
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 11,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	seedGoodAPIWorkload(t, host.Mac)

	err := proxmoxSyncService.RunAutomatic(context.Background(), config)
	if err == nil || proxmoxapi.KindOf(err) != proxmoxapi.ErrorAuthentication {
		t.Fatalf("automatic auth error=%v kind=%q", err, proxmoxapi.KindOf(err))
	}
	assertLastGoodAPIWorkloadActive(t, host.Mac)

	stored, _, _ := gdb.SelectProxmoxAPIConfig(host.Mac)
	if stored.LastSyncStatus != "error" || stored.LastSuccessfulCollectionAt != "" ||
		stored.LastSyncAttemptAt == "" || stored.LastSyncTrigger != "automatic" {
		t.Fatalf("automatic auth runtime state = %+v", stored)
	}
}

func TestProxmoxAutomaticTLSFailurePreservesLastGoodInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-tls", Mac: "AA:BB:CC:DD:EE:EC", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, false, false)
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: true, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 12,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	seedGoodAPIWorkload(t, host.Mac)

	err := proxmoxSyncService.RunAutomatic(context.Background(), config)
	if err == nil || proxmoxapi.KindOf(err) != proxmoxapi.ErrorTLS {
		t.Fatalf("automatic TLS error=%v kind=%q", err, proxmoxapi.KindOf(err))
	}
	assertLastGoodAPIWorkloadActive(t, host.Mac)
}

func TestProxmoxAutomaticTimeoutPreservesLastGoodInventory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-timeout", Mac: "AA:BB:CC:DD:EE:ED", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 13,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	seedGoodAPIWorkload(t, host.Mac)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := proxmoxSyncService.RunAutomatic(ctx, config)
	if err == nil || proxmoxapi.KindOf(err) != proxmoxapi.ErrorTimeout {
		t.Fatalf("automatic timeout error=%v kind=%q", err, proxmoxapi.KindOf(err))
	}
	assertLastGoodAPIWorkloadActive(t, host.Mac)
}

func TestProxmoxAutomaticCompleteSnapshotRetiresMissingManagedWorkload(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-auto-retire", Mac: "AA:BB:CC:DD:EE:EE", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)
	server := newAPIIntegrationTestServer(t, false, false)
	defer server.Close()

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: server.URL, TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: false, TimeoutSeconds: 5, SyncIntervalMinutes: 15, ConfigRevision: 14,
	}
	if err := gdb.UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	seedGoodAPIWorkload(t, host.Mac)

	if err := proxmoxSyncService.RunAutomatic(context.Background(), config); err != nil {
		t.Fatalf("RunAutomatic: %v", err)
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectInfrastructureWorkloadsByHypervisorMAC: %v", err)
	}
	foundRetired := false
	activeNew := 0
	for _, record := range records {
		switch record.Workload.NativeID {
		case "900":
			foundRetired = record.Workload.RetiredAt != ""
		case "119", "127":
			if record.Workload.RetiredAt == "" {
				activeNew++
			}
		}
	}
	if !foundRetired || activeNew != 2 {
		t.Fatalf("complete automatic reconciliation records=%+v", records)
	}

	stored, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectProxmoxAPIConfig found=%v err=%v", found, err)
	}
	if stored.LastSyncAttemptAt == "" || stored.LastSuccessfulCollectionAt == "" ||
		stored.LastSuccessfulSync == "" || stored.LastSyncStatus != "healthy" {
		t.Fatalf("successful automatic timestamps/status = %+v", stored)
	}
	state, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil || !found || state.ImportedAt == "" {
		t.Fatalf("successful automatic applied state found=%v err=%v state=%+v", found, err, state)
	}
}

func assertLastGoodAPIWorkloadActive(t *testing.T, mac string) {
	t.Helper()
	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(mac)
	if err != nil || len(records) != 1 || records[0].Workload.NativeID != "900" || records[0].Workload.RetiredAt != "" {
		t.Fatalf("last-good inventory changed records=%+v err=%v", records, err)
	}
}
