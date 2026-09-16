import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import {
  apiGetDiagnostics,
  refreshSharedScannerStatus,
  scannerClockOffsetMs,
  scannerCountdownLabel,
  sharedScannerStatus,
  type DiagnosticsItem,
  type DiagnosticsReport,
} from "../../functions/scannerApi";

function Diagnostics() {
  const [report, setReport] = createSignal<DiagnosticsReport>();
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal("");
  const [clockOffset, setClockOffset] = createSignal(0);
  const [tick, setTick] = createSignal(Date.now());

  const load = async () => {
    setLoading(true);
    setError("");

    try {
      const [scanner, diagnostics] = await Promise.all([
        refreshSharedScannerStatus(),
        apiGetDiagnostics(),
      ]);
      setClockOffset(scannerClockOffsetMs(scanner));
      setReport(diagnostics);
    } catch (loadError) {
      setError(loadError instanceof Error && loadError.message ? loadError.message : "Diagnostics could not be loaded.");
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    void load();
    const timer = window.setInterval(() => setTick(Date.now()), 1000);
    onCleanup(() => window.clearInterval(timer));
  });

  const scanner = sharedScannerStatus;
  const scannerClass = () => {
    const status = scanner()?.status;
    if (status === "problem") return " is-error";
    if (status === "scanning") return " is-scanning";
    return " is-ok";
  };
  const scannerLabel = () => {
    const status = scanner();
    if (!status) return "Unavailable";
    if (status.status === "problem") return "Problem";
    if (status.status === "scanning") return "Scanning";
    return "Healthy";
  };
  const nextScanLabel = () => {
    tick();
    const status = scanner();
    return status ? scannerCountdownLabel(status, clockOffset()) : "Unknown";
  };
  const formatTimestamp = (value: string | null | undefined) => {
    if (!value) return "Not available";
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
  };
  const formatDuration = (value: number | undefined) => {
    if (value === undefined || value < 0) return "Not available";
    if (value < 1000) return Math.round(value) + " ms";
    return (value / 1000).toFixed(1) + " sec";
  };
  const diagnosticIcon = (item: DiagnosticsItem) => {
    if (item.status === "ok") return "bi bi-check-circle-fill";
    if (item.status === "warning") return "bi bi-exclamation-circle-fill";
    return "bi bi-x-circle-fill";
  };

  return (
    <>
      <div class="card wyl-panel config-panel diagnostics-health-panel">
        <div class="card-header diagnostics-card-header">
          <span>Scanner Health</span>
          <span class={"diagnostics-summary-badge" + scannerClass()}>
            <i class={scanner()?.status === "scanning" ? "bi bi-arrow-repeat" : scanner()?.status === "problem" ? "bi bi-exclamation-triangle-fill" : "bi bi-check-circle-fill"} aria-hidden="true"></i>
            {scannerLabel()}
          </span>
        </div>
        <div class="card-body">
          <Show when={scanner()} fallback={<div class="config-field-helper">Scanner status has not loaded yet.</div>}>
            <dl class="diagnostics-health-grid">
              <div><dt>Next scan</dt><dd>{nextScanLabel()}</dd></div>
              <div><dt>Last scan</dt><dd>{formatTimestamp(scanner()?.lastScanAt)}</dd></div>
              <div><dt>Last successful</dt><dd>{formatTimestamp(scanner()?.lastSuccessfulScanAt)}</dd></div>
              <div><dt>Duration</dt><dd>{formatDuration(scanner()?.durationMs)}</dd></div>
              <div><dt>Devices found</dt><dd>{scanner()?.devicesFound ?? 0}</dd></div>
              <div><dt>Interfaces</dt><dd>{scanner()?.interfaces?.join(", ") || "None"}</dd></div>
              <div><dt>Database</dt><dd>{scanner()?.database?.status === "connected" ? ((scanner()?.database?.backend || "database") + " connected") : (scanner()?.database?.error || "Problem")}</dd></div>
            </dl>
            <Show when={scanner()?.lastError}>
              {(lastError) => (
                <div class="diagnostics-last-error" role="alert">
                  <strong>Last scanner error:</strong> {lastError().source || "scanner"} · {lastError().message}
                  <Show when={lastError().output}><div>{lastError().output}</div></Show>
                </div>
              )}
            </Show>
          </Show>
        </div>
      </div>

      <div class="card wyl-panel config-panel diagnostics-panel">
        <div class="card-header diagnostics-card-header">
          <span>Diagnostics</span>
          <button type="button" class="btn btn-sm wyl-button" disabled={loading()} onClick={() => void load()}>
            <i class={loading() ? "bi bi-arrow-repeat" : "bi bi-activity"} aria-hidden="true"></i>
            <span>{loading() ? "Running" : "Run checks"}</span>
          </button>
        </div>
        <div class="card-body">
          <Show when={!error()} fallback={<div class="settings-status-panel settings-status-error" role="alert">{error()}</div>}>
            <Show when={report()} fallback={<div class="config-field-helper">Running diagnostics…</div>}>
              {(current) => (
                <>
                  <div class={"diagnostics-overall" + (current().ok ? " is-ok" : " is-error")}>
                    <i class={current().ok ? "bi bi-check-circle-fill" : "bi bi-exclamation-triangle-fill"} aria-hidden="true"></i>
                    <span>{current().ok ? "All critical checks passed" : "Diagnostics found one or more problems"}</span>
                  </div>
                  <div class="diagnostics-check-list">
                    <For each={current().checks}>{item => (
                      <div class={"diagnostics-check is-" + item.status}>
                        <i class={diagnosticIcon(item)} aria-hidden="true"></i>
                        <div class="diagnostics-check-body">
                          <div class="diagnostics-check-title">{item.check}</div>
                          <div class="diagnostics-check-details">{item.details}</div>
                          <Show when={item.suggestedFix}>
                            <div class="diagnostics-check-fix"><strong>Suggested fix:</strong> {item.suggestedFix}</div>
                          </Show>
                        </div>
                      </div>
                    )}</For>
                  </div>
                </>
              )}
            </Show>
          </Show>
        </div>
      </div>
    </>
  );
}

export default Diagnostics;
