import { createEffect, createMemo, createSignal, For, Show } from "solid-js";

import {
  apiApplyProxmoxImport,
  apiDeleteInfrastructureWorkloadLink,
  apiGetHostWorkloadMatches,
  apiGetHostWorkloads,
  apiGetProxmoxSourceState,
  apiPreviewProxmoxImport,
  apiSetInfrastructureWorkloadLink,
  type InfrastructureWorkload,
  type InfrastructureWorkloadMatch,
  type ProxmoxImportPreview,
  type ProxmoxSnapshot,
  type WorkloadMatchCandidate,
} from "../../functions/api";
import { formatLastSeen } from "../../functions/dateFormat";
import type { Host } from "../../functions/exports";

type Props = {
  host: Host;
};

const maxImportBytes = 2 * 1024 * 1024;

function ProxmoxInventoryCard(props: Props) {
  const [sourceState, setSourceState] = createSignal<Awaited<ReturnType<typeof apiGetProxmoxSourceState>>>(null);
  const [workloads, setWorkloads] = createSignal<InfrastructureWorkload[]>([]);
  const [matches, setMatches] = createSignal<InfrastructureWorkloadMatch[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal("");
  const [importText, setImportText] = createSignal("");
  const [snapshot, setSnapshot] = createSignal<ProxmoxSnapshot | null>(null);
  const [preview, setPreview] = createSignal<ProxmoxImportPreview | null>(null);
  const [previewing, setPreviewing] = createSignal(false);
  const [applying, setApplying] = createSignal(false);
  const [linkBusy, setLinkBusy] = createSignal(0);
  const [importError, setImportError] = createSignal("");
  const [importStatus, setImportStatus] = createSignal("");
  let requestID = 0;

  const activeWorkloads = createMemo(() => workloads().filter((item) => !item.retiredAt));
  const totalCount = createMemo(() => activeWorkloads().length);
  const runningCount = createMemo(() => activeWorkloads().filter((item) => item.status === "running").length);
  const stoppedCount = createMemo(() => activeWorkloads().filter((item) => item.status === "stopped").length);
  const retiredCount = createMemo(() => workloads().filter((item) => Boolean(item.retiredAt)).length);
  const matchedCount = createMemo(() => activeWorkloads().filter((item) => item.link !== null).length);
  const matchesByWorkload = createMemo(() => {
    const result = new Map<number, InfrastructureWorkloadMatch>();
    for (const item of matches()) {
      result.set(item.workloadId, item);
    }
    return result;
  });

  createEffect(() => {
    const id = props.host.ID;
    requestID++;
    setSourceState(null);
    setWorkloads([]);
    setMatches([]);
    setLoadError("");
    clearImport();
    if (id < 1) return;
    void refresh(id);
  });

  const refresh = async (id = props.host.ID) => {
    if (id < 1) return;
    const activeRequest = ++requestID;
    setLoading(true);
    setLoadError("");
    try {
      const [nextSource, nextWorkloads, nextMatches] = await Promise.all([
        apiGetProxmoxSourceState(id),
        apiGetHostWorkloads(id),
        apiGetHostWorkloadMatches(id),
      ]);
      if (activeRequest !== requestID) return;
      setSourceState(nextSource);
      setWorkloads(nextWorkloads ?? []);
      setMatches(nextMatches ?? []);
    } catch {
      if (activeRequest !== requestID) return;
      setLoadError("Proxmox inventory could not be loaded.");
    } finally {
      if (activeRequest === requestID) setLoading(false);
    }
  };

  const clearPreview = () => {
    setSnapshot(null);
    setPreview(null);
    setImportError("");
    setImportStatus("");
  };

  const clearImport = () => {
    setImportText("");
    setSnapshot(null);
    setPreview(null);
    setImportError("");
    setImportStatus("");
  };

  const handleImportText = (value: string) => {
    setImportText(value);
    clearPreview();
  };

  const handleFile = async (file?: File) => {
    if (!file) return;
    setImportError("");
    setImportStatus("");
    if (file.size > maxImportBytes) {
      setImportError("Collector JSON is larger than the 2 MiB import limit.");
      return;
    }
    try {
      const text = await file.text();
      setImportText(text);
      setSnapshot(null);
      setPreview(null);
      setImportStatus("Loaded "+file.name+". Review with Preview before applying.");
    } catch {
      setImportError("The selected JSON file could not be read.");
    }
  };

  const parseSnapshot = (): ProxmoxSnapshot | null => {
    const text = importText().trim();
    if (!text) {
      setImportError("Paste collector JSON or choose a JSON file first.");
      return null;
    }
    if (new Blob([text]).size > maxImportBytes) {
      setImportError("Collector JSON is larger than the 2 MiB import limit.");
      return null;
    }
    try {
      const parsed = JSON.parse(text);
      if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error("invalid root");
      }
      return parsed as ProxmoxSnapshot;
    } catch {
      setImportError("Collector output is not valid JSON.");
      return null;
    }
  };

  const handlePreview = async () => {
    if (previewing() || props.host.ID < 1) return;
    setImportError("");
    setImportStatus("");
    const parsed = parseSnapshot();
    if (!parsed) return;

    setPreviewing(true);
    try {
      const next = await apiPreviewProxmoxImport(props.host.ID, parsed);
      setSnapshot(parsed);
      setPreview(next);
      setImportStatus(next.applyAllowed
        ? "Preview ready. No changes have been written yet."
        : "Preview ready, but Apply is blocked until the listed issues are resolved.");
    } catch (error) {
      setSnapshot(null);
      setPreview(null);
      setImportError(apiErrorMessage(error, "Snapshot validation failed."));
    } finally {
      setPreviewing(false);
    }
  };

  const handleApply = async () => {
    const currentPreview = preview();
    const currentSnapshot = snapshot();
    if (!currentPreview || !currentSnapshot || !currentPreview.applyAllowed || applying() || props.host.ID < 1) return;

    const summary = currentPreview.summary;
    const accepted = window.confirm(
      "Apply this reviewed Proxmox snapshot?\n\n"+
      "Add: "+summary.added+
      " · Update: "+summary.updated+
      " · Retire: "+summary.retired+
      " · Unchanged: "+summary.unchanged+
      "\n\nManaged Device Profile fields will not be overwritten."
    );
    if (!accepted) return;

    setApplying(true);
    setImportError("");
    setImportStatus("");
    try {
      const result = await apiApplyProxmoxImport(
        props.host.ID,
        currentPreview.previewToken,
        currentSnapshot,
      );
      setPreview(null);
      setSnapshot(null);
      setImportStatus("Snapshot applied at "+formatTimestamp(result.importedAt)+".");
      await refresh(props.host.ID);
    } catch (error) {
      setImportError(apiErrorMessage(error, "Proxmox snapshot could not be applied."));
    } finally {
      setApplying(false);
    }
  };

  const linkCandidate = async (workloadID: number, hostID: number) => {
    if (linkBusy() !== 0 || props.host.ID < 1) return;
    setLinkBusy(workloadID);
    setLoadError("");
    try {
      await apiSetInfrastructureWorkloadLink(props.host.ID, workloadID, hostID);
      await refresh(props.host.ID);
    } catch (error) {
      setLoadError(apiErrorMessage(error, "Workload match could not be saved."));
    } finally {
      setLinkBusy(0);
    }
  };

  const unlinkWorkload = async (workloadID: number) => {
    if (linkBusy() !== 0 || props.host.ID < 1) return;
    setLinkBusy(workloadID);
    setLoadError("");
    try {
      await apiDeleteInfrastructureWorkloadLink(props.host.ID, workloadID);
      await refresh(props.host.ID);
    } catch (error) {
      setLoadError(apiErrorMessage(error, "Workload match could not be removed."));
    } finally {
      setLinkBusy(0);
    }
  };

  return (
    <section class="card wyl-panel host-panel proxmox-panel" aria-labelledby="proxmox-inventory-title">
      <div class="card-header host-panel-header">
        <div>
          <div id="proxmox-inventory-title" class="host-panel-title">Proxmox inventory</div>
          <div class="host-panel-subtitle">
            Imported source data and hosted workloads. Managed profile fields above remain independent.
          </div>
        </div>
        <div class="d-flex align-items-center gap-2">
          <span class="host-detail-section-badge">Imported</span>
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            disabled={loading() || props.host.ID < 1}
            onClick={() => void refresh()}
          >
            <i class={loading() ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-arrow-clockwise me-1"} aria-hidden="true"></i>
            Refresh
          </button>
        </div>
      </div>

      <div class="card-body">
        <Show when={loadError()}>
          <div class="host-inline-error mb-3" role="alert">{loadError()}</div>
        </Show>

        <Show
          when={!loading() || sourceState() !== null || workloads().length > 0}
          fallback={<div class="device-cell-muted">Loading Proxmox inventory…</div>}
        >
          <SourceSummary
            state={sourceState()}
            total={totalCount()}
            running={runningCount()}
            stopped={stoppedCount()}
            retired={retiredCount()}
            matched={matchedCount()}
          />

          <div class="proxmox-section">
            <div class="proxmox-section-heading">
              <div>
                <div class="small fw-semibold">Workloads</div>
                <div class="small device-cell-muted">
                  VM/LXC inventory is separate from LANnventory Hosts. Exact MAC matches may link automatically; weaker evidence requires your choice.
                </div>
              </div>
              <span class="badge text-bg-secondary">{workloads().length}</span>
            </div>

            <Show
              when={workloads().length > 0}
              fallback={
                <div class="proxmox-empty">
                  <i class="bi bi-hdd-stack" aria-hidden="true"></i>
                  <span>No workloads imported yet.</span>
                </div>
              }
            >
              <div class="table-responsive proxmox-workload-table-wrap">
                <table class="table table-sm align-middle mb-0 proxmox-workload-table">
                  <thead>
                    <tr>
                      <th>Guest</th>
                      <th>Status</th>
                      <th>Network identity</th>
                      <th>LANnventory match</th>
                    </tr>
                  </thead>
                  <tbody>
                    <For each={workloads()}>
                      {(workload) => (
                        <WorkloadRow
                          workload={workload}
                          match={matchesByWorkload().get(workload.id)}
                          busy={linkBusy() === workload.id}
                          onLink={(hostID) => void linkCandidate(workload.id, hostID)}
                          onUnlink={() => void unlinkWorkload(workload.id)}
                        />
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </Show>
          </div>

          <ImportSection
            importText={importText()}
            preview={preview()}
            previewing={previewing()}
            applying={applying()}
            error={importError()}
            status={importStatus()}
            onText={handleImportText}
            onFile={(file) => void handleFile(file)}
            onClear={clearImport}
            onPreview={() => void handlePreview()}
            onApply={() => void handleApply()}
          />
        </Show>
      </div>
    </section>
  );
}

function SourceSummary(props: {
  state: Awaited<ReturnType<typeof apiGetProxmoxSourceState>>;
  total: number;
  running: number;
  stopped: number;
  retired: number;
  matched: number;
}) {
  return (
    <div class="proxmox-source-block">
      <div class="proxmox-section-heading">
        <div>
          <div class="small fw-semibold">Source state</div>
          <div class="small device-cell-muted">Last successfully applied read-only collector snapshot.</div>
        </div>
        <Show when={props.state} fallback={<span class="badge text-bg-secondary">Not imported</span>}>
          {(state) => (
            <span class={"badge "+(state().nodeStatus === "online" ? "text-bg-success" : state().nodeStatus === "offline" ? "text-bg-danger" : "text-bg-secondary")}>
              {capitalize(state().nodeStatus)}
            </span>
          )}
        </Show>
      </div>

      <Show
        when={props.state}
        fallback={
          <div class="proxmox-empty proxmox-empty-source">
            <i class="bi bi-box-arrow-in-down" aria-hidden="true"></i>
            <div>
              <div class="fw-semibold">No collector snapshot has been applied.</div>
              <div class="small device-cell-muted">Use the import workflow below to preview the read-only collector output first.</div>
            </div>
          </div>
        }
      >
        {(state) => (
          <div class="proxmox-source-grid">
            <SourceMetric label="Node" value={state().nodeHostname} monospace />
            <SourceMetric label="PVE version" value={state().nodePveVersion} />
            <SourceMetric label="Cluster" value={state().nodeClusterName || "Standalone"} />
            <SourceMetric label="Source" value="Script import" />
            <SourceMetric label="Collected" value={formatTimestamp(state().collectedAt)} />
            <SourceMetric label="Imported" value={formatTimestamp(state().importedAt)} />
            <SourceMetric label="Collector" value={state().collectorVersion} />
            <SourceMetric label="Schema" value={"v"+state().schemaVersion} />
          </div>
        )}
      </Show>

      <div class="proxmox-summary-grid">
        <SummaryMetric label="Active" value={props.total} />
        <SummaryMetric label="Running" value={props.running} />
        <SummaryMetric label="Stopped" value={props.stopped} />
        <SummaryMetric label="Matched" value={props.matched} />
        <SummaryMetric label="Retired" value={props.retired} />
      </div>
    </div>
  );
}

function SourceMetric(props: { label: string; value: string; monospace?: boolean }) {
  return (
    <div class="proxmox-source-metric">
      <div class="proxmox-source-label">{props.label}</div>
      <div class={"proxmox-source-value"+(props.monospace ? " font-monospace" : "")}>{props.value || "—"}</div>
    </div>
  );
}

function SummaryMetric(props: { label: string; value: number }) {
  return (
    <div class="proxmox-summary-metric">
      <div class="proxmox-summary-value">{props.value}</div>
      <div class="proxmox-summary-label">{props.label}</div>
    </div>
  );
}

function WorkloadRow(props: {
  workload: InfrastructureWorkload;
  match?: InfrastructureWorkloadMatch;
  busy: boolean;
  onLink: (hostID: number) => void;
  onUnlink: () => void;
}) {
  const retired = () => Boolean(props.workload.retiredAt);
  return (
    <tr class={retired() ? "proxmox-workload-retired" : ""}>
      <td>
        <div class="proxmox-workload-title">
          <span class="font-monospace proxmox-vmid">{props.workload.nativeId}</span>
          <span class="fw-semibold">{props.workload.name || "Unnamed workload"}</span>
        </div>
        <div class="proxmox-workload-meta">
          <span class="badge text-bg-secondary">{props.workload.workloadType === "container" ? "LXC" : "VM"}</span>
          <span>{props.workload.source === "script-import" ? "Imported" : "Manual"}</span>
          <Show when={retired()}>
            <span>Retired {formatTimestamp(props.workload.retiredAt)}</span>
          </Show>
        </div>
      </td>
      <td>
        <span class={"proxmox-status "+statusClass(props.workload.status)}>
          <i class={props.workload.status === "running" ? "bi bi-play-circle-fill" : props.workload.status === "stopped" ? "bi bi-stop-circle-fill" : "bi bi-question-circle-fill"} aria-hidden="true"></i>
          {capitalize(props.workload.status)}
        </span>
      </td>
      <td>
        <Show
          when={props.workload.interfaces.length > 0}
          fallback={<span class="device-cell-muted">No network identity</span>}
        >
          <div class="proxmox-interface-list">
            <For each={props.workload.interfaces}>
              {(iface) => (
                <div class="proxmox-interface">
                  <span class="proxmox-interface-name">{iface.name}</span>
                  <Show when={iface.mac}><span class="font-monospace">{iface.mac}</span></Show>
                  <Show when={iface.configuredAddress}><span>{iface.configuredAddress}</span></Show>
                  <Show when={iface.bridge}><span class="device-cell-muted">{iface.bridge}{iface.vlanTag ? " · VLAN "+iface.vlanTag : ""}</span></Show>
                </div>
              )}
            </For>
          </div>
        </Show>
      </td>
      <td>
        <MatchCell
          workload={props.workload}
          match={props.match}
          busy={props.busy}
          onLink={props.onLink}
          onUnlink={props.onUnlink}
        />
      </td>
    </tr>
  );
}

function MatchCell(props: {
  workload: InfrastructureWorkload;
  match?: InfrastructureWorkloadMatch;
  busy: boolean;
  onLink: (hostID: number) => void;
  onUnlink: () => void;
}) {
  const currentHost = () => props.workload.matchedHost;
  const currentLink = () => props.workload.link;

  return (
    <div class="proxmox-match-cell">
      <Show
        when={currentLink() && currentHost()}
        fallback={
          <>
            <Show when={props.match?.exactAmbiguous}>
              <div class="proxmox-match-warning">
                <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
                Multiple exact MAC candidates — choose the correct Host.
              </div>
            </Show>
            <Show
              when={(props.match?.candidates?.length ?? 0) > 0}
              fallback={<span class="device-cell-muted">No Host candidates</span>}
            >
              <div class="proxmox-candidate-list">
                <For each={props.match?.candidates ?? []}>
                  {(candidate) => (
                    <CandidateAction
                      candidate={candidate}
                      busy={props.busy}
                      onLink={() => props.onLink(candidate.hostId)}
                    />
                  )}
                </For>
              </div>
            </Show>
          </>
        }
      >
        <div class="proxmox-linked-host">
          <div>
            <div class="fw-semibold">{currentHost()?.name || currentHost()?.ip || currentHost()?.mac}</div>
            <div class="small device-cell-muted">
              {[currentHost()?.ip, currentHost()?.mac].filter(Boolean).join(" · ")}
            </div>
          </div>
          <div class="proxmox-link-actions">
            <span class={"badge "+(currentLink()?.linkSource === "exact-mac" ? "text-bg-success" : "text-bg-primary")}>
              {currentLink()?.linkSource === "exact-mac" ? "Exact MAC" : "Manual"}
            </span>
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              disabled={props.busy}
              onClick={props.onUnlink}
            >
              {props.busy ? "Working…" : "Unlink"}
            </button>
          </div>
        </div>
      </Show>
    </div>
  );
}

function CandidateAction(props: { candidate: WorkloadMatchCandidate; busy: boolean; onLink: () => void }) {
  const detail = () => props.candidate.evidence.map((item) => item.detail).join("\n");
  return (
    <div class="proxmox-candidate" title={detail()}>
      <div class="proxmox-candidate-body">
        <div class="proxmox-candidate-title">
          <span>{props.candidate.name || props.candidate.ip || props.candidate.mac}</span>
          <span class={"badge "+candidateBadgeClass(props.candidate.strength)}>{candidateLabel(props.candidate.strength)}</span>
        </div>
        <div class="small device-cell-muted">
          {[props.candidate.ip, props.candidate.mac].filter(Boolean).join(" · ")}
        </div>
      </div>
      <button
        type="button"
        class="btn btn-sm btn-outline-primary"
        disabled={props.busy}
        onClick={props.onLink}
      >
        Link
      </button>
    </div>
  );
}

function ImportSection(props: {
  importText: string;
  preview: ProxmoxImportPreview | null;
  previewing: boolean;
  applying: boolean;
  error: string;
  status: string;
  onText: (value: string) => void;
  onFile: (file?: File) => void;
  onClear: () => void;
  onPreview: () => void;
  onApply: () => void;
}) {
  return (
    <div class="proxmox-section proxmox-import-section">
      <div class="proxmox-section-heading">
        <div>
          <div class="small fw-semibold">Import read-only collector snapshot</div>
          <div class="small device-cell-muted">
            Run the collector on the Proxmox node, then paste or upload its JSON. Preview is mandatory before Apply.
          </div>
        </div>
        <span class="host-detail-section-badge">No credentials</span>
      </div>

      <div class="proxmox-collector-command font-monospace">
        python3 proxmox/collect_inventory.py &gt; lannventory-proxmox.json
      </div>

      <div class="proxmox-import-controls">
        <textarea
          class="form-control form-control-sm wyl-control proxmox-import-textarea font-monospace"
          rows={7}
          placeholder="Paste collector JSON here…"
          value={props.importText}
          onInput={(event) => props.onText(event.currentTarget.value)}
        ></textarea>
        <div class="proxmox-import-actions">
          <label class="btn btn-sm btn-outline-secondary mb-0">
            <i class="bi bi-file-earmark-arrow-up me-1" aria-hidden="true"></i>
            Choose JSON
            <input
              class="visually-hidden"
              type="file"
              accept=".json,application/json"
              onChange={(event) => props.onFile(event.currentTarget.files?.[0])}
            />
          </label>
          <button type="button" class="btn btn-sm btn-outline-secondary" disabled={!props.importText || props.previewing || props.applying} onClick={props.onClear}>
            Clear
          </button>
          <button type="button" class="btn btn-sm btn-primary" disabled={!props.importText.trim() || props.previewing || props.applying} onClick={props.onPreview}>
            <i class={props.previewing ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-search me-1"} aria-hidden="true"></i>
            {props.previewing ? "Validating…" : "Preview"}
          </button>
        </div>
      </div>

      <Show when={props.error}>
        <div class="host-inline-error mt-2" role="alert">{props.error}</div>
      </Show>
      <Show when={props.status}>
        <div class="small text-success mt-2">{props.status}</div>
      </Show>

      <Show when={props.preview}>
        {(preview) => (
          <div class="proxmox-preview">
            <div class="proxmox-preview-header">
              <div>
                <div class="fw-semibold">Import preview</div>
                <div class="small device-cell-muted">
                  Collected {formatTimestamp(preview().collectedAt)} · node {preview().node.after.hostname || "unknown"}
                </div>
              </div>
              <span class={"badge "+(preview().applyAllowed ? "text-bg-success" : "text-bg-warning")}>
                {preview().applyAllowed ? "Ready to apply" : "Blocked"}
              </span>
            </div>

            <div class="proxmox-preview-summary">
              <PreviewMetric label="Add" value={preview().summary.added} />
              <PreviewMetric label="Update" value={preview().summary.updated} />
              <PreviewMetric label="Unchanged" value={preview().summary.unchanged} />
              <PreviewMetric label="Retire" value={preview().summary.retired} />
              <PreviewMetric label="Conflicts" value={preview().summary.conflicts} />
            </div>

            <Show when={preview().blockedReasons.length > 0}>
              <div class="proxmox-preview-alert is-blocked">
                <For each={preview().blockedReasons}>{(item) => <div><i class="bi bi-x-circle-fill me-1" aria-hidden="true"></i>{item}</div>}</For>
              </div>
            </Show>
            <Show when={preview().warnings.length > 0}>
              <div class="proxmox-preview-alert">
                <For each={preview().warnings}>{(item) => <div><i class="bi bi-exclamation-triangle-fill me-1" aria-hidden="true"></i>{item}</div>}</For>
              </div>
            </Show>
            <Show when={preview().managedConflicts.length > 0}>
              <div class="proxmox-managed-conflicts">
                <div class="small fw-semibold mb-1">Managed vs imported differences</div>
                <For each={preview().managedConflicts}>
                  {(item) => (
                    <div class="proxmox-conflict-row">
                      <span>{managedFieldLabel(item.field)}</span>
                      <span class="font-monospace">{item.managed || "—"}</span>
                      <i class="bi bi-arrow-left-right" aria-hidden="true"></i>
                      <span class="font-monospace">{item.imported || "—"}</span>
                    </div>
                  )}
                </For>
                <div class="small device-cell-muted mt-1">Managed values will not be overwritten.</div>
              </div>
            </Show>

            <Show when={preview().workloads.length > 0}>
              <div class="table-responsive mt-2">
                <table class="table table-sm align-middle mb-0 proxmox-preview-table">
                  <thead>
                    <tr><th>Action</th><th>Workload</th><th>Changes</th></tr>
                  </thead>
                  <tbody>
                    <For each={preview().workloads}>
                      {(item) => (
                        <tr>
                          <td><span class={"badge "+previewActionClass(item.action)}>{capitalize(item.action)}</span></td>
                          <td>
                            <span class="font-monospace me-2">{item.after?.nativeId ?? item.before?.nativeId ?? item.key}</span>
                            <span>{item.after?.name ?? item.before?.name ?? ""}</span>
                          </td>
                          <td>{item.changes.length ? item.changes.join(", ") : "No changes"}</td>
                        </tr>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </Show>

            <div class="proxmox-preview-footer">
              <div class="small device-cell-muted">Apply revalidates the snapshot and rejects a stale Preview if inventory changed meanwhile.</div>
              <button
                type="button"
                class="btn btn-sm btn-success"
                disabled={!preview().applyAllowed || props.applying}
                onClick={props.onApply}
              >
                <i class={props.applying ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-check2-circle me-1"} aria-hidden="true"></i>
                {props.applying ? "Applying…" : "Apply reviewed snapshot"}
              </button>
            </div>
          </div>
        )}
      </Show>
    </div>
  );
}

function PreviewMetric(props: { label: string; value: number }) {
  return <div><strong>{props.value}</strong><span>{props.label}</span></div>;
}

function statusClass(status: InfrastructureWorkload["status"]) {
  if (status === "running") return "is-running";
  if (status === "stopped") return "is-stopped";
  return "is-unknown";
}

function candidateLabel(strength: WorkloadMatchCandidate["strength"]) {
  if (strength === "exact-mac") return "Exact MAC";
  if (strength === "address") return "Address";
  return "Name";
}

function candidateBadgeClass(strength: WorkloadMatchCandidate["strength"]) {
  if (strength === "exact-mac") return "text-bg-success";
  if (strength === "address") return "text-bg-info";
  return "text-bg-secondary";
}

function previewActionClass(action: string) {
  if (action === "add") return "text-bg-success";
  if (action === "update") return "text-bg-primary";
  if (action === "retire") return "text-bg-warning";
  if (action === "conflict") return "text-bg-danger";
  return "text-bg-secondary";
}

function managedFieldLabel(field: string) {
  if (field === "nodeName") return "Node name";
  if (field === "clusterName") return "Cluster";
  if (field === "version") return "Version";
  return field;
}

function capitalize(value: string) {
  if (!value) return "Unknown";
  return value.charAt(0).toUpperCase()+value.slice(1);
}

function formatTimestamp(value: string) {
  if (!value) return "—";
  return formatLastSeen(value);
}

function apiErrorMessage(error: unknown, fallback: string) {
  if (!(error instanceof Error)) return fallback;
  const message = error.message.trim();
  if (!message) return fallback;
  try {
    const parsed = JSON.parse(message) as { error?: string };
    return parsed.error || fallback;
  } catch {
    return message;
  }
}

export default ProxmoxInventoryCard;
