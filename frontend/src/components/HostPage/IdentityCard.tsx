import { createEffect, createMemo, createSignal, For, Show, onCleanup } from "solid-js";

import {
  apiGetHostIdentity,
  type AddressMACObservation,
  type DiscoveryEvidenceObservation,
  type HostIdentity,
  type HostIdentityAddress,
} from "../../functions/api";
import { formatLastSeen } from "../../functions/dateFormat";
import type { Host } from "../../functions/exports";

type IdentityCardProps = {
  host: Host;
};

const emptyIdentity: HostIdentity = {
  mac: "",
  addresses: [],
  evidence: [],
  dataSources: [],
};

function IdentityCard(props: IdentityCardProps) {
  const [identity, setIdentity] = createSignal<HostIdentity>(emptyIdentity);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal("");
  let requestID = 0;

  createEffect(() => {
    const id = props.host.ID;
    if (id <= 0) {
      setIdentity(emptyIdentity);
      setLoadError("");
      setLoading(false);
      return;
    }

    const activeRequest = ++requestID;
    setLoading(true);
    setLoadError("");

    apiGetHostIdentity(id)
      .then((result) => {
        if (activeRequest !== requestID) {
          return;
        }
        setIdentity(result);
      })
      .catch(() => {
        if (activeRequest !== requestID) {
          return;
        }
        setIdentity(emptyIdentity);
        setLoadError("Identity observations could not be loaded.");
      })
      .finally(() => {
        if (activeRequest === requestID) {
          setLoading(false);
        }
      });
  });

  onCleanup(() => {
    requestID++;
  });

  const activeEvidence = createMemo(() => identity().evidence.filter((item) => item.active));
  const historicalEvidence = createMemo(() => identity().evidence.filter((item) => !item.active));

  return (
    <div class="card wyl-panel host-panel">
      <div class="card-header host-panel-header">
        <div>
          <div class="host-panel-title">Identity observations</div>
          <div class="host-panel-subtitle">
            Discovered network identity is read only and never overwrites managed inventory.
          </div>
        </div>
        <span class="host-detail-section-badge">Discovered · Read only</span>
      </div>

      <div class="card-body">
        <Show when={!loading()} fallback={<div class="device-cell-muted">Loading identity observations…</div>}>
          <Show when={!loadError()} fallback={<div class="host-inline-error" role="alert">{loadError()}</div>}>
            <div class="row g-3">
              <div class="col-12 col-xl-6">
                <h6 class="mb-2">Addresses</h6>
                <Show
                  when={identity().addresses.length > 0}
                  fallback={<div class="device-cell-muted">No retained address observations yet.</div>}
                >
                  <For each={identity().addresses}>{(address) =>
                    <AddressObservation address={address} currentMac={identity().mac || props.host.Mac}></AddressObservation>
                  }</For>
                </Show>
              </div>

              <div class="col-12 col-xl-6">
                <h6 class="mb-2">Data sources</h6>
                <Show
                  when={identity().dataSources.length > 0}
                  fallback={<div class="device-cell-muted mb-3">No discovery sources have reported identity data yet.</div>}
                >
                  <div class="d-flex flex-wrap gap-2 mb-3">
                    <For each={identity().dataSources}>{(source) =>
                      <span class="badge rounded-pill text-bg-secondary">✓ {sourceLabel(source)}</span>
                    }</For>
                  </div>
                </Show>

                <h6 class="mb-2">Discovered identities</h6>
                <Show
                  when={identity().evidence.length > 0}
                  fallback={<div class="device-cell-muted">No discovered identity evidence yet.</div>}
                >
                  <Show when={activeEvidence().length > 0}>
                    <div class="small fw-semibold mb-1">Current evidence</div>
                    <For each={activeEvidence()}>{(item) => <EvidenceObservation item={item}></EvidenceObservation>}</For>
                  </Show>
                  <Show when={historicalEvidence().length > 0}>
                    <div class="small fw-semibold mt-3 mb-1">Previous evidence</div>
                    <For each={historicalEvidence()}>{(item) => <EvidenceObservation item={item} historical></EvidenceObservation>}</For>
                  </Show>
                </Show>
              </div>
            </div>
          </Show>
        </Show>
      </div>
    </div>
  );
}

function AddressObservation(props: { address: HostIdentityAddress; currentMac: string }) {
  const otherMACs = createMemo(() => {
    const current = normalizeMAC(props.currentMac);
    return props.address.macHistory.filter((item) => normalizeMAC(item.mac) !== current);
  });

  return (
    <div class="border rounded p-2 mb-2">
      <div class="d-flex flex-wrap justify-content-between gap-2 align-items-center">
        <span class="font-monospace">{props.address.address}</span>
        <span class={props.address.active ? "badge text-bg-success" : "badge text-bg-secondary"}>
          {props.address.active ? "Current" : "Previous"}
        </span>
      </div>
      <div class="small device-cell-muted mt-1">
        {props.address.family.toUpperCase()}
        <Show when={props.address.iface}> · Scanner interface {props.address.iface}</Show>
      </div>
      <div class="small mt-1">
        First seen {formatIdentityTime(props.address.firstSeen)} · Last seen {formatIdentityTime(props.address.lastSeen)}
      </div>

      <Show when={otherMACs().length > 0}>
        <div class="mt-2 small">
          <strong>Also observed with {otherMACs().length} other MAC{otherMACs().length === 1 ? "" : "s"}</strong>
          <For each={otherMACs()}>{(observation) => <MACObservation observation={observation}></MACObservation>}</For>
        </div>
      </Show>
    </div>
  );
}

function MACObservation(props: { observation: AddressMACObservation }) {
  return (
    <div class="mt-1">
      <span class="font-monospace">{props.observation.mac}</span>
      <span class="device-cell-muted">
        {" · "}{formatIdentityTime(props.observation.firstSeen)} → {formatIdentityTime(props.observation.lastSeen)}
        {props.observation.active ? " · currently active" : ""}
      </span>
    </div>
  );
}

function EvidenceObservation(props: { item: DiscoveryEvidenceObservation; historical?: boolean }) {
  return (
    <div class={"border rounded p-2 mb-2" + (props.historical ? " opacity-75" : "")}>
      <div class="d-flex flex-wrap justify-content-between gap-2">
        <div>
          <span class="small fw-semibold">{kindLabel(props.item.kind)}</span>
          <span class="device-cell-muted small"> · {sourceLabel(props.item.source)}</span>
        </div>
        <Show when={props.item.address}>
          <span class="small font-monospace">{props.item.address}</span>
        </Show>
      </div>
      <div class="mt-1">{props.item.value}</div>
      <div class="small device-cell-muted mt-1">
        First seen {formatIdentityTime(props.item.firstSeen)} · Last seen {formatIdentityTime(props.item.lastSeen)}
      </div>
    </div>
  );
}

function formatIdentityTime(value: string) {
  return value ? formatLastSeen(value) : "Unknown";
}

function normalizeMAC(value: string) {
  return value.trim().replace(/-/g, ":").toUpperCase();
}

function sourceLabel(source: string) {
  switch (source) {
    case "scanner": return "Scanner";
    case "reverse-dns": return "Reverse DNS";
    case "system-resolver": return "System resolver";
    case "mdns": return "mDNS";
    case "ssdp": return "SSDP / UPnP";
    default: return source || "Unknown source";
  }
}

function kindLabel(kind: string) {
  switch (kind) {
    case "vendor": return "Vendor";
    case "hostname": return "Hostname";
    case "friendly-name": return "Friendly name";
    case "manufacturer": return "Manufacturer";
    case "model": return "Model";
    case "model-number": return "Model number";
    default: return kind || "Identity";
  }
}

export default IdentityCard;
