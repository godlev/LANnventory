package gdb

import (
	"errors"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestProxmoxAPIRevisionGuardIsAtomicWithApply(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:D8"
	config := models.ProxmoxAPIConfig{
		HypervisorMac: mac, Enabled: true, AutomaticSync: true,
		BaseURL: "https://pve.example:8006", TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: true, TimeoutSeconds: 10, SyncIntervalMinutes: 60, ConfigRevision: 9,
	}
	if err := UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	state := models.ProxmoxSourceState{
		Source: models.InfrastructureWorkloadSourceProxmoxAPI,
		SchemaVersion: 1,
		CollectorVersion: "1.0.0",
		CollectedAt: "2026-09-24T12:00:00Z",
		Complete: true,
		NodeHostname: "pve-guard",
		NodePVEVersion: "9.2.10",
		NodeStatus: "online",
		SnapshotDigest: "guard-digest",
		ImportedAt: "2026-09-24T12:00:01Z",
	}

	err := ApplyProxmoxAPIImportIfRevision(mac, state, nil, 8, true)
	if !errors.Is(err, ErrProxmoxAPIConfigChanged) {
		t.Fatalf("stale guarded apply error = %v, want ErrProxmoxAPIConfigChanged", err)
	}
	if _, found, err := SelectProxmoxSourceState(mac, models.InfrastructureWorkloadSourceProxmoxAPI); err != nil || found {
		t.Fatalf("stale guarded apply persisted source state found=%v err=%v", found, err)
	}

	if err := ApplyProxmoxAPIImportIfRevision(mac, state, nil, 9, true); err != nil {
		t.Fatalf("current guarded apply: %v", err)
	}
	if _, found, err := SelectProxmoxSourceState(mac, models.InfrastructureWorkloadSourceProxmoxAPI); err != nil || !found {
		t.Fatalf("current guarded apply source state found=%v err=%v", found, err)
	}

	config.AutomaticSync = false
	config.ConfigRevision = 10
	if err := UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("disable automatic sync: %v", err)
	}
	state.CollectedAt = "2026-09-24T12:05:00Z"
	state.ImportedAt = "2026-09-24T12:05:01Z"
	state.SnapshotDigest = "guard-digest-2"
	err = ApplyProxmoxAPIImportIfRevision(mac, state, nil, 10, true)
	if !errors.Is(err, ErrProxmoxAPIConfigChanged) {
		t.Fatalf("automatic-disabled guarded apply error = %v", err)
	}
}

func TestDeletingProxmoxHostRemovesAutomaticScheduleState(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		Name: "pve-delete", Mac: "AA:BB:CC:DD:EE:D9", IP: "10.4.1.99",
		Date: "2026-09-24 12:00:00", Known: 1, Now: 1, DeviceType: "server",
	}
	if err := UpdateWithError("now", host); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	rows := SelectByMAC("now", host.Mac)
	if len(rows) != 1 {
		t.Fatalf("seeded host rows = %+v", rows)
	}
	host = rows[0]

	config := models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac, Enabled: true, AutomaticSync: true,
		BaseURL: "https://pve.example:8006", TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: true, TimeoutSeconds: 10, SyncIntervalMinutes: 60, ConfigRevision: 2,
		NextSyncAt: "2026-09-24T13:00:00Z",
	}
	if err := UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}
	state := models.ProxmoxSourceState{
		Source: models.InfrastructureWorkloadSourceProxmoxAPI,
		SchemaVersion: 1,
		CollectorVersion: "1.0.0",
		CollectedAt: "2026-09-24T12:00:00Z",
		Complete: true,
		NodeHostname: "pve-delete",
		NodePVEVersion: "9.2.10",
		NodeStatus: "online",
		SnapshotDigest: "delete-digest",
		ImportedAt: "2026-09-24T12:00:01Z",
	}
	if err := ApplyProxmoxImport(host.Mac, state, nil); err != nil {
		t.Fatalf("seed source state: %v", err)
	}

	if err := DeleteCurrentHostWithMetadata(host); err != nil {
		t.Fatalf("DeleteCurrentHostWithMetadata: %v", err)
	}
	if _, found, err := SelectProxmoxAPIConfig(host.Mac); err != nil || found {
		t.Fatalf("deleted host API config found=%v err=%v", found, err)
	}
	if _, found, err := SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI); err != nil || found {
		t.Fatalf("deleted host source state found=%v err=%v", found, err)
	}
	configs, err := SelectAutomaticProxmoxAPIConfigs()
	if err != nil {
		t.Fatalf("SelectAutomaticProxmoxAPIConfigs: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("deleted host remains scheduled: %+v", configs)
	}
}
