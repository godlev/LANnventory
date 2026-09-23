import { createEffect, createMemo, createSignal, For, Show } from "solid-js";

import {
  apiApplyProxmoxAPISync,
  apiApplyProxmoxImport,
  apiClearInfrastructureWorkloadCandidateRejection,
  apiDeleteInfrastructureWorkloadLink,
  apiGetHostWorkloadMatches,
  apiGetHostWorkloads,
  apiGetProxmoxAPIConfig,
  apiGetProxmoxSourceState,
  apiPatchProxmoxAPIConfig,
  apiPreviewProxmoxAPISync,
  apiPreviewProxmoxImport,
  apiTestProxmoxAPIConnection,
  apiRejectInfrastructureWorkloadCandidate,
  apiSetInfrastructureWorkloadLink,
  type InfrastructureWorkload,
  type InfrastructureWorkloadMatch,
  type ProxmoxAPIConfig,
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
  const [collectionMode, setCollectionMode] = createSignal<"script" | "api">("script");
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
  const possibleConflictCount = createMemo(() =>
    matches().filter((item) => item.candidates.some((candidate) => candidate.possibleIpConflict && !candidate.rejected)).length
  );
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
    setCollectionMode("script");
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
      const [scriptSource, apiSource, nextWorkloads, nextMatches, apiConfig] = await Promise.all([
        apiGetProxmoxSourceState(id, "script-import"),
        apiGetProxmoxSourceState(id, "proxmox-api"),
        apiGetHostWorkloads(id),
        apiGetHostWorkloadMatches(id),
        apiGetProxmoxAPIConfig(id),
      ]);
      if (activeRequest !== requestID) return;
      const nextSource = newestProxmoxSource(scriptSource, apiSource);
      setSourceState(nextSource);
      setWorkloads(nextWorkloads ?? []);
      setMatches(nextMatches ?? []);
      if (apiConfig.enabled) setCollectionMode("api");
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
      "Import this reviewed snapshot into LANnventory?\n\n"+
      "New workload records: "+summary.added+
      " · Update: "+summary.updated+
      " · Retire: "+summary.retired+
      " · Unchanged: "+summary.unchanged+
      "\n\nThis changes LANnventory inventory only. It does not create, modify, stop, start, or delete anything on Proxmox. Workloads are not created as LANnventory Hosts. Managed Device Profile fields and manual links are preserved."
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

  const rejectCandidate = async (workloadID: number, candidate: WorkloadMatchCandidate) => {
    if (linkBusy() !== 0 || props.host.ID < 1 || candidate.strength === "exact-mac") return;
    setLinkBusy(workloadID);
    setLoadError("");
    try {
      await apiRejectInfrastructureWorkloadCandidate(
        props.host.ID,
        workloadID,
        candidate.hostId,
        candidate.evidenceFingerprint,
      );
      await refresh(props.host.ID);
    } catch (error) {
      setLoadError(apiErrorMessage(error, "Candidate rejection could not be saved. Refresh and review the current evidence."));
    } finally {
      setLinkBusy(0);
    }
  };

  const clearCandidateRejection = async (workloadID: number, candidateHostID: number) => {
    if (linkBusy() !== 0 || props.host.ID < 1) return;
    setLinkBusy(workloadID);
    setLoadError("");
    try {
      await apiClearInfrastructureWorkloadCandidateRejection(props.host.ID, workloadID, candidateHostID);
      await refresh(props.host.ID);
    } catch (error) {
      setLoadError(apiErrorMessage(error, "Candidate rejection could not be cleared."));
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
          <div class="proxmox-section">
            <div class="proxmox-section-heading">
              <div>
                <div class="small fw-semibold">Proxmox inventory source</div>
                <div class="small device-cell-muted">Choose the collection method. Both paths use the same Preview/Diff and workload matching rules.</div>
              </div>
              <div class="btn-group btn-group-sm" role="group" aria-label="Proxmox inventory source">
                <button
                  type="button"
                  class={"btn "+(collectionMode() === "script" ? "btn-primary" : "btn-outline-secondary")}
                  onClick={() => setCollectionMode("script")}
                >
                  Manual / script
                </button>
                <button
                  type="button"
                  class={"btn "+(collectionMode() === "api" ? "btn-primary" : "btn-outline-secondary")}
                  onClick={() => setCollectionMode("api")}
                >
                  Proxmox API
                </button>
              </div>
            </div>
          </div>

          <SourceSummary
            state={sourceState()}
            total={totalCount()}
            running={runningCount()}
            stopped={stoppedCount()}
            retired={retiredCount()}
            matched={matchedCount()}
            conflicts={possibleConflictCount()}
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
                          onReject={(candidate) => void rejectCandidate(workload.id, candidate)}
                          onClearRejection={(hostID) => void clearCandidateRejection(workload.id, hostID)}
                        />
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </Show>
          </div>

          <Show
            when={collectionMode() === "api"}
            fallback={
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
                proxmoxAddress={props.host.IP}
              />
            }
          >
            <ProxmoxAPISection hostID={props.host.ID} onApplied={() => refresh(props.host.ID)} />
          </Show>
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
  conflicts: number;
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
            <SourceMetric label="Source" value={state().source === "proxmox-api" ? "Proxmox API" : "Script import"} />
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
        <SummaryMetric label="IP conflicts" value={props.conflicts} />
        <SummaryMetric label="Retired" value={props.retired} />
      </div>
    </div>
  );
}

function SourceMetric(props: { label: string; value: string; monospace?: boolean }) {
  return (
    <div class="proxmox-source-metric">
      <div class="proxmox-source-label">
        {props.label}
        <span
          class="profile-source-icon is-imported ms-1"
          title="Imported: observed by the read-only Proxmox collector. Managed profile fields are separate."
          aria-label="Imported from Proxmox collector"
          tabindex="0"
        >
          <i class="bi bi-box-arrow-in-down" aria-hidden="true"></i>
        </span>
      </div>
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
  onReject: (candidate: WorkloadMatchCandidate) => void;
  onClearRejection: (hostID: number) => void;
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
          <span>{props.workload.source === "proxmox-api" ? "API" : props.workload.source === "script-import" ? "Imported" : "Manual"}</span>
          <Show when={props.workload.nodeName}><span>Node {props.workload.nodeName}</span></Show>
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
          onReject={props.onReject}
          onClearRejection={props.onClearRejection}
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
  onReject: (candidate: WorkloadMatchCandidate) => void;
  onClearRejection: (hostID: number) => void;
}) {
  const currentHost = () => props.workload.matchedHost;
  const currentLink = () => props.workload.link;
  const activeCandidates = () => (props.match?.candidates ?? []).filter((candidate) => !candidate.rejected);
  const rejectedCandidates = () => (props.match?.candidates ?? []).filter((candidate) => candidate.rejected);

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
              when={activeCandidates().length > 0}
              fallback={
                <span class="device-cell-muted">
                  {rejectedCandidates().length > 0 ? "No active Host candidates" : "No Host candidates"}
                </span>
              }
            >
              <div class="proxmox-candidate-list">
                <For each={activeCandidates()}>
                  {(candidate) => (
                    <CandidateAction
                      workload={props.workload}
                      candidate={candidate}
                      busy={props.busy}
                      onLink={() => props.onLink(candidate.hostId)}
                      onReject={() => props.onReject(candidate)}
                    />
                  )}
                </For>
              </div>
            </Show>
            <Show when={rejectedCandidates().length > 0}>
              <details class="proxmox-rejected-candidates">
                <summary>
                  Rejected suggestions ({rejectedCandidates().length})
                </summary>
                <For each={rejectedCandidates()}>
                  {(candidate) => (
                    <div class="proxmox-rejected-candidate">
                      <div>
                        <div class="fw-semibold">{candidate.name || candidate.ip || candidate.mac}</div>
                        <div class="small device-cell-muted">
                          {candidateAssessmentLabel(candidate)} · {candidate.ip || "no address"} · {candidate.mac || "no MAC"}
                        </div>
                      </div>
                      <button
                        type="button"
                        class="btn btn-sm btn-outline-secondary"
                        disabled={props.busy}
                        onClick={() => props.onClearRejection(candidate.hostId)}
                      >
                        {props.busy ? "Working…" : "Show again"}
                      </button>
                    </div>
                  )}
                </For>
              </details>
            </Show>
          </>
        }
      >
        <div class="proxmox-linked-host">
          <div>
            <div class="proxmox-linked-state">
              <span class={"badge "+(currentLink()?.linkSource === "exact-mac" ? "text-bg-success" : "text-bg-primary")}>
                {currentLink()?.linkSource === "exact-mac" ? "Linked automatically" : "Linked manually"}
              </span>
              <span class="small device-cell-muted">
                {currentLink()?.linkSource === "exact-mac" ? "Exact MAC" : "Manual choice"}
              </span>
            </div>
            <div class="fw-semibold proxmox-linked-host-name">
              {currentHost()?.name || currentHost()?.ip || currentHost()?.mac}
            </div>
            <div class="small device-cell-muted proxmox-linked-host-identity">
              {[currentHost()?.ip, currentHost()?.mac].filter(Boolean).join(" · ")}
            </div>
          </div>
          <div class="proxmox-link-actions">
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              disabled={props.busy}
              title={currentLink()?.linkSource === "exact-mac"
                ? "Remove this workload ↔ Host relationship. Exact MAC evidence remains and a future import may link it automatically again."
                : "Remove this workload ↔ Host relationship."}
              onClick={props.onUnlink}
            >
              {props.busy ? "Working…" : "Remove link"}
            </button>
          </div>
        </div>
      </Show>
    </div>
  );
}

function CandidateAction(props: {
  workload: InfrastructureWorkload;
  candidate: WorkloadMatchCandidate;
  busy: boolean;
  onLink: () => void;
  onReject: () => void;
}) {
  const matchedAddress = () => props.candidate.matchedAddresses?.[0] ?? "";
  const matchingInterface = () => {
    const address = matchedAddress();
    if (address) {
      const byAddress = props.workload.interfaces.find((iface) => normalizeConfiguredAddress(iface.configuredAddress) === address);
      if (byAddress) return byAddress;
    }
    if (props.candidate.strength === "exact-mac") {
      const targetMAC = normalizeMac(props.candidate.mac);
      const byMAC = props.workload.interfaces.find((iface) => normalizeMac(iface.mac) === targetMAC);
      if (byMAC) return byMAC;
    }
    return props.workload.interfaces[0];
  };
  const workloadIP = () => matchingInterface()?.configuredAddress || matchedAddress() || "—";
  const workloadMAC = () => matchingInterface()?.mac || props.candidate.workloadMacs?.[0] || "—";
  const hostIP = () => props.candidate.ip || matchedAddress() || "—";
  const hostMAC = () => props.candidate.mac || "—";
  const sameAddress = () => Boolean(matchedAddress()) &&
    normalizeConfiguredAddress(workloadIP()) === normalizeConfiguredAddress(hostIP());
  const sameMAC = () => workloadMAC() !== "—" && hostMAC() !== "—" &&
    normalizeMac(workloadMAC()) === normalizeMac(hostMAC());
  const addressEvidence = () => props.candidate.evidence.find((item) => item.strength === "address");
  const hasNameEvidence = () => props.candidate.evidence.some((item) => item.strength === "name");
  const isWeak = () => props.candidate.strength !== "exact-mac";
  const detail = () => props.candidate.evidence.map((item) => item.detail).join("\n");

  return (
    <div
      class={"proxmox-candidate"+(props.candidate.possibleIpConflict ? " is-ip-conflict" : "")}
      title={detail()}
    >
      <Show when={props.candidate.possibleIpConflict}>
        <div class="proxmox-ip-conflict-title">
          <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
          <span>Possible IP conflict</span>
        </div>
      </Show>

      <div class="proxmox-candidate-body">
        <div class="proxmox-candidate-title">
          <span>{props.candidate.name || props.candidate.ip || props.candidate.mac}</span>
          <span class={"badge "+candidateAssessmentBadgeClass(props.candidate)}>
            {candidateAssessmentLabel(props.candidate)}
          </span>
        </div>

        <Show
          when={props.candidate.possibleIpConflict}
          fallback={
            <div class="proxmox-candidate-evidence-summary">
              <strong>{candidateEvidenceSummary(props.candidate)}</strong>
              <Show when={addressEvidence()?.firstSeen || addressEvidence()?.lastSeen}>
                <span class="device-cell-muted">
                  {formatEvidenceWindow(addressEvidence()?.firstSeen, addressEvidence()?.lastSeen)}
                </span>
              </Show>
            </div>
          }
        >
          <div class="proxmox-match-comparison">
            <div class="proxmox-match-side">
              <div class="proxmox-match-side-label">PROXMOX WORKLOAD</div>
              <div class="fw-semibold">{props.workload.name || props.workload.nativeId}</div>
              <div class="proxmox-match-field">
                <span>IP</span>
                <span class="font-monospace">{workloadIP()}</span>
              </div>
              <div class="proxmox-match-field">
                <span>MAC</span>
                <span class="font-monospace">{workloadMAC()}</span>
              </div>
            </div>
            <div class="proxmox-match-side">
              <div class="proxmox-match-side-label">LANnventory HOST</div>
              <div class="fw-semibold">{props.candidate.name || props.candidate.ip || props.candidate.mac}</div>
              <div class="proxmox-match-field">
                <span>IP</span>
                <span class="font-monospace">{hostIP()}</span>
                <span class={sameAddress() ? "text-success" : "text-danger"} aria-label={sameAddress() ? "same" : "different"}>
                  {sameAddress() ? "✓ same" : "✕ different"}
                </span>
              </div>
              <div class="proxmox-match-field">
                <span>MAC</span>
                <span class="font-monospace">{hostMAC()}</span>
                <span class={sameMAC() ? "text-success" : "text-danger"} aria-label={sameMAC() ? "same" : "different"}>
                  {sameMAC() ? "✓ same" : "✕ different"}
                </span>
              </div>
            </div>
          </div>
          <div class="proxmox-candidate-evidence-summary">
            <strong>Evidence: current IP address match only</strong>
            <span class="device-cell-muted">
              MAC differs{hasNameEvidence() ? " · additional name evidence also exists" : ""}
            </span>
            <Show when={addressEvidence()?.firstSeen || addressEvidence()?.lastSeen}>
              <span class="device-cell-muted">
                {formatEvidenceWindow(addressEvidence()?.firstSeen, addressEvidence()?.lastSeen)}
              </span>
            </Show>
            <div class="proxmox-ip-conflict-guidance">
              <i class="bi bi-info-circle" aria-hidden="true"></i>
              <span>
                This address overlap is not proof that both records are the same device. If you change the VM/LXC IP in Proxmox, import a fresh snapshot and this conflict disappears once the current addresses no longer overlap. Historical IP reuse is kept in Host history only.
              </span>
            </div>
          </div>
        </Show>
      </div>

      <div class="proxmox-candidate-actions">
        <button
          type="button"
          class="btn btn-sm btn-outline-primary"
          disabled={props.busy}
          onClick={props.onLink}
        >
          {props.busy ? "Working…" : isWeak() ? "Link anyway" : "Link"}
        </button>
        <Show when={isWeak()}>
          <button
            type="button"
            class="btn btn-sm btn-outline-secondary"
            disabled={props.busy}
            onClick={props.onReject}
          >
            {props.busy ? "Working…" : "Not this Host"}
          </button>
        </Show>
      </div>
    </div>
  );
}

function candidateEvidenceSummary(candidate: WorkloadMatchCandidate) {
  switch (candidate.assessment) {
    case "exact-mac": return "Exact MAC match";
    case "possible-ip-conflict": return "IP address matches, but MAC differs";
    case "address-only": return "IP address match only · No MAC confirmation";
    case "name-only": return "Name match only · No MAC confirmation";
    default: return candidateLabel(candidate.strength);
  }
}

function candidateAssessmentLabel(candidate: WorkloadMatchCandidate) {
  if (candidate.possibleIpConflict) return "IP conflict";
  switch (candidate.assessment) {
    case "exact-mac": return "Exact MAC";
    case "address-only": return "Address";
    case "name-only": return "Name";
    default: return candidateLabel(candidate.strength);
  }
}

function candidateAssessmentBadgeClass(candidate: WorkloadMatchCandidate) {
  if (candidate.possibleIpConflict) return "text-bg-warning";
  return candidateBadgeClass(candidate.strength);
}

function normalizeConfiguredAddress(value: string) {
  const trimmed = (value || "").trim();
  if (!trimmed) return "";
  if (trimmed.startsWith("[") && trimmed.includes("]")) {
    return trimmed.slice(1, trimmed.indexOf("]"));
  }
  const slash = trimmed.indexOf("/");
  return slash >= 0 ? trimmed.slice(0, slash) : trimmed;
}

function normalizeMac(value: string) {
  return (value || "").trim().replace(/-/g, ":").toUpperCase();
}

function formatEvidenceWindow(firstSeen?: string, lastSeen?: string) {
  if (firstSeen && lastSeen) {
    return "Observed "+formatTimestamp(firstSeen)+" → "+formatTimestamp(lastSeen);
  }
  if (lastSeen) return "Observed "+formatTimestamp(lastSeen);
  if (firstSeen) return "First observed "+formatTimestamp(firstSeen);
  return "";
}


function ProxmoxAPISection(props: { hostID: number; onApplied: () => void | Promise<void> }) {
  const [config, setConfig] = createSignal<ProxmoxAPIConfig | null>(null);
  const [baseUrl, setBaseUrl] = createSignal("");
  const [tokenId, setTokenId] = createSignal("");
  const [tokenSecret, setTokenSecret] = createSignal("");
  const [clearTokenSecret, setClearTokenSecret] = createSignal(false);
  const [verifyTls, setVerifyTls] = createSignal(true);
  const [timeoutSeconds, setTimeoutSeconds] = createSignal(10);
  const [dirty, setDirty] = createSignal(false);
  const [loadingConfig, setLoadingConfig] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [syncing, setSyncing] = createSignal(false);
  const [applyingAPI, setApplyingAPI] = createSignal(false);
  const [error, setError] = createSignal("");
  const [status, setStatus] = createSignal("");
  const [testResult, setTestResult] = createSignal<Awaited<ReturnType<typeof apiTestProxmoxAPIConnection>> | null>(null);
  const [apiSnapshot, setAPISnapshot] = createSignal<ProxmoxSnapshot | null>(null);
  const [apiPreview, setAPIPreview] = createSignal<ProxmoxImportPreview | null>(null);

  const applyConfig = (next: ProxmoxAPIConfig) => {
    setConfig(next);
    setBaseUrl(next.baseUrl || "");
    setTokenId(next.tokenId || "");
    setTokenSecret("");
    setClearTokenSecret(false);
    setVerifyTls(next.verifyTls);
    setTimeoutSeconds(next.timeoutSeconds || 10);
    setDirty(false);
  };

  const loadConfig = async (id: number) => {
    setLoadingConfig(true);
    setError("");
    try {
      applyConfig(await apiGetProxmoxAPIConfig(id));
    } catch (loadError) {
      setError(apiErrorMessage(loadError, "Proxmox API configuration could not be loaded."));
    } finally {
      setLoadingConfig(false);
    }
  };

  createEffect(() => {
    const id = props.hostID;
    setConfig(null);
    setError("");
    setStatus("");
    setTestResult(null);
    setAPISnapshot(null);
    setAPIPreview(null);
    if (id > 0) void loadConfig(id);
  });

  const markDirty = () => {
    setDirty(true);
    setTestResult(null);
    setAPISnapshot(null);
    setAPIPreview(null);
    setStatus("");
  };

  const saveConfig = async () => {
    if (props.hostID < 1 || saving()) return;
    setSaving(true);
    setError("");
    setStatus("");
    try {
      const patch = {
        baseUrl: baseUrl().trim(),
        tokenId: tokenId().trim(),
        verifyTls: verifyTls(),
        timeoutSeconds: timeoutSeconds(),
        ...(tokenSecret() ? { tokenSecret: tokenSecret() } : {}),
        ...(clearTokenSecret() ? { clearTokenSecret: true } : {}),
      };
      const next = await apiPatchProxmoxAPIConfig(props.hostID, patch);
      applyConfig(next);
      setStatus("API settings saved. The token secret remains write-only.");
    } catch (saveError) {
      setError(apiErrorMessage(saveError, "Proxmox API settings could not be saved."));
    } finally {
      setSaving(false);
    }
  };

  const disableIntegration = async () => {
    if (props.hostID < 1 || saving() || !config()?.enabled) return;
    setSaving(true);
    setError("");
    setStatus("");
    try {
      const next = await apiPatchProxmoxAPIConfig(props.hostID, { enabled: false });
      applyConfig(next);
      setStatus("Proxmox API inventory source disabled. Stored credentials were kept.");
    } catch (disableError) {
      setError(apiErrorMessage(disableError, "Proxmox API integration could not be disabled."));
    } finally {
      setSaving(false);
    }
  };

  const testConnection = async () => {
    if (props.hostID < 1 || testing()) return;
    if (dirty()) {
      setError("Save the API settings before testing the connection.");
      return;
    }
    setTesting(true);
    setError("");
    setStatus("");
    setTestResult(null);
    try {
      const result = await apiTestProxmoxAPIConnection(props.hostID);
      setTestResult(result);
      setStatus("Connection verified. No LANnventory inventory was changed.");
      applyConfig(await apiGetProxmoxAPIConfig(props.hostID));
    } catch (testError) {
      setError(apiErrorMessage(testError, "Proxmox API connection test failed."));
      try {
        applyConfig(await apiGetProxmoxAPIConfig(props.hostID));
      } catch {
        // Keep the connection error as the user-facing result.
      }
    } finally {
      setTesting(false);
    }
  };

  const syncNow = async () => {
    if (props.hostID < 1 || syncing()) return;
    if (dirty()) {
      setError("Save the API settings before syncing.");
      return;
    }
    setSyncing(true);
    setError("");
    setStatus("");
    setAPISnapshot(null);
    setAPIPreview(null);
    try {
      const result = await apiPreviewProxmoxAPISync(props.hostID);
      setAPISnapshot(result.snapshot);
      setAPIPreview(result.preview);
      setStatus(result.preview.applyAllowed
        ? (config()?.enabled
          ? "Sync preview ready. No inventory changes have been written yet."
          : "First sync preview ready. Review it and Apply to enable the API inventory source.")
        : "Sync preview is blocked. Review the reasons below; existing inventory is unchanged.");
      applyConfig(await apiGetProxmoxAPIConfig(props.hostID));
    } catch (syncError) {
      setError(apiErrorMessage(syncError, "Proxmox API sync failed. Existing inventory was preserved."));
      try {
        applyConfig(await apiGetProxmoxAPIConfig(props.hostID));
      } catch {
        // Keep the sync failure visible even if status reload also fails.
      }
    } finally {
      setSyncing(false);
    }
  };

  const applyAPIPreview = async () => {
    const currentPreview = apiPreview();
    const currentSnapshot = apiSnapshot();
    if (!currentPreview || !currentSnapshot || !currentPreview.applyAllowed || applyingAPI() || props.hostID < 1) return;
    const s = currentPreview.summary;
    const accepted = window.confirm(
      "Apply this reviewed Proxmox API snapshot to LANnventory?\n\n"+
      "New: "+s.added+" · Update: "+s.updated+" · Retire: "+s.retired+" · Unchanged: "+s.unchanged+
      "\n\nThis changes LANnventory inventory only. The integration has no VM/LXC control actions."
    );
    if (!accepted) return;

    setApplyingAPI(true);
    setError("");
    setStatus("");
    try {
      const result = await apiApplyProxmoxAPISync(props.hostID, currentPreview.previewToken, currentSnapshot);
      setAPISnapshot(null);
      setAPIPreview(null);
      setStatus("API snapshot applied at "+formatTimestamp(result.importedAt)+". The API inventory source is now enabled.");
      applyConfig(await apiGetProxmoxAPIConfig(props.hostID));
      await props.onApplied();
    } catch (applyError) {
      setError(apiErrorMessage(applyError, "Proxmox API snapshot could not be applied."));
    } finally {
      setApplyingAPI(false);
    }
  };

  return (
    <div class="proxmox-section proxmox-import-section">
      <div class="proxmox-section-heading">
        <div>
          <div class="small fw-semibold">Proxmox API</div>
          <div class="small device-cell-muted">
            Optional read-only source. The first Sync always previews changes; the integration becomes enabled only after you explicitly Apply the reviewed preview.
          </div>
        </div>
        <span class={"host-detail-section-badge"+(config()?.enabled ? " is-active" : "")}>
          {config()?.enabled ? "Enabled" : "Not enabled"}
        </span>
      </div>

      <Show when={!loadingConfig()} fallback={<div class="device-cell-muted">Loading API configuration…</div>}>
        <div class="proxmox-permission-note">
          <i class="bi bi-shield-lock" aria-hidden="true"></i>
          <div>
            <div class="fw-semibold">Use a dedicated read-only API token</div>
            <div>
              Prefer a privilege-separated token with read-only audit permissions. Do not use root credentials.
              LANnventory stores the token secret write-only and never returns it through this page.
            </div>
          </div>
        </div>

        <div class="row g-3 mt-1">
          <div class="col-12 col-lg-7">
            <label class="form-label small fw-semibold">Base URL</label>
            <input
              type="url"
              class="form-control form-control-sm wyl-control"
              placeholder="https://10.4.1.6:8006"
              value={baseUrl()}
              onInput={(event) => { setBaseUrl(event.currentTarget.value); markDirty(); }}
            />
            <div class="form-text">HTTPS only. Do not include <span class="font-monospace">/api2/json</span>.</div>
          </div>
          <div class="col-12 col-lg-5">
            <label class="form-label small fw-semibold">API Token ID</label>
            <input
              type="text"
              class="form-control form-control-sm wyl-control font-monospace"
              placeholder="lannventory@pve!inventory"
              value={tokenId()}
              onInput={(event) => { setTokenId(event.currentTarget.value); markDirty(); }}
            />
          </div>
          <div class="col-12 col-lg-7">
            <label class="form-label small fw-semibold">API Token Secret</label>
            <input
              type="password"
              autocomplete="new-password"
              class="form-control form-control-sm wyl-control font-monospace"
              placeholder={config()?.tokenSecretConfigured ? "Stored secret — leave blank to keep" : "Enter token secret"}
              disabled={clearTokenSecret()}
              value={tokenSecret()}
              onInput={(event) => { setTokenSecret(event.currentTarget.value); markDirty(); }}
            />
            <Show when={config()?.tokenSecretConfigured}>
              <div class="form-text">A secret is already stored. Leaving this blank keeps the existing value.</div>
            </Show>
          </div>
          <div class="col-12 col-lg-5">
            <label class="form-label small fw-semibold">Connection timeout</label>
            <div class="input-group input-group-sm">
              <input
                type="number"
                min="1"
                max="60"
                class="form-control wyl-control"
                value={timeoutSeconds()}
                onInput={(event) => { setTimeoutSeconds(Number(event.currentTarget.value) || 10); markDirty(); }}
              />
              <span class="input-group-text">seconds</span>
            </div>
          </div>
        </div>

        <div class="d-flex flex-wrap gap-3 align-items-center mt-3">
          <label class="form-check mb-0">
            <input
              class="form-check-input"
              type="checkbox"
              checked={verifyTls()}
              onChange={(event) => { setVerifyTls(event.currentTarget.checked); markDirty(); }}
            />
            <span class="form-check-label">Verify TLS certificate</span>
          </label>
          <Show when={config()?.tokenSecretConfigured}>
            <label class="form-check mb-0">
              <input
                class="form-check-input"
                type="checkbox"
                checked={clearTokenSecret()}
                onChange={(event) => { setClearTokenSecret(event.currentTarget.checked); markDirty(); }}
              />
              <span class="form-check-label">Clear stored token secret</span>
            </label>
          </Show>
        </div>

        <Show when={!verifyTls()}>
          <div class="alert alert-warning py-2 px-3 mt-3 mb-0 small">
            <i class="bi bi-exclamation-triangle-fill me-1" aria-hidden="true"></i>
            TLS certificate verification is disabled explicitly. LANnventory will not disable it automatically or silently.
          </div>
        </Show>

        <div class="d-flex flex-wrap gap-2 align-items-center mt-3">
          <button type="button" class="btn btn-sm btn-outline-secondary" disabled={saving()} onClick={() => void saveConfig()}>
            <i class={saving() ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-floppy me-1"} aria-hidden="true"></i>
            {saving() ? "Saving…" : "Save"}
          </button>
          <button type="button" class="btn btn-sm btn-outline-primary" disabled={testing() || dirty() || !config()?.tokenSecretConfigured} onClick={() => void testConnection()}>
            <i class={testing() ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-plug me-1"} aria-hidden="true"></i>
            {testing() ? "Testing…" : "Test connection"}
          </button>
          <button type="button" class="btn btn-sm btn-primary" disabled={syncing() || dirty() || !config()?.tokenSecretConfigured} onClick={() => void syncNow()}>
            <i class={syncing() ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-arrow-repeat me-1"} aria-hidden="true"></i>
            {syncing() ? "Collecting…" : "Sync now"}
          </button>
          <Show when={config()?.enabled}>
            <button type="button" class="btn btn-sm btn-outline-danger" disabled={saving() || dirty()} onClick={() => void disableIntegration()}>
              <i class="bi bi-pause-circle me-1" aria-hidden="true"></i>
              Disable API source
            </button>
          </Show>
          <Show when={dirty()}><span class="small text-warning">Unsaved changes</span></Show>
        </div>

        <Show when={config()}>
          {(current) => (
            <div class="proxmox-source-grid mt-3">
              <SourceMetric label="Status" value={capitalize(current().status || "not-configured")} />
              <SourceMetric label="Last attempt" value={current().lastAttemptAt ? formatTimestamp(current().lastAttemptAt!) : "Never"} />
              <SourceMetric label="Last successful sync" value={current().lastSuccessfulSync ? formatTimestamp(current().lastSuccessfulSync!) : "Never"} />
              <SourceMetric label="TLS verification" value={current().verifyTls ? "On" : "Off"} />
            </div>
          )}
        </Show>

        <Show when={testResult()}>
          {(result) => (
            <div class="proxmox-preview mt-3">
              <div class="proxmox-preview-header">
                <div>
                  <div class="fw-semibold">Connection test</div>
                  <div class="small device-cell-muted">{result().result.pveVersion}</div>
                </div>
                <span class="badge text-bg-success">Connected</span>
              </div>
              <div class="proxmox-preview-summary">
                <PreviewMetric label="Nodes" value={result().result.nodeCount} />
                <PreviewMetric label="VMs" value={result().result.vmCount} />
                <PreviewMetric label="LXCs" value={result().result.lxcCount} />
                <div><strong>{result().result.vmInventory ? "OK" : "No"}</strong><span>VM access</span></div>
                <div><strong>{result().result.lxcInventory ? "OK" : "No"}</strong><span>LXC access</span></div>
              </div>
            </div>
          )}
        </Show>

        <Show when={error()}>
          <div class="host-inline-error mt-3" role="alert">{error()}</div>
        </Show>
        <Show when={status()}>
          <div class="small text-success mt-2">{status()}</div>
        </Show>

        <Show when={apiPreview()}>
          {(preview) => (
            <div class="proxmox-preview">
              <div class="proxmox-preview-header">
                <div>
                  <div class="fw-semibold">API sync preview</div>
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

              <ApplyImpact preview={preview()} />

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
                    <thead><tr><th>Action</th><th>Workload</th><th>What changes in LANnventory</th></tr></thead>
                    <tbody>
                      <For each={preview().workloads}>
                        {(item) => (
                          <tr>
                            <td><span class={"badge "+previewActionClass(item.action)}>{capitalize(item.action)}</span></td>
                            <td>
                              <span class="font-monospace me-2">{item.after?.nativeId ?? item.before?.nativeId ?? item.key}</span>
                              <span>{item.after?.name ?? item.before?.name ?? ""}</span>
                            </td>
                            <td>{previewChangeDescription(item.action, item.changes)}</td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>

              <div class="proxmox-preview-footer">
                <div class="small device-cell-muted">
                  Apply revalidates this Snapshot and its stale-preview token before writing anything.
                  {!config()?.enabled ? " The first successful Apply also enables this API inventory source." : ""}
                </div>
                <button
                  type="button"
                  class="btn btn-sm btn-success"
                  disabled={!preview().applyAllowed || applyingAPI()}
                  onClick={() => void applyAPIPreview()}
                >
                  <i class={applyingAPI() ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-check2-circle me-1"} aria-hidden="true"></i>
                  {applyingAPI() ? "Applying…" : "Apply to LANnventory"}
                </button>
              </div>
            </div>
          )}
        </Show>
      </Show>
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
  proxmoxAddress: string;
}) {
  const [selectedCommand, setSelectedCommand] = createSignal("");
  let collectCommandRef: HTMLElement | undefined;
  let scpCommandRef: HTMLElement | undefined;
  let catCommandRef: HTMLElement | undefined;

  const collectorURL = () => {
    if (typeof window === "undefined") return "/lannventory-proxmox-collector.py";
    return window.location.origin+"/lannventory-proxmox-collector.py";
  };
  const snapshotPath = "/tmp/lannventory-proxmox.json";
  const collectorCommand = () =>
    "curl -fsSL "+collectorURL()+" -o /tmp/lannventory-proxmox-collector.py && python3 /tmp/lannventory-proxmox-collector.py --compact > "+snapshotPath+" && echo 'Snapshot saved to "+snapshotPath+"'";
  const scpCommand = () => {
    const address = props.proxmoxAddress?.trim() || "<PROXMOX-IP>";
    return "scp root@"+address+":"+snapshotPath+" .";
  };
  const catCommand = () => "cat "+snapshotPath;

  const selectCommand = (element: HTMLElement | undefined, key: string) => {
    if (!element || typeof window === "undefined") return;
    const selection = window.getSelection();
    if (!selection) return;
    const range = document.createRange();
    range.selectNodeContents(element);
    selection.removeAllRanges();
    selection.addRange(range);
    setSelectedCommand(key);
    window.setTimeout(() => {
      if (selectedCommand() === key) setSelectedCommand("");
    }, 3500);
  };

  return (
    <div class="proxmox-section proxmox-import-section">
      <div class="proxmox-section-heading">
        <div>
          <div class="small fw-semibold">Import from Proxmox</div>
          <div class="small device-cell-muted">
            Guided read-only import. LANnventory does not need your Proxmox password or API token for this method.
          </div>
        </div>
        <span class="host-detail-section-badge">No credentials</span>
      </div>

      <div class="proxmox-permission-note">
        <i class="bi bi-shield-check" aria-hidden="true"></i>
        <div>
          <div class="fw-semibold">How access works</div>
          <div>
            The collector runs <strong>on the Proxmox node</strong> and uses the permissions of the shell user running it.
            Use <strong>root</strong> or an account allowed to run <span class="font-monospace">qm</span>/<span class="font-monospace">pct</span>
            and read the allowlisted network lines under <span class="font-monospace">/etc/pve</span>.
            It does not log in to Proxmox from LANnventory and it does not send collected data anywhere.
          </div>
        </div>
      </div>

      <div class="proxmox-import-wizard">
        <WizardStep number="1" title="Open the Proxmox shell">
          <div class="small device-cell-muted">
            In the Proxmox web UI open <strong>Shell</strong> for this node, preferably as root.
          </div>
        </WizardStep>

        <WizardStep number="2" title="Create a compact snapshot file">
          <div class="small device-cell-muted mb-2">
            Run this on the Proxmox node. The collector output is written directly to <span class="font-monospace">{snapshotPath}</span>, so large environments do not flood the terminal with JSON.
          </div>
          <CommandBox
            command={collectorCommand()}
            selected={selectedCommand() === "collect"}
            onSelect={(element) => {
              collectCommandRef = element;
              selectCommand(collectCommandRef, "collect");
            }}
          />
          <div class="proxmox-command-security-note">
            <i class="bi bi-shield-lock me-1" aria-hidden="true"></i>
            LANnventory does not copy terminal commands to your clipboard automatically. Select the command, press <strong>Ctrl+C</strong>, then paste it into the Proxmox shell.
          </div>
          <div class="proxmox-download-fallback">
            <span class="small device-cell-muted">If the Proxmox node cannot reach this LANnventory URL:</span>
            <a
              class="btn btn-sm btn-outline-secondary"
              href="/lannventory-proxmox-collector.py"
              download="lannventory-proxmox-collector.py"
            >
              <i class="bi bi-download me-1" aria-hidden="true"></i>
              Download collector
            </a>
          </div>
        </WizardStep>

        <WizardStep number="3" title="Bring the snapshot file to this browser">
          <div class="small device-cell-muted mb-2">
            <strong>Recommended for larger environments:</strong> copy the JSON file from Proxmox to the computer where this browser is running, then use <strong>Choose JSON file</strong>.
          </div>
          <div class="proxmox-transfer-option">
            <div class="small fw-semibold">Option A · Copy the file with SCP from your computer</div>
            <CommandBox
              command={scpCommand()}
              selected={selectedCommand() === "scp"}
              onSelect={(element) => {
                scpCommandRef = element;
                selectCommand(scpCommandRef, "scp");
              }}
            />
            <div class="small device-cell-muted mt-1">
              Run this in a terminal on your computer, not in the Proxmox shell. If SSH uses a different user, address, or port, adjust the command.
            </div>
          </div>
          <div class="proxmox-transfer-option">
            <div class="small fw-semibold">Option B · Paste JSON manually</div>
            <div class="small device-cell-muted mb-1">
              For smaller setups you can print the compact file and copy the single JSON line.
            </div>
            <CommandBox
              command={catCommand()}
              selected={selectedCommand() === "cat"}
              onSelect={(element) => {
                catCommandRef = element;
                selectCommand(catCommandRef, "cat");
              }}
            />
          </div>
          <textarea
            class="form-control form-control-sm wyl-control proxmox-import-textarea font-monospace mt-2"
            rows={7}
            placeholder={'Paste compact collector JSON here, starting with "{" …'}
            value={props.importText}
            onInput={(event) => props.onText(event.currentTarget.value)}
          ></textarea>
          <div class="proxmox-import-actions mt-2">
            <label class="btn btn-sm btn-outline-secondary mb-0">
              <i class="bi bi-file-earmark-arrow-up me-1" aria-hidden="true"></i>
              Choose JSON file
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
          </div>
        </WizardStep>

        <WizardStep number="4" title="Review before anything is saved">
          <div class="proxmox-review-row">
            <div class="small device-cell-muted">
              Preview validates the snapshot and shows exactly what LANnventory would add, update, keep unchanged, or mark retired. Nothing is written until you review and confirm the import.
            </div>
            <button
              type="button"
              class="btn btn-sm btn-primary"
              disabled={!props.importText.trim() || props.previewing || props.applying}
              onClick={props.onPreview}
            >
              <i class={props.previewing ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-search me-1"} aria-hidden="true"></i>
              {props.previewing ? "Validating…" : "Preview changes"}
            </button>
          </div>
        </WizardStep>
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
                {preview().applyAllowed ? "Ready to import" : "Blocked"}
              </span>
            </div>

            <div class="proxmox-preview-summary">
              <PreviewMetric label="Add" value={preview().summary.added} />
              <PreviewMetric label="Update" value={preview().summary.updated} />
              <PreviewMetric label="Unchanged" value={preview().summary.unchanged} />
              <PreviewMetric label="Retire" value={preview().summary.retired} />
              <PreviewMetric label="Conflicts" value={preview().summary.conflicts} />
            </div>

            <ApplyImpact preview={preview()} />

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
                    <tr><th>Action</th><th>Workload</th><th>What changes in LANnventory</th></tr>
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
                          <td>{previewChangeDescription(item.action, item.changes)}</td>
                        </tr>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </Show>

            <div class="proxmox-preview-footer">
              <div class="small device-cell-muted">
                Import revalidates the snapshot and rejects a stale Preview if inventory changed meanwhile.
              </div>
              <button
                type="button"
                class="btn btn-sm btn-success"
                disabled={!preview().applyAllowed || props.applying}
                onClick={props.onApply}
              >
                <i class={props.applying ? "bi bi-arrow-repeat proxmox-spin me-1" : "bi bi-check2-circle me-1"} aria-hidden="true"></i>
                {props.applying ? "Importing…" : "Import into LANnventory"}
              </button>
            </div>
          </div>
        )}
      </Show>
    </div>
  );
}

function CommandBox(props: {
  command: string;
  selected: boolean;
  onSelect: (element: HTMLElement) => void;
}) {
  let commandRef: HTMLElement | undefined;
  return (
    <div class="proxmox-command-row">
      <code ref={commandRef} class="proxmox-collector-command">{props.command}</code>
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary proxmox-copy-button"
        onClick={() => commandRef && props.onSelect(commandRef)}
      >
        <i class={props.selected ? "bi bi-check2 me-1" : "bi bi-text-paragraph me-1"} aria-hidden="true"></i>
        {props.selected ? "Selected — Ctrl+C" : "Select command"}
      </button>
    </div>
  );
}

function ApplyImpact(props: { preview: ProxmoxImportPreview }) {
  const s = () => props.preview.summary;
  return (
    <div class="proxmox-apply-impact">
      <div class="proxmox-apply-impact-title">
        <i class="bi bi-info-circle-fill" aria-hidden="true"></i>
        What Import will do in LANnventory
      </div>
      <div class="proxmox-apply-impact-grid">
        <div>
          <strong>{s().added}</strong>
          <span>new workload record{s().added === 1 ? "" : "s"} will be added under this Proxmox Host.</span>
        </div>
        <div>
          <strong>{s().updated}</strong>
          <span>existing workload record{s().updated === 1 ? "" : "s"} will be updated.</span>
        </div>
        <div>
          <strong>{s().retired}</strong>
          <span>previously imported workload{s().retired === 1 ? "" : "s"} will be marked retired, not deleted.</span>
        </div>
      </div>
      <div class="proxmox-apply-impact-safety">
        <div><i class="bi bi-check2-circle" aria-hidden="true"></i> Workloads remain separate from LANnventory Hosts; no fake Hosts are created.</div>
        <div><i class="bi bi-check2-circle" aria-hidden="true"></i> Only a unique exact-MAC match may auto-link to an existing Host; weaker matches still require your choice.</div>
        <div><i class="bi bi-shield-check" aria-hidden="true"></i> No VM/LXC is created, changed, started, stopped, or deleted on Proxmox.</div>
        <div><i class="bi bi-shield-check" aria-hidden="true"></i> Managed Device Profile fields and manual workload links are not overwritten.</div>
      </div>
    </div>
  );
}

function previewChangeDescription(action: string, changes: string[]) {
  if (action === "add") return "Create a new workload inventory record.";
  if (action === "retire") return "Mark the imported workload as retired; keep its history.";
  if (action === "conflict") return "Blocked by a manual/imported ownership conflict.";
  if (action === "unchanged") return "No inventory fields will change.";
  if (changes.length === 0) return "Update the existing workload record.";
  return "Update: "+changes.join(", ")+".";
}

function WizardStep(props: { number: string; title: string; children: any }) {
  return (
    <section class="proxmox-wizard-step">
      <div class="proxmox-wizard-step-number" aria-hidden="true">{props.number}</div>
      <div class="proxmox-wizard-step-body">
        <div class="proxmox-wizard-step-title">{props.title}</div>
        <div>{props.children}</div>
      </div>
    </section>
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

function newestProxmoxSource(
  scriptState: Awaited<ReturnType<typeof apiGetProxmoxSourceState>>,
  apiState: Awaited<ReturnType<typeof apiGetProxmoxSourceState>>,
) {
  if (!scriptState) return apiState;
  if (!apiState) return scriptState;
  const scriptTime = Date.parse(scriptState.importedAt || scriptState.collectedAt || "") || 0;
  const apiTime = Date.parse(apiState.importedAt || apiState.collectedAt || "") || 0;
  return apiTime >= scriptTime ? apiState : scriptState;
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
