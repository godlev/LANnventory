import { createEffect, createMemo, createSignal, For, Show, onCleanup } from "solid-js";

import {
  apiClearHostIdentityDecision,
  apiGetHostIdentityCandidates,
  apiGetHostIdentityDecisions,
  apiGetHostIdentityGroup,
  apiSetHostIdentityDecision,
  type ConfirmedIdentityGroupMember,
  type CorrelationDecision,
  type HostIdentityCandidate,
  type HostIdentityGroup,
  type IdentityCorrelationDecision,
} from "../../functions/correlationApi";
import { formatLastSeen } from "../../functions/dateFormat";
import type { Host } from "../../functions/exports";

type CorrelationPanelProps = {
  host: Host;
};

type CorrelationRow = {
  mac: string;
  candidate?: HostIdentityCandidate;
  decision?: IdentityCorrelationDecision;
};

const emptyGroup: HostIdentityGroup = {
  mac: "",
  confirmed: false,
  firstSeen: "",
  lastSeen: "",
  members: [],
};

function CorrelationPanel(props: CorrelationPanelProps) {
  const [candidates, setCandidates] = createSignal<HostIdentityCandidate[]>([]);
  const [decisions, setDecisions] = createSignal<IdentityCorrelationDecision[]>([]);
  const [group, setGroup] = createSignal<HostIdentityGroup>(emptyGroup);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal("");
  const [actionMAC, setActionMAC] = createSignal("");
  const [actionError, setActionError] = createSignal("");
  let requestID = 0;

  const load = async (id: number, activeRequest: number) => {
    setLoading(true);
    setLoadError("");
    try {
      const [candidateResult, decisionResult, groupResult] = await Promise.all([
        apiGetHostIdentityCandidates(id),
        apiGetHostIdentityDecisions(id),
        apiGetHostIdentityGroup(id),
      ]);
      if (activeRequest !== requestID) {
        return;
      }
      setCandidates(candidateResult.candidates ?? []);
      setDecisions(decisionResult.decisions ?? []);
      setGroup({
        mac: groupResult.mac ?? "",
        confirmed: groupResult.confirmed === true,
        firstSeen: groupResult.firstSeen ?? "",
        lastSeen: groupResult.lastSeen ?? "",
        members: groupResult.members ?? [],
      });
    } catch {
      if (activeRequest !== requestID) {
        return;
      }
      setCandidates([]);
      setDecisions([]);
      setGroup(emptyGroup);
      setLoadError("Identity correlation information could not be loaded.");
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
      setGroup(emptyGroup);
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
    } catch (error) {
      setActionError(errorMessage(error, "The identity decision could not be saved."));
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
    } catch (error) {
      setActionError(errorMessage(error, "The identity decision could not be cleared."));
    } finally {
      setActionMAC("");
    }
  };

  return (
    <div class="host-identity-matches mt-4 pt-3 border-top">
      <Show when={actionError()}>
        <div class="host-inline-error mb-3" role="alert">{actionError()}</div>
      </Show>

      <Show when={!loading()} fallback={<div class="device-cell-muted">Loading identity correlation…</div>}>
        <Show when={!loadError()} fallback={<div class="host-inline-error" role="alert">{loadError()}</div>}>
          <Show when={group().confirmed && group().members.length > 1}>
            <div class="mb-4">
              <div class="d-flex flex-wrap justify-content-between align-items-start gap-2 mb-2">
                <div>
                  <h6 class="mb-1">Confirmed same-device identities</h6>
                  <div class="small device-cell-muted">
                    These MAC identities are grouped only because you explicitly confirmed the relationship. Original observations stay separate.
                  </div>
                  <Show when={group().firstSeen || group().lastSeen}>
                    <div class="small device-cell-muted mt-1">
                      <Show when={group().firstSeen}>Group first seen {formatIdentityTime(group().firstSeen)}</Show>
                      <Show when={group().firstSeen && group().lastSeen}> · </Show>
                      <Show when={group().lastSeen}>Last seen {formatIdentityTime(group().lastSeen)}</Show>
                    </div>
                  </Show>
                </div>
                <span class="host-detail-section-badge">User confirmed</span>
              </div>
              <div class="row g-2">
                <For each={group().members}>{(member) =>
                  <div class="col-12 col-xl-6">
                    <ConfirmedGroupMember member={member} currentMac={group().mac || props.host.Mac}></ConfirmedGroupMember>
                  </div>
                }</For>
              </div>
            </div>
          </Show>

          <div class={group().confirmed && group().members.length > 1 ? "pt-3 border-top" : ""}>
            <div class="d-flex flex-wrap justify-content-between align-items-start gap-2 mb-2">
              <div>
                <h6 class="mb-1">Possible identity matches</h6>
                <div class="small device-cell-muted">
                  LANnventory uses network evidence to suggest possible same-device relationships. Shared IP history alone is not a same-device conclusion.
                </div>
              </div>
              <span class="host-detail-section-badge">Suggestion · User decision</span>
            </div>

            <Show
              when={rows().length > 0}
              fallback={<div class="device-cell-muted">No evidence currently suggests that another MAC belongs to this same physical device.</div>}
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
          </div>
        </Show>
      </Show>
    </div>
  );
}

function ConfirmedGroupMember(props: { member: ConfirmedIdentityGroupMember; currentMac: string }) {
  const current = () => normalizeMAC(props.member.mac) === normalizeMAC(props.currentMac);
  const label = () => props.member.name || props.member.deviceType || "";

  return (
    <div class={"border rounded p-2 h-100" + (current() ? " border-success" : "")}>
      <div class="d-flex flex-wrap justify-content-between align-items-start gap-2">
        <div>
          <div class="d-flex flex-wrap align-items-center gap-2">
            <Show
              when={props.member.exists && props.member.hostId > 0}
              fallback={<span class="font-monospace">{props.member.mac}</span>}
            >
              <a class="font-monospace" href={"/host/" + props.member.hostId}>{props.member.mac}</a>
            </Show>
            <Show when={label()}><strong>{label()}</strong></Show>
          </div>
          <div class="d-flex flex-wrap gap-1 mt-1">
            <Show when={current()}><span class="badge text-bg-success">Viewed identity</span></Show>
            <Show when={props.member.exists}>
              <span class={props.member.active ? "badge text-bg-success" : "badge text-bg-secondary"}>
                {props.member.active ? "Current host · online" : "Current host · offline"}
              </span>
            </Show>
            <Show when={!props.member.exists}><span class="badge text-bg-secondary">Historical MAC</span></Show>
          </div>
        </div>
      </div>

      <Show when={props.member.addresses.length > 0}>
        <div class="small mt-2">
          <span class="device-cell-muted">Addresses: </span>
          <For each={props.member.addresses}>{(address, index) =>
            <>
              <Show when={index() > 0}>, </Show>
              <span class="font-monospace">{address}</span>
            </>
          }</For>
        </div>
      </Show>

      <Show when={props.member.firstSeen || props.member.lastSeen}>
        <div class="small device-cell-muted mt-1">
          <Show when={props.member.firstSeen}>First seen {formatIdentityTime(props.member.firstSeen)}</Show>
          <Show when={props.member.firstSeen && props.member.lastSeen}> · </Show>
          <Show when={props.member.lastSeen}>Last seen {formatIdentityTime(props.member.lastSeen)}</Show>
        </div>
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

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }
  return fallback;
}

function formatIdentityTime(value: string) {
  return value ? formatLastSeen(value) : "Unknown";
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
