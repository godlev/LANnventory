package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProgressMissingFileReturnsIdle(t *testing.T) {
	service := NewServiceWithURLAndProgressPath(nil, "http://example.invalid/releases", filepath.Join(t.TempDir(), "missing.json"))

	progress, err := service.Progress()
	if err != nil {
		t.Fatalf("Progress() error = %v", err)
	}
	if progress.Status != ProgressStatusIdle || progress.Stage != UpdateStageIdle {
		t.Fatalf("Progress() = %+v, want idle state", progress)
	}
}

func TestPersistProgressRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "update-progress.json")
	service := NewServiceWithURLAndProgressPath(nil, "http://example.invalid/releases", path)
	now := time.Date(2026, 9, 25, 18, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	want := Progress{
		AttemptID:       "beta10-to-beta11-1",
		Status:          ProgressStatusRunning,
		Stage:           UpdateStageBackup,
		PreviousVersion: "0.1.0-beta.10",
		TargetVersion:   "0.1.0-beta.11",
		StartedAt:       "2026-09-25T18:29:00Z",
		BackupPath:      "/var/lib/lannventory/update-backups/example",
		BackupCreated:   true,
	}
	if err := service.persistProgress(want); err != nil {
		t.Fatalf("persistProgress() error = %v", err)
	}

	got, err := service.Progress()
	if err != nil {
		t.Fatalf("Progress() error = %v", err)
	}
	if got.AttemptID != want.AttemptID || got.Status != want.Status || got.Stage != want.Stage {
		t.Fatalf("Progress() = %+v, want identity/status/stage from %+v", got, want)
	}
	if got.UpdatedAt != now.Format(time.RFC3339) {
		t.Fatalf("UpdatedAt = %q, want %q", got.UpdatedAt, now.Format(time.RFC3339))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("progress mode = %o, want 600", info.Mode().Perm())
	}
}

func TestProgressRejectsMalformedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-progress.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	service := NewServiceWithURLAndProgressPath(nil, "http://example.invalid/releases", path)

	_, err := service.Progress()
	if err == nil || !strings.Contains(err.Error(), "decode update progress") {
		t.Fatalf("Progress() error = %v, want decode error", err)
	}
}

func TestPersistProgressRejectsInvalidState(t *testing.T) {
	service := NewServiceWithURLAndProgressPath(nil, "http://example.invalid/releases", filepath.Join(t.TempDir(), "progress.json"))

	if err := service.persistProgress(Progress{Status: "mystery", Stage: UpdateStagePreparing}); err == nil {
		t.Fatal("persistProgress() accepted invalid status")
	}
	if err := service.persistProgress(Progress{Status: ProgressStatusRunning}); err == nil {
		t.Fatal("persistProgress() accepted empty stage")
	}
}
