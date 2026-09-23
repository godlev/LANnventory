package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestProxmoxAPIScheduleQueryAndRevisionGuard(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "proxmox-schedule.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})
	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	auto := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:D1", Enabled: true, AutomaticSync: true,
		BaseURL: "https://pve.example:8006", TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: true, TimeoutSeconds: 10, SyncIntervalMinutes: 60, ConfigRevision: 4,
	}
	manual := auto
	manual.HypervisorMac = "AA:BB:CC:DD:EE:D2"
	manual.AutomaticSync = false
	if err := UpsertProxmoxAPIConfig(auto); err != nil {
		t.Fatalf("Upsert auto: %v", err)
	}
	if err := UpsertProxmoxAPIConfig(manual); err != nil {
		t.Fatalf("Upsert manual: %v", err)
	}

	configs, err := SelectAutomaticProxmoxAPIConfigs()
	if err != nil {
		t.Fatalf("SelectAutomaticProxmoxAPIConfigs: %v", err)
	}
	if len(configs) != 1 || configs[0].HypervisorMac != auto.HypervisorMac {
		t.Fatalf("automatic configs = %+v", configs)
	}

	updated, err := UpdateProxmoxAPINextSyncIfRevision(auto.HypervisorMac, 3, "2026-09-23T19:00:00Z")
	if err != nil {
		t.Fatalf("stale next update: %v", err)
	}
	if updated {
		t.Fatal("stale config revision unexpectedly updated next sync")
	}

	updated, err = UpdateProxmoxAPINextSyncIfRevision(auto.HypervisorMac, 4, "2026-09-23T19:00:00Z")
	if err != nil || !updated {
		t.Fatalf("current next update updated=%v err=%v", updated, err)
	}
	stored, found, err := SelectProxmoxAPIConfig(auto.HypervisorMac)
	if err != nil || !found || stored.NextSyncAt != "2026-09-23T19:00:00Z" {
		t.Fatalf("stored schedule found=%v err=%v config=%+v", found, err, stored)
	}
}


func TestProxmoxAPISyncRuntimeRevisionGuardRejectsStaleResult(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "proxmox-runtime-guard.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})
	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:D3", Enabled: true, AutomaticSync: true,
		BaseURL: "https://pve.example:8006", TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: true, TimeoutSeconds: 10, SyncIntervalMinutes: 60, ConfigRevision: 9,
	}
	if err := UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("Upsert config: %v", err)
	}

	status := "healthy"
	attempt := "2026-09-23T18:00:00Z"
	updated, err := UpdateProxmoxAPISyncRuntimeIfRevision(config.HypervisorMac, 8, ProxmoxAPISyncRuntimeUpdate{
		LastSyncAttemptAt: &attempt,
		LastSyncStatus:    &status,
	})
	if err != nil {
		t.Fatalf("stale runtime update: %v", err)
	}
	if updated {
		t.Fatal("stale runtime update unexpectedly matched")
	}

	updated, err = UpdateProxmoxAPISyncRuntimeIfRevision(config.HypervisorMac, 9, ProxmoxAPISyncRuntimeUpdate{
		LastSyncAttemptAt: &attempt,
		LastSyncStatus:    &status,
	})
	if err != nil || !updated {
		t.Fatalf("current runtime update updated=%v err=%v", updated, err)
	}
	stored, found, err := SelectProxmoxAPIConfig(config.HypervisorMac)
	if err != nil || !found || stored.LastSyncAttemptAt != attempt || stored.LastSyncStatus != status {
		t.Fatalf("stored runtime found=%v err=%v config=%+v", found, err, stored)
	}
}
