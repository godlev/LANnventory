package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestProxmoxAPISchedulePersistsAcrossDatabaseRestart(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	dbPath := filepath.Join(t.TempDir(), "proxmox-schedule-restart.db")
	conf.SetAppConfigForTest(models.Conf{UseDB: "sqlite", DBPath: dbPath})
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
		HypervisorMac: "AA:BB:CC:DD:EE:D5", Enabled: true, AutomaticSync: true,
		BaseURL: "https://pve.example:8006", TokenID: "u@pve!t", TokenSecret: "secret",
		VerifyTLS: true, TimeoutSeconds: 10, SyncIntervalMinutes: 360, ConfigRevision: 6,
		NextSyncAt: "2026-09-24T22:00:00Z",
		LastSyncStatus: "healthy",
	}
	if err := UpsertProxmoxAPIConfig(config); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}

	if err := Close(); err != nil {
		t.Fatalf("Close before restart: %v", err)
	}
	if err := StartErr(); err != nil {
		t.Fatalf("StartErr after restart: %v", err)
	}

	stored, found, err := SelectProxmoxAPIConfig(config.HypervisorMac)
	if err != nil || !found {
		t.Fatalf("SelectProxmoxAPIConfig found=%v err=%v", found, err)
	}
	if !stored.Enabled || !stored.AutomaticSync || stored.SyncIntervalMinutes != 360 ||
		stored.ConfigRevision != 6 || stored.NextSyncAt != config.NextSyncAt {
		t.Fatalf("schedule config changed across restart: %+v", stored)
	}
	configs, err := SelectAutomaticProxmoxAPIConfigs()
	if err != nil {
		t.Fatalf("SelectAutomaticProxmoxAPIConfigs: %v", err)
	}
	if len(configs) != 1 || configs[0].HypervisorMac != config.HypervisorMac {
		t.Fatalf("automatic ownership not restored after restart: %+v", configs)
	}
}
