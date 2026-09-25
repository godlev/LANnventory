package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultProgressPath = "/var/lib/lannventory/update-progress.json"

const (
	ProgressStatusIdle     = "idle"
	ProgressStatusRunning  = "running"
	ProgressStatusComplete = "complete"
	ProgressStatusFailed   = "failed"

	UpdateStageIdle        = "idle"
	UpdateStagePreparing   = "preparing"
	UpdateStageDownloading = "downloading"
	UpdateStageVerifying   = "verifying"
	UpdateStageBackup      = "backup"
	UpdateStageInstalling  = "installing"
	UpdateStageRestarting  = "restarting"
	UpdateStageHealth      = "health-check"
	UpdateStageComplete    = "complete"
)

// Progress is the durable state of the current or most recent package-update attempt.
// It intentionally contains no package bytes, credentials, or command output.
type Progress struct {
	AttemptID       string `json:"attemptId"`
	Status          string `json:"status"`
	Stage           string `json:"stage"`
	PreviousVersion string `json:"previousVersion"`
	TargetVersion   string `json:"targetVersion"`
	StartedAt       string `json:"startedAt"`
	UpdatedAt       string `json:"updatedAt"`
	CompletedAt     string `json:"completedAt"`
	BackupPath      string `json:"backupPath"`
	BackupCreated   bool   `json:"backupCreated"`
	FailedStage     string `json:"failedStage"`
	Error           string `json:"error"`
	ServiceRestored bool   `json:"serviceRestored"`
	SystemdUnit      string `json:"systemdUnit"`
}

func idleProgress() Progress {
	return Progress{
		Status: ProgressStatusIdle,
		Stage:  UpdateStageIdle,
	}
}

// Progress returns persisted update state. A missing state file means no update attempt
// has been recorded on this installation yet.
func (s *Service) Progress() (Progress, error) {
	path := strings.TrimSpace(s.progressPath)
	if path == "" {
		return idleProgress(), nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return idleProgress(), nil
	}
	if err != nil {
		return Progress{}, fmt.Errorf("read update progress: %w", err)
	}

	var progress Progress
	if err := json.Unmarshal(data, &progress); err != nil {
		return Progress{}, fmt.Errorf("decode update progress: %w", err)
	}
	if !validProgressStatus(progress.Status) {
		return Progress{}, fmt.Errorf("decode update progress: invalid status %q", progress.Status)
	}
	if strings.TrimSpace(progress.Stage) == "" {
		return Progress{}, errors.New("decode update progress: stage is required")
	}
	return progress, nil
}

func (s *Service) persistProgress(progress Progress) error {
	path := strings.TrimSpace(s.progressPath)
	if path == "" {
		return errors.New("update progress path is empty")
	}
	if !validProgressStatus(progress.Status) {
		return fmt.Errorf("invalid update progress status %q", progress.Status)
	}
	if strings.TrimSpace(progress.Stage) == "" {
		return errors.New("update progress stage is required")
	}
	progress.UpdatedAt = s.now().UTC().Format(time.RFC3339)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create update progress directory: %w", err)
	}

	file, err := os.CreateTemp(dir, ".update-progress-*")
	if err != nil {
		return fmt.Errorf("create update progress temp file: %w", err)
	}
	tempPath := file.Name()
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure update progress temp file: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(progress); err != nil {
		return fmt.Errorf("encode update progress: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync update progress: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close update progress: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace update progress: %w", err)
	}
	cleanup = false
	return nil
}

func validProgressStatus(status string) bool {
	switch status {
	case ProgressStatusIdle, ProgressStatusRunning, ProgressStatusComplete, ProgressStatusFailed:
		return true
	default:
		return false
	}
}
