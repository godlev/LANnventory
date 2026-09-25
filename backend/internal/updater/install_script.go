package updater

import (
	"encoding/json"
	"fmt"
	"strings"
)

type applyScriptParams struct {
	ActualHashHex  string
	DebPath        string
	BackupDir      string
	CurrentVersion string
	TargetVersion  string
	HealthURL      string
	UpdateDir      string
	ProgressPath   string
	AttemptID      string
	StartedAt      string
	SystemdUnit    string
}

func buildApplyUpdateScript(params applyScriptParams) string {
	var builder strings.Builder

	attemptJSON := jsonStringLiteral(params.AttemptID)
	previousJSON := jsonStringLiteral(params.CurrentVersion)
	targetJSON := jsonStringLiteral(params.TargetVersion)
	startedJSON := jsonStringLiteral(params.StartedAt)
	backupJSON := jsonStringLiteral(params.BackupDir)
	unitJSON := jsonStringLiteral(params.SystemdUnit)

	fmt.Fprintln(&builder, "#!/bin/sh")
	fmt.Fprintln(&builder, "set -eu")
	fmt.Fprintf(&builder, "progress_path=%s\n", shellQuote(params.ProgressPath))
	fmt.Fprintf(&builder, "attempt_json=%s\n", shellQuote(attemptJSON))
	fmt.Fprintf(&builder, "previous_json=%s\n", shellQuote(previousJSON))
	fmt.Fprintf(&builder, "target_json=%s\n", shellQuote(targetJSON))
	fmt.Fprintf(&builder, "started_json=%s\n", shellQuote(startedJSON))
	fmt.Fprintf(&builder, "backup_json=%s\n", shellQuote(backupJSON))
	fmt.Fprintf(&builder, "unit_json=%s\n", shellQuote(unitJSON))
	fmt.Fprintln(&builder, `write_progress() {`)
	fmt.Fprintln(&builder, `  status="$1"`)
	fmt.Fprintln(&builder, `  stage="$2"`)
	fmt.Fprintln(&builder, `  backup_created="$3"`)
	fmt.Fprintln(&builder, `  failed_stage="$4"`)
	fmt.Fprintln(&builder, `  error_message="$5"`)
	fmt.Fprintln(&builder, `  service_restored="$6"`)
	fmt.Fprintln(&builder, `  completed="$7"`)
	fmt.Fprintln(&builder, `  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"`)
	fmt.Fprintln(&builder, `  completed_at=""`)
	fmt.Fprintln(&builder, `  if [ "$completed" = "true" ]; then completed_at="$now"; fi`)
	fmt.Fprintln(&builder, `  tmp="$progress_path.tmp.$$"`)
	fmt.Fprintln(&builder, `  printf '{"attemptId":%s,"status":"%s","stage":"%s","previousVersion":%s,"targetVersion":%s,"startedAt":%s,"updatedAt":"%s","completedAt":"%s","backupPath":%s,"backupCreated":%s,"failedStage":"%s","error":"%s","serviceRestored":%s,"systemdUnit":%s}\n' "$attempt_json" "$status" "$stage" "$previous_json" "$target_json" "$started_json" "$now" "$completed_at" "$backup_json" "$backup_created" "$failed_stage" "$error_message" "$service_restored" "$unit_json" > "$tmp"`)
	fmt.Fprintln(&builder, `  chmod 600 "$tmp"`)
	fmt.Fprintln(&builder, `  mv "$tmp" "$progress_path"`)
	fmt.Fprintln(&builder, `}`)
	fmt.Fprintln(&builder, `failure_message() {`)
	fmt.Fprintln(&builder, `  case "$current_stage" in`)
	fmt.Fprintln(&builder, `    verifying) echo "Package verification failed." ;;`)
	fmt.Fprintln(&builder, `    backup) echo "Recovery backup failed." ;;`)
	fmt.Fprintln(&builder, `    installing) echo "Package installation failed." ;;`)
	fmt.Fprintln(&builder, `    restarting) echo "LANnventory restart failed." ;;`)
	fmt.Fprintln(&builder, `    health-check) echo "Post-update health check failed." ;;`)
	fmt.Fprintln(&builder, `    *) echo "Update failed." ;;`)
	fmt.Fprintln(&builder, `  esac`)
	fmt.Fprintln(&builder, `}`)
	fmt.Fprintln(&builder, `recover_service() {`)
	fmt.Fprintln(&builder, `  code=$?`)
	fmt.Fprintln(&builder, `  trap - EXIT`)
	fmt.Fprintln(&builder, `  restored=false`)
	fmt.Fprintln(&builder, `  systemctl daemon-reload >/dev/null 2>&1 || true`)
	fmt.Fprintln(&builder, `  systemctl start lannventory >/dev/null 2>&1 || true`)
	fmt.Fprintln(&builder, `  if systemctl is-active --quiet lannventory; then restored=true; fi`)
	fmt.Fprintln(&builder, `  write_progress failed "$current_stage" "$backup_created" "$current_stage" "$(failure_message)" "$restored" true || true`)
	fmt.Fprintf(&builder, "  echo %s >&2\n", shellQuote("LANnventory update did not complete successfully. Recovery files are preserved at "+params.BackupDir))
	fmt.Fprintln(&builder, `  exit "$code"`)
	fmt.Fprintln(&builder, `}`)

	fmt.Fprintln(&builder, "sleep 2")
	fmt.Fprintln(&builder, `current_stage=verifying`)
	fmt.Fprintln(&builder, `backup_created=false`)
	fmt.Fprintln(&builder, `trap recover_service EXIT`)
	fmt.Fprintf(&builder, "echo %s | sha256sum -c -\n", shellQuote(params.ActualHashHex+"  "+params.DebPath))
	fmt.Fprintln(&builder, `current_stage=backup`)
	fmt.Fprintln(&builder, `write_progress running backup false "" "" false false`)
	fmt.Fprintf(&builder, "mkdir -p %s\n", shellQuote(params.BackupDir))
	fmt.Fprintf(&builder, "cp -a /usr/bin/lannventory %s\n", shellQuote(params.BackupDir+"/lannventory"))
	fmt.Fprintf(&builder, "printf '%%s\\n' %s %s > %s\n", shellQuote("from="+params.CurrentVersion), shellQuote("to="+params.TargetVersion), shellQuote(params.BackupDir+"/update.txt"))
	fmt.Fprintln(&builder, `systemctl stop lannventory`)
	fmt.Fprintf(&builder, "if [ -d /etc/watchyourlan ]; then cp -a /etc/watchyourlan %s; fi\n", shellQuote(params.BackupDir+"/watchyourlan"))
	fmt.Fprintln(&builder, `backup_created=true`)
	fmt.Fprintln(&builder, `current_stage=installing`)
	fmt.Fprintln(&builder, `write_progress running installing true "" "" false false`)
	fmt.Fprintf(&builder, "dpkg -i %s\n", shellQuote(params.DebPath))
	fmt.Fprintln(&builder, `current_stage=restarting`)
	fmt.Fprintln(&builder, `write_progress running restarting true "" "" false false`)
	fmt.Fprintln(&builder, `systemctl daemon-reload`)
	fmt.Fprintln(&builder, `systemctl start lannventory`)
	fmt.Fprintln(&builder, `current_stage=health-check`)
	fmt.Fprintln(&builder, `write_progress running health-check true "" "" false false`)
	fmt.Fprintln(&builder, `healthy=0`)
	fmt.Fprintln(&builder, `i=0`)
	fmt.Fprintf(&builder, `while [ "$i" -lt 45 ]; do if systemctl is-active --quiet lannventory && curl -fsS --max-time 3 %s >/dev/null 2>&1; then healthy=1; break; fi; i=$((i + 1)); sleep 2; done
`, shellQuote(params.HealthURL))
	fmt.Fprintf(&builder, "if [ \"$healthy\" -ne 1 ]; then echo %s >&2; exit 1; fi\n", shellQuote("LANnventory did not pass the post-update health check at "+params.HealthURL))
	fmt.Fprintln(&builder, `write_progress complete complete true "" "" true true`)
	fmt.Fprintln(&builder, `trap - EXIT`)
	fmt.Fprintf(&builder, "rm -rf %s\n", shellQuote(params.UpdateDir))

	return builder.String()
}

func jsonStringLiteral(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
