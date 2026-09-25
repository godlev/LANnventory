package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestProxmoxAPIScheduledSyncMigrationIsAdditive(t *testing.T) {
	fixtureDB := openMigrationFixtureDB(t, ":memory:")
	defer closeFixtureDB(t, fixtureDB)

	if err := fixtureDB.Exec(`
		CREATE TABLE proxmox_api_configs (
			HYPERVISOR_MAC text PRIMARY KEY,
			ENABLED numeric NOT NULL,
			BASE_URL text,
			TOKEN_ID text,
			TOKEN_SECRET text,
			VERIFY_TLS numeric NOT NULL,
			TIMEOUT_SECONDS integer NOT NULL,
			LAST_ATTEMPT_AT text,
			LAST_SUCCESSFUL_SYNC text,
			LAST_ERROR text,
			STATUS text,
			UPDATED_AT text
		)
	`).Error; err != nil {
		t.Fatalf("create Phase 35.8 table: %v", err)
	}

	if err := fixtureDB.Exec(`
		INSERT INTO proxmox_api_configs (
			HYPERVISOR_MAC, ENABLED, BASE_URL, TOKEN_ID, TOKEN_SECRET,
			VERIFY_TLS, TIMEOUT_SECONDS, LAST_ATTEMPT_AT, LAST_SUCCESSFUL_SYNC,
			LAST_ERROR, STATUS, UPDATED_AT
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"AA:BB:CC:DD:EE:C1", true, "https://10.4.1.6:8006", "lannventory@pve!inventory",
		"migration-secret", true, 10, "2026-09-23T16:40:00Z", "2026-09-23T16:41:00Z",
		"", "connected", "2026-09-23T16:41:00Z",
	).Error; err != nil {
		t.Fatalf("insert Phase 35.8 config: %v", err)
	}

	if err := migrate(fixtureDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var got models.ProxmoxAPIConfig
	if err := fixtureDB.Table(proxmoxAPIConfigsTable).
		Where(`"HYPERVISOR_MAC" = ?`, "AA:BB:CC:DD:EE:C1").
		First(&got).Error; err != nil {
		t.Fatalf("read migrated config: %v", err)
	}

	if !got.Enabled || got.BaseURL != "https://10.4.1.6:8006" || got.TokenID != "lannventory@pve!inventory" ||
		got.TokenSecret != "migration-secret" || !got.VerifyTLS || got.TimeoutSeconds != 10 ||
		got.LastAttemptAt != "2026-09-23T16:40:00Z" || got.LastSuccessfulSync != "2026-09-23T16:41:00Z" ||
		got.Status != "connected" {
		t.Fatalf("Phase 35.8 fields changed during migration: %+v", got)
	}
	if got.AutomaticSync {
		t.Fatal("automatic sync must remain disabled after migration")
	}
	if got.SyncIntervalMinutes != 60 {
		t.Fatalf("migrated sync interval = %d, want 60", got.SyncIntervalMinutes)
	}
	if got.ConfigRevision != 0 {
		t.Fatalf("migrated config revision = %d, want 0", got.ConfigRevision)
	}
	if got.LastSyncAttemptAt != "" || got.LastSuccessfulCollectionAt != "" || got.NextSyncAt != "" ||
		got.LastSyncStatus != "" || got.LastSyncError != "" || got.LastSyncTrigger != "" {
		t.Fatalf("migration invented scheduler runtime state: %+v", got)
	}
}
