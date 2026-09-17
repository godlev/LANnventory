import { createEffect, createMemo, createSignal, For, Show, onCleanup } from "solid-js";

import {
  apiClearHostIdentityDecision,
  apiGetHostIdentityCandidates,
  apiGetHostIdentityDecisions,
  apiSetHostIdentityDecision,
  type CorrelationDecision,
  type HostIdentityCandidate,
  type IdentityCorrelationDecision,
} from "../../functions/correlationApi";
import type { Host } from "../../functions/exports";

type CorrelationPanelProps = {
  host: Host;
};

type CorrelationRow = {
  mac: string;
  candidate?: HostIdentityCandidate;
  decision?: IdentityCorrelationDecision;
};

function CorrelationPanel(props: CorrelationPanelProps) {
  const [candidates, setCandidates] = createSignal<HostIdentityCandidate[]>([]);
  const [decisions, setDecisions] = createSignal<IdentityCorrelationDecision[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal("");
  const [actionMAC, setActionMAC] = createSignal("");
  const [actionError, setActionError] = createSignal("");
  let requestID = 0;

  const load = async (id: number, activeRequest: number) => {
    setLoading(true);
    setLoadError("");
    try {
      const [candidateResult, decisionResult] = await Promise.all([
        apiGetHostIdentityCandidates(id),
        apiGetHostIdentityDecisions(id),
      ]);
      if (activeRequest !== requestID) {
        return;
      }
      setCandidates(candidateResult.candidates ?? []);
      setDecisions(decisionResult.decisions ?? []);
    } catch {
      if (activeRequest !== requestID) {
        return;
      }
      setCandidates([]);
      setDecisions([]);
      setLoadError("Identity correlation suggestions could not be loaded.");
    } finally {
      if (activeRequest === requestID) {
        setLoading(false);
      }
    }
  };

  createEffect(() => {
    const id = props.host.ID;
    if (id <= 0) {
      setCandidates([]);
      setDecisions([]);
      setLoadError("");
      setLoading(false);
      return;
    }
    const activeRequest = ++requestID;
    void load(id, activeRequest);
  });

  onCleanup(() => {
    requestID++;
  });

  const rows = createMemo<CorrelationRow[]>(() => {
    const byMAC = new Map<string, CorrelationRow>();
    for (const candidate of candidates()) {
      byMAC.set(normalizeMAC(candidate.mac), { mac: candidate.mac, candidate });
    }
    for (const decision of decisions()) {
      const key = normalizeMAC(decision.mac);
      const row = byMAC.get(key) ?? { mac: decision.mac };
      row.decision = decision;
      byMAC.set(key, row);
    }
    return [...byMAC.values()].sort((left, right) => {
      const leftConfirmed = left.decision?.decision === "confirmed" ? 1 : 0;
      const rightConfirmed = right.decision?.decision === "confirmed" ? 1 : 0;
      if (leftConfirmed !== rightConfirmed) {
        return rightConfirmed - leftConfirmed;
      }
      const scoreDiff = (right.candidate?.score ?? -1) - (left.candidate?.score ?? -1);
      return scoreDiff !== 0 ? scoreDiff : left.mac.localeCompare(right.mac);
    });
  });

  const setDecision = async (mac: string, decision: CorrelationDecision) => {
    if (props.host.ID <= 0 || actionMAC() !== "") {
      return;
    }
    setActionMAC(mac);
    setActionError("");
    try {
      await apiSetHostIdentityDecision(props.host.ID, mac, decision);
      const activeRequest = ++requestID;
      await load(props.host.ID, activeRequest);
    } catch {
      setActionError("The identity decision could not be saved.");
    } finally {
      setActionMAC("");
    }
  };

  const clearDecision = async (mac: string) => {
    if (props.host.ID <= 0 || actionMAC() !== "") {
      return;
    }
    setActionMAC(mac);
    setActionError("");
    try {
      await apiClearHostIdentityDecision(props.host.ID, mac);
      const activeRequest = ++requestID;
      await load(props.host.ID, activeRequest);
    } catch {
      setActionError("The identity decision could not be cleared.");
    } finally {
      setActionMAC("");
    }
  };

  return (
    <div class="mt-4 pt-3 border-top">
      <div class="d-flex flex-wrap justify-content-between align-items-start gap-2 mb-2">
        <div>
          <h6 class="mb-1">Possible same device</h6>
          <div class="small device-cell-muted">
            Suggestions are evidence-based. Nothing is merged automatically; your decision only records the relationship.
          </div>
        </div>
        <span class="host-detail-section-badge">Suggestion · User confirmed</span>
      </div>

      <Show when={actionError()}>
        <div class="host-inline-error mb-2" role="alert">{actionError()}</div>
      </Show>

      <Show when={!loading()} fallback={<div class="device-cell-muted">Loading correlation suggestions…</div>}>
        <Show when={!loadError()} fallback={<div class="host-inline-error" role="alert">{loadError()}</div>}>
          <Show
            when={rows().length > 0}
            fallback={<div class="device-cell-muted">No same-device candidates or explicit decisions yet.</div>}
          >
            <For each={rows()}>{(row) =>
              <CorrelationRowView
                row={row}
                busy={actionMAC() === row.mac}
                anyBusy={actionMAC() !== ""}
                onDecision={setDecision}
                onClear={clearDecision}
              ></CorrelationRowView>
            }</For>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

function CorrelationRowView(props: {
  row: CorrelationRow;
  busy: boolean;
  anyBusy: boolean;
  onDecision: (mac: string, decision: CorrelationDecision) => Promise<void>;
  onClear: (mac: string) => Promise<void>;
}) {
  const candidate = () => props.row.candidate;
  const decision = () => props.row.decision?.decision;
  const displayName = () => props.row.decision?.name || candidate()?.name || "";
  const exists = () => props.row.decision?.exists ?? candidate()?.exists ?? false;
  const active = () => props.row.decision?.active ?? candidate()?.active ?? false;

  return (
    <div class="border rounded p-2 mb-2">
      <div class="d-flex flex-wrap justify-content-between align-items-start gap-2">
        <div>
          <div class="d-flex flex-wrap align-items-center gap-2">
            <span class="font-monospace">{props.row.mac}</span>
            <Show when={displayName()}>
              <strong>{displayName()}</strong>
            </Show>
            <Show when={exists()}>
              <span class={active() ? "badge text-bg-success" : "badge text-bg-secondary"}>
                {active() ? "Current host" : "Known host"}
              </span>
            </Show>
            <Show when={!exists()}>
              <span class="badge text-bg-secondary">Historical MAC</span>
            </Show>
          </div>

          <Show when={candidate()}>
            {(item) =>
              <div class="small mt-1">
                <strong>{confidenceLabel(item().confidence)}</strong>
                <span class="device-cell-muted"> · score {item().score}</span>
              </div>
            }
          </Show>

          <Show when={decision()}>
            <div class="small mt-1">
              <strong>{decision() === "confirmed" ? "User confirmed same device" : "User marked not same device"}</strong>
            </div>
          </Show>
        </div>

        <div class="d-flex flex-wrap gap-1">
          <button
            type="button"
            class={"btn btn-sm " + (decision() === "confirmed" ? "btn-success" : "btn-outline-success")}
            disabled={props.anyBusy || decision() === "confirmed"}
            onClick={() => void props.onDecision(props.row.mac, "confirmed")}
          >
            {props.busy ? "Saving…" : "Same device"}
          </button>
          <button
            type="button"
            class={"btn btn-sm " + (decision() === "rejected" ? "btn-secondary" : "btn-outline-secondary")}
            disabled={props.anyBusy || decision() === "rejected"}
            onClick={() => void props.onDecision(props.row.mac, "rejected")}
          >
            {props.busy ? "Saving…" : "Not same"}
          </button>
          <Show when={decision()}>
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              disabled={props.anyBusy}
              onClick={() => void props.onClear(props.row.mac)}
            >
              Clear
            </button>
          </Show>
        </div>
      </div>

      <Show when={(candidate()?.reasons?.length ?? 0) > 0}>
        <div class="mt-2 small">
          <For each={candidate()?.reasons ?? []}>{(reason) =>
            <div class="d-flex gap-2 align-items-start mt-1">
              <span class={reason.weight >= 0 ? "text-success" : "text-danger"} aria-hidden="true">
                {reason.weight >= 0 ? "+" : "−"}
              </span>
              <span>{reason.detail}</span>
            </div>
          }</For>
        </div>
      </Show>
    </div>
  );
}

function normalizeMAC(value: string) {
  return value.trim().replace(/-/g, ":").toUpperCase();
}

function confidenceLabel(value: string) {
  switch (value.toLowerCase()) {
    case "high": return "High confidence suggestion";
    case "medium": return "Medium confidence suggestion";
    case "low": return "Low confidence suggestion";
    default: return value ? value + " confidence suggestion" : "Correlation suggestion";
  }
}

export default CorrelationPanel;
