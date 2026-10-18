import { createEffect, createMemo, createSignal, For, onCleanup, Show } from "solid-js";
import {
  apiCreateManualRelationship,
  apiDeleteManualRelationship,
  apiGetRelationshipGraph,
  apiGetRelationshipTypes,
  apiPatchManualRelationshipNote,
  apiSearchRelationshipTargets,
  type RelationshipDefinition,
  type RelationshipGraph,
  type RelationshipGraphEdge,
  type RelationshipGraphNode,
  type RelationshipHostTarget,
} from "../../functions/api";
import type { Host } from "../../functions/exports";

type Props = { host: Host; active: boolean };
type ManualType = "connected_to" | "manages" | "depends_on";
const defaultLimit = 100;
const expandedLimit = 500;
const noteMaxLength = 500;

function RelationshipsCard(props: Props) {
  const [graph, setGraph] = createSignal<RelationshipGraph | null>(null);
  const [graphLoading, setGraphLoading] = createSignal(false);
  const [graphError, setGraphError] = createSignal("");
  const [graphLimit, setGraphLimit] = createSignal(defaultLimit);
  const [definitions, setDefinitions] = createSignal<RelationshipDefinition[]>([]);
  const [registryError, setRegistryError] = createSignal("");
  const [adding, setAdding] = createSignal(false);
  const [relationshipType, setRelationshipType] = createSignal<ManualType>("connected_to");
  const [reverse, setReverse] = createSignal(false);
  const [targetQuery, setTargetQuery] = createSignal("");
  const [selectedTarget, setSelectedTarget] = createSignal<RelationshipHostTarget | null>(null);
  const [targetResults, setTargetResults] = createSignal<RelationshipHostTarget[]>([]);
  const [searching, setSearching] = createSignal(false);
  const [targetError, setTargetError] = createSignal("");
  const [hasSearched, setHasSearched] = createSignal(false);
  const [newNote, setNewNote] = createSignal("");
  const [editingID, setEditingID] = createSignal("");
  const [editingNote, setEditingNote] = createSignal("");
  const [saving, setSaving] = createSignal(false);
  const [actionError, setActionError] = createSignal("");
  const [actionStatus, setActionStatus] = createSignal("");
  let graphController: AbortController | undefined;
  let graphRequestID = 0;
  let contextVersion = 0;

  const manualTypes = createMemo(() =>
    definitions().filter((item) =>
      item.manualAllowed && item.sourceTypes.includes("host") && item.targetTypes.includes("host") &&
      (item.type === "connected_to" || item.type === "manages" || item.type === "depends_on"),
    ),
  );
  const nodes = createMemo(() => new Map<string, RelationshipGraphNode>((graph()?.nodes ?? []).map((node): [string, RelationshipGraphNode] => [node.id, node])));
  const getOther = (edge: RelationshipGraphEdge): RelationshipGraphNode | undefined => {
    const isSource = edge.source.entityType === "host" && edge.source.entityId === props.host.ID;
    const endpoint = isSource ? edge.target : edge.source;
    return nodes().get(endpoint.entityType + ":" + endpoint.entityId);
  };
  const relationLabel = (edge: RelationshipGraphEdge) =>
    edge.source.entityType === "host" && edge.source.entityId === props.host.ID
      ? edge.sourceLabel : edge.inverseLabel;
  const canManage = (edge: RelationshipGraphEdge) =>
    edge.owner === "relationship_assertions" && edge.provenance === "manual" &&
    !edge.readOnly && edge.id.startsWith("manual:");

  const resetDraft = () => {
    setAdding(false);
    setReverse(false);
    setRelationshipType("connected_to");
    setTargetQuery("");
    setSelectedTarget(null);
    setTargetResults([]);
    setTargetError("");
    setHasSearched(false);
    setNewNote("");
    setEditingID("");
    setEditingNote("");
    setActionError("");
  };

  const loadGraph = async (hostID: number, limit: number) => {
    graphController?.abort();
    const controller = new AbortController();
    graphController = controller;
    const request = ++graphRequestID;
    setGraphLoading(true);
    setGraphError("");
    try {
      const next = await apiGetRelationshipGraph(hostID, limit, controller.signal);
      if (controller.signal.aborted || request !== graphRequestID || props.host.ID !== hostID || !props.active) return;
      setGraph(next);
    } catch {
      if (controller.signal.aborted || request !== graphRequestID || props.host.ID !== hostID || !props.active) return;
      setGraphError("Relationships could not be loaded. Try again.");
    } finally {
      if (request === graphRequestID) setGraphLoading(false);
    }
  };

  const retryRegistry = () => {
    const hostID = props.host.ID;
    const version = contextVersion;
    if (hostID < 1 || !props.active) return;
    setRegistryError("");
    void apiGetRelationshipTypes()
      .then((items) => {
        if (version === contextVersion && hostID === props.host.ID && props.active) setDefinitions(items);
      })
      .catch(() => {
        if (version === contextVersion && hostID === props.host.ID && props.active) setRegistryError("Manual relationship types are unavailable.");
      });
  };

  createEffect(() => {
    const hostID = props.host.ID;
    const active = props.active;
    const version = ++contextVersion;
    graphController?.abort();
    graphRequestID++;
    setGraph(null);
    setGraphLoading(false);
    setGraphError("");
    setGraphLimit(defaultLimit);
    setSaving(false);
    setActionStatus("");
    setRegistryError("");
    setDefinitions([]);
    resetDraft();
    if (!active || hostID < 1) return;
    const registryController = new AbortController();
    void apiGetRelationshipTypes(registryController.signal)
      .then((items) => {
        if (registryController.signal.aborted || version !== contextVersion) return;
        setDefinitions(items);
      })
      .catch(() => {
        if (!registryController.signal.aborted && version === contextVersion) {
          setRegistryError("Manual relationship types are unavailable.");
        }
      });
    void loadGraph(hostID, defaultLimit);
    onCleanup(() => {
      registryController.abort();
      graphController?.abort();
      graphRequestID++;
    });
  });

  createEffect(() => {
    const query = targetQuery().trim();
    const open = adding();
    const currentID = props.host.ID;
    const active = props.active;
    const selected = selectedTarget();
    setSearching(false);
    setHasSearched(false);
    if (!active || !open || selected || query.length < 2 || query.length > 100) {
      setTargetResults([]);
      setTargetError("");
      return;
    }
    let controller: AbortController | undefined;
    const timer = window.setTimeout(() => {
      controller = new AbortController();
      setSearching(true);
      setTargetError("");
      void apiSearchRelationshipTargets(query, currentID, controller.signal)
        .then((items) => {
          if (!controller?.signal.aborted && currentID === props.host.ID && props.active) {
            setTargetResults(items);
            setHasSearched(true);
          }
        })
        .catch(() => {
          if (!controller?.signal.aborted) {
            setTargetResults([]);
            setHasSearched(true);
            setTargetError("Host search failed. Try again.");
          }
        })
        .finally(() => {
          if (!controller?.signal.aborted) setSearching(false);
        });
    }, 250);
    onCleanup(() => {
      window.clearTimeout(timer);
      controller?.abort();
    });
  });

  const refresh = () => void loadGraph(props.host.ID, graphLimit());

  const createRelationship = async () => {
    const chosen = selectedTarget();
    const hostID = props.host.ID;
    if (saving() || !chosen || hostID < 1 || chosen.id === hostID ||
      !manualTypes().some((item) => item.type === relationshipType())) {
      setActionError("Choose a valid relationship type and a different Host from the search results.");
      return;
    }
    if (Array.from(newNote()).length > noteMaxLength) {
      setActionError("Note must be at most 500 characters.");
      return;
    }
    const version = contextVersion;
    const sourceID = reverse() ? chosen.id : hostID;
    const targetID = reverse() ? hostID : chosen.id;
    setSaving(true);
    setActionError("");
    setActionStatus("");
    try {
      await apiCreateManualRelationship({
        type: relationshipType(),
        source: { entityType: "host", entityId: sourceID },
        target: { entityType: "host", entityId: targetID },
        note: newNote(),
      });
      if (version !== contextVersion) return;
      resetDraft();
      setActionStatus("Manual relationship saved.");
      refresh();
    } catch (error) {
      if (version !== contextVersion) return;
      setActionError(apiRelationshipError(error, "Relationship could not be saved."));
    } finally {
      if (version === contextVersion) setSaving(false);
    }
  };

  const saveNote = async (edge: RelationshipGraphEdge) => {
    const id = manualEdgeID(edge);
    if (saving() || !canManage(edge) || id === null) return;
    if (Array.from(editingNote()).length > noteMaxLength) {
      setActionError("Note must be at most 500 characters.");
      return;
    }
    const version = contextVersion;
    setSaving(true);
    setActionError("");
    setActionStatus("");
    try {
      await apiPatchManualRelationshipNote(id, editingNote());
      if (version !== contextVersion) return;
      setEditingID("");
      setActionStatus("Relationship note updated.");
      refresh();
    } catch (error) {
      if (version !== contextVersion) return;
      setActionError(apiRelationshipError(error, "Note could not be saved."));
    } finally {
      if (version === contextVersion) setSaving(false);
    }
  };

  const deleteRelationship = async (edge: RelationshipGraphEdge) => {
    const id = manualEdgeID(edge);
    if (saving() || !canManage(edge) || id === null) return;
    if (!window.confirm("Delete this manual relationship? This cannot be undone.")) return;
    const version = contextVersion;
    setSaving(true);
    setActionError("");
    setActionStatus("");
    try {
      await apiDeleteManualRelationship(id);
      if (version !== contextVersion) return;
      setEditingID("");
      setActionStatus("Manual relationship removed.");
      refresh();
    } catch (error) {
      if (version !== contextVersion) return;
      setActionError(apiRelationshipError(error, "Relationship could not be removed."));
    } finally {
      if (version === contextVersion) setSaving(false);
    }
  };

  const toggleAdd = () => {
    if (adding()) {
      resetDraft();
    } else {
      setEditingID("");
      setActionError("");
      setActionStatus("");
      setAdding(true);
    }
  };

  onCleanup(() => {
    contextVersion++;
    graphController?.abort();
    graphRequestID++;
  });

  return (
    <section class="card wyl-panel host-panel host-relationships-panel" aria-labelledby="host-relationships-title">
      <div class="card-header host-panel-header">
        <div>
          <div id="host-relationships-title" class="host-panel-title">Relationships</div>
          <div class="host-panel-subtitle">
            Manual connections and read-only infrastructure relationships. No physical topology is inferred.
          </div>
        </div>
        <button
          type="button"
          class="btn btn-sm btn-outline-primary"
          disabled={props.host.ID < 1 || saving() || graphLoading() || definitions().length === 0}
          aria-expanded={adding()}
          onClick={toggleAdd}
        >
          <i class={"bi " + (adding() ? "bi-x-lg" : "bi-plus-lg") + " me-1"} aria-hidden="true"></i>
          {adding() ? "Cancel" : "Add relationship"}
        </button>
      </div>
      <div class="card-body">
        <Show when={registryError()}>
          <div class="host-inline-error mb-2" role="alert">
            {registryError()} <button type="button" class="btn btn-sm btn-outline-secondary ms-2" onClick={retryRegistry}>Retry</button>
          </div>
        </Show>
        <Show when={adding()}>
          <div class="host-relationship-editor" aria-label="Create manual relationship">
            <div class="row g-2">
              <div class="col-12 col-md-5">
                <label class="form-label small fw-semibold" for="host-relationship-type">Type</label>
                <select id="host-relationship-type" class="form-select form-select-sm wyl-control" value={relationshipType()} disabled={saving()} onChange={(event) => setRelationshipType(event.currentTarget.value as ManualType)}>
                  <For each={manualTypes()}>{(item) => <option value={item.type}>{item.sourceLabel} ({item.type})</option>}</For>
                </select>
              </div>
              <div class="col-12 col-md-7">
                <label class="form-label small fw-semibold" for="host-relationship-direction">Direction</label>
                <select id="host-relationship-direction" class="form-select form-select-sm wyl-control" value={reverse() ? "incoming" : "outgoing"} disabled={saving()} onChange={(event) => setReverse(event.currentTarget.value === "incoming")}>
                  <option value="outgoing">This Host → selected Host</option>
                  <option value="incoming">Selected Host → this Host</option>
                </select>
              </div>
              <div class="col-12">
                <label class="form-label small fw-semibold" for="host-relationship-target">Other Host</label>
                <input
                  id="host-relationship-target"
                  class="form-control form-control-sm wyl-control"
                  type="search"
                  autocomplete="off"
                  value={targetQuery()}
                  disabled={saving()}
                  placeholder="Search name, DNS, IP or MAC (2+ characters)"
                  onInput={(event) => {
                    setTargetQuery(event.currentTarget.value);
                    setSelectedTarget(null);
                    setHasSearched(false);
                    setActionError("");
                  }}
                />
                <Show when={selectedTarget()}>{(selected) =>
                  <div class="small text-success mt-1"><i class="bi bi-check-circle me-1" aria-hidden="true"></i>Selected: {selected().name || selected().ip || selected().mac} (Host #{selected().id})</div>
                }</Show>
                <Show when={searching()}><div class="small device-cell-muted mt-1" role="status">Searching current Hosts…</div></Show>
                <Show when={targetError()}><div class="host-inline-error mt-1" role="alert">{targetError()}</div></Show>
                <Show when={!selectedTarget() && !searching() && hasSearched() && targetQuery().trim().length >= 2 && targetResults().length === 0 && !targetError()}>
                  <div class="small device-cell-muted mt-1">No matching current Hosts.</div>
                </Show>
                <Show when={!selectedTarget() && targetResults().length > 0}>
                  <div class="list-group host-relationship-targets mt-1" aria-label="Matching current Hosts">
                    <For each={targetResults()}>{(item) =>
                      <button type="button" class="list-group-item list-group-item-action py-2" disabled={saving()} onClick={() => {
                        setSelectedTarget(item);
                        setTargetQuery(item.name || item.ip || item.mac);
                        setTargetResults([]);
                      }}>
                        <span class="fw-semibold">{item.name || item.ip || item.mac}</span>
                        <span class="small device-cell-muted ms-2">{item.ip || item.mac} · Host #{item.id}</span>
                      </button>
                    }</For>
                  </div>
                </Show>
              </div>
              <div class="col-12">
                <label class="form-label small fw-semibold" for="host-relationship-note">Note (optional)</label>
                <textarea id="host-relationship-note" class="form-control form-control-sm wyl-control" rows={2} maxLength={noteMaxLength} value={newNote()} disabled={saving()} onInput={(event) => setNewNote(event.currentTarget.value)} />
              </div>
              <div class="col-12 d-flex align-items-center gap-2 flex-wrap">
                <button type="button" class="btn btn-sm btn-primary" disabled={saving() || !selectedTarget() || manualTypes().length === 0} onClick={() => void createRelationship()}>
                  {saving() ? "Saving…" : "Save relationship"}
                </button>
                <button type="button" class="btn btn-sm btn-outline-secondary" disabled={saving()} onClick={resetDraft}>Cancel</button>
                <span class="small device-cell-muted">Manual assertions never modify Proxmox ownership or matches.</span>
              </div>
            </div>
          </div>
        </Show>
        <Show when={actionError()}><div class="host-inline-error mb-2" role="alert">{actionError()}</div></Show>
        <Show when={actionStatus()}><div class="small text-success mb-2" role="status">{actionStatus()}</div></Show>
        <Show when={graphLoading() && graph() === null}>
          <div class="device-cell-muted" role="status">Loading relationships…</div>
        </Show>
        <Show when={graphError()}>
          <div class="host-inline-error" role="alert">
            {graphError()}
            <button type="button" class="btn btn-sm btn-outline-secondary ms-2" disabled={graphLoading()} onClick={refresh}>Retry</button>
          </div>
        </Show>
        <Show when={!graphLoading() && !graphError() && graph()?.edges.length === 0}>
          <div class="profile-empty-state">
            <i class="bi bi-diagram-3" aria-hidden="true"></i>
            <div>
              <div class="fw-semibold">No relationships recorded</div>
              <div class="small device-cell-muted">Add a manual Host relationship or connect Proxmox inventory to see source-managed links.</div>
            </div>
          </div>
        </Show>
        <Show when={graph()?.edges.length}>
          <div class="host-relationships-list">
            <For each={graph()?.edges ?? []}>{(edge) => (
              <div class="host-relationship-row">
                <div class="host-relationship-main">
                  <span class="host-relationship-direction">{relationLabel(edge)}</span>
                  <Show when={getOther(edge)}>{(node) =>
                    <span class="host-relationship-entity">
                      <Show when={node().entityType === "host"} fallback={
                        <span><span class="badge text-bg-secondary me-1">{node().workloadType === "container" ? "LXC" : "VM"} {node().nativeId}</span>{node().label}</span>
                      }>
                        <a href={"/host/" + node().entityId}>{node().label || "Host #" + node().entityId}</a>
                      </Show>
                    </span>
                  }</Show>
                  <div class="host-relationship-meta">
                    <span class={"badge " + (canManage(edge) ? "text-bg-primary" : "text-bg-secondary")}>
                      {canManage(edge) ? "Manual" : "Read-only"}
                    </span>
                    <span>{sourceDescription(edge)}</span>
                    <Show when={edge.retiredAt}><span class="badge text-bg-secondary">Retired</span></Show>
                    <Show when={edge.note}><span title={edge.note} class="text-break">{edge.note}</span></Show>
                  </div>
                </div>
                <Show when={canManage(edge)}>
                  <div class="host-relationship-actions">
                    <button type="button" class="btn btn-sm btn-outline-secondary" disabled={saving()} aria-label={"Edit note for " + edge.type} onClick={() => {
                      setAdding(false);
                      setActionError("");
                      setEditingID(edge.id);
                      setEditingNote(edge.note || "");
                    }}>Edit note</button>
                    <button type="button" class="btn btn-sm btn-outline-danger" disabled={saving()} aria-label={"Delete " + edge.type + " relationship"} onClick={() => void deleteRelationship(edge)}>Delete</button>
                  </div>
                </Show>
                <Show when={editingID() === edge.id && canManage(edge)}>
                  <div class="w-100">
                    <label class="form-label small fw-semibold" for={"host-relationship-edit-" + edge.id}>Manual note</label>
                    <textarea id={"host-relationship-edit-" + edge.id} class="form-control form-control-sm wyl-control mb-2" rows={2} maxLength={noteMaxLength} value={editingNote()} disabled={saving()} onInput={(event) => setEditingNote(event.currentTarget.value)} />
                    <div class="d-flex gap-2">
                      <button type="button" class="btn btn-sm btn-primary" disabled={saving()} onClick={() => void saveNote(edge)}>Save note</button>
                      <button type="button" class="btn btn-sm btn-outline-secondary" disabled={saving()} onClick={() => setEditingID("")}>Cancel</button>
                    </div>
                  </div>
                </Show>
              </div>
            )}</For>
          </div>
        </Show>
        <Show when={graph()?.truncated}>
          <div class="small device-cell-muted mt-2" role="status">
            Results are limited; additional relationships may exist.
            <Show when={graphLimit() < expandedLimit}>
              <button type="button" class="btn btn-sm btn-link" disabled={graphLoading()} onClick={() => {
                setGraphLimit(expandedLimit);
                void loadGraph(props.host.ID, expandedLimit);
              }}>Show up to 500</button>
            </Show>
          </div>
        </Show>
        <Show when={graph() && !graphError()}>
          <div class="mt-2">
            <button type="button" class="btn btn-sm btn-outline-secondary" disabled={graphLoading() || saving()} onClick={refresh}>
              <i class="bi bi-arrow-clockwise me-1" aria-hidden="true"></i>
              {graphLoading() ? "Refreshing…" : "Refresh relationships"}
            </button>
          </div>
        </Show>
      </div>
    </section>
  );
}

function manualEdgeID(edge: RelationshipGraphEdge): number | null {
  if (!edge.id.startsWith("manual:")) return null;
  const id = Number(edge.id.slice("manual:".length));
  return Number.isSafeInteger(id) && id > 0 ? id : null;
}

function sourceDescription(edge: RelationshipGraphEdge): string {
  if (edge.owner === "relationship_assertions") {
    return edge.provenance === "manual" ? "User-maintained" : "Imported assertion";
  }
  if (edge.type === "hosts") {
    return "Workload inventory · " + (edge.sourceDetail || "unknown source");
  }
  if (edge.type === "matches_host") {
    return edge.sourceDetail === "exact-mac" ? "Proxmox match · exact MAC" : "Proxmox match · selected manually";
  }
  return edge.sourceDetail || edge.provenance;
}

function apiRelationshipError(error: unknown, fallback: string): string {
  const text = error instanceof Error ? error.message : "";
  if (text.includes("relationship already exists")) return "This manual relationship already exists.";
  if (text.includes("endpoint not found")) return "The selected Host no longer exists. Refresh and try again.";
  return fallback;
}

export default RelationshipsCard;
