package updater

import (
	"strings"
	"testing"
)

func TestBuildApplyUpdateScriptTracksDurableStages(t *testing.T) {
	script := buildApplyUpdateScript(applyScriptParams{
		ActualHashHex:  "abc123",
		DebPath:        "/var/tmp/update/lannventory.deb",
		BackupDir:      "/var/lib/lannventory/update-backups/example",
		CurrentVersion: "0.1.0-beta.10",
		TargetVersion:  "0.1.0-beta.11",
		HealthURL:      "http://127.0.0.1:8840/api/health",
		UpdateDir:      "/var/tmp/update",
		ProgressPath:   "/var/lib/lannventory/update-progress.json",
		AttemptID:      "update-1",
		StartedAt:      "2026-09-25T18:00:00Z",
		SystemdUnit:    "lannventory-update-1",
	})

	for _, want := range []string{
		"write_progress running backup",
		"write_progress running installing",
		"write_progress running restarting",
		"write_progress running health-check",
		"write_progress complete complete",
		"write_progress failed",
		"Package installation failed.",
		"Post-update health check failed.",
		"progress_path='/var/lib/lannventory/update-progress.json'",
		"\"systemdUnit\":%s",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q\n%s", want, script)
		}
	}
}

func TestBuildApplyUpdateScriptPreservesExistingSafetyFlow(t *testing.T) {
	script := buildApplyUpdateScript(applyScriptParams{
		ActualHashHex:  "abc123",
		DebPath:        "/tmp/pkg.deb",
		BackupDir:      "/tmp/backup",
		CurrentVersion: "0.1.0-beta.10",
		TargetVersion:  "0.1.0-beta.11",
		HealthURL:      "http://127.0.0.1:8840/api/health",
		UpdateDir:      "/tmp/update",
		ProgressPath:   "/tmp/progress.json",
		AttemptID:      "update-1",
		StartedAt:      "2026-09-25T18:00:00Z",
		SystemdUnit:    "lannventory-update-1",
	})

	checksumIndex := strings.Index(script, "sha256sum -c -")
	stopIndex := strings.Index(script, "systemctl stop lannventory")
	installIndex := strings.Index(script, "dpkg -i")
	startIndex := -1
	if installIndex >= 0 {
		if offset := strings.Index(script[installIndex:], "systemctl start lannventory"); offset >= 0 {
			startIndex = installIndex + offset
		}
	}
	healthIndex := strings.Index(script, "curl -fsS --max-time 3")
	if checksumIndex < 0 || stopIndex < 0 || installIndex < 0 || startIndex < 0 || healthIndex < 0 {
		t.Fatalf("script is missing one or more safety operations:\n%s", script)
	}
	if !(checksumIndex < stopIndex && stopIndex < installIndex && installIndex < startIndex && startIndex < healthIndex) {
		t.Fatalf("update safety operation order changed:\n%s", script)
	}
}
