import { createEffect, createMemo, createSignal, For, Show, onCleanup } from "solid-js";

import {
  apiGetHostIdentity,
  type AddressMACObservation,
  type DiscoveryEvidenceObservation,
  type HostIdentity,
  type HostIdentityAddress,
} from "../../functions/api";
import { formatLastSeen } from "../../functions/dateFormat";
import ActionTooltip from "../ActionTooltip";
import type { Host } from "../../functions/exports";
import CorrelationPanel from "./CorrelationPanel";

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

  const activeAddresses = createMemo(() => identity().addresses.filter((item) => item.active));
  const historicalAddresses = createMemo(() => identity().addresses.filter((item) => !item.active));
  const activeEvidence = createMemo(() => identity().evidence.filter((item) => item.active));
  const historicalEvidence = createMemo(() => identity().evidence.filter((item) => !item.active));

  return (
    <div class="card wyl-panel host-panel">
      <div class="card-header host-panel-header">
        <div>
          <div class="host-panel-title">Network identity</div>
          <div class="host-panel-subtitle">
            Current observations first. Historical IP reuse and previous discovered names remain available separately and do not prove physical-device identity.
          </div>
        </div>
        <span class="host-detail-section-badge">Discovered · Read only</span>
      </div>

      <div class="card-body host-identity-body">
        <Show when={!loading()} fallback={<div class="device-cell-muted">Loading identity observations…</div>}>
          <Show when={!loadError()} fallback={<div class="host-inline-error" role="alert">{loadError()}</div>}>
            <section class="host-identity-current" aria-labelledby="host-current-identity-title">
              <div class="host-services-section-heading">
                <div>
                  <div id="host-current-identity-title" class="small fw-semibold">Current network identity</div>
                  <div class="small device-cell-muted">What LANnventory currently observes for this MAC and its active addresses.</div>
                </div>
                <span class="host-detail-section-badge">Current</span>
              </div>

              <div class="row g-3 mt-0">
                <div class="col-12 col-xl-6">
                  <div class="small fw-semibold mb-2">Current addresses</div>
                  <Show
                    when={activeAddresses().length > 0}
                    fallback={<div class="device-cell-muted">No active address observation is retained right now.</div>}
                  >
                    <For each={activeAddresses()}>{(address) =>
                      <AddressObservation address={address} currentMac={identity().mac || props.host.Mac}></AddressObservation>
                    }</For>
                  </Show>
                </div>

                <div class="col-12 col-xl-6">
                  <div class="small fw-semibold mb-1">Current discovered identities</div>
                  <div class="small device-cell-muted mb-2">Scanner, Reverse DNS, mDNS and SSDP are discovery sources, not managed device properties.</div>

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

                  <Show
                    when={activeEvidence().length > 0}
                    fallback={<div class="device-cell-muted">No current discovered identity evidence is retained.</div>}
                  >
                    <For each={activeEvidence()}>{(item) => <EvidenceObservation item={item}></EvidenceObservation>}</For>
                  </Show>
                </div>
              </div>
            </section>

            <Show when={historicalAddresses().length > 0 || historicalEvidence().length > 0}>
              <details class="host-identity-history">
                <summary>
                  <span>
                    <i class="bi bi-clock-history" aria-hidden="true"></i>
                    Identity history
                  </span>
                  <span class="badge text-bg-secondary">
                    {historicalAddresses().length + historicalEvidence().length}
                  </span>
                </summary>
                <div class="host-identity-history-body">
                  <div class="small device-cell-muted mb-3">
                    Previous addresses and names are historical observations. An IP used by another MAC is context only, not proof that both MACs belong to the same physical device.
                  </div>
                  <div class="row g-3">
                    <Show when={historicalAddresses().length > 0}>
                      <div class="col-12 col-xl-6">
                        <div class="small fw-semibold mb-2">Previous addresses</div>
                        <For each={historicalAddresses()}>{(address) =>
                          <AddressObservation address={address} currentMac={identity().mac || props.host.Mac}></AddressObservation>
                        }</For>
                      </div>
                    </Show>
                    <Show when={historicalEvidence().length > 0}>
                      <div class="col-12 col-xl-6">
                        <div class="small fw-semibold mb-2">Previous discovered identities</div>
                        <For each={historicalEvidence()}>{(item) => <EvidenceObservation item={item} historical></EvidenceObservation>}</For>
                      </div>
                    </Show>
                  </div>
                </div>
              </details>
            </Show>

            <CorrelationPanel host={props.host}></CorrelationPanel>
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
        <span>
          <span class="device-cell-muted">IP </span>
          <span class="font-monospace">{props.address.address}</span>
        </span>
        <span class={props.address.active ? "badge text-bg-success" : "badge text-bg-secondary"}>
          {props.address.active ? "Current" : "Previous"}
        </span>
      </div>
      <div class="small device-cell-muted mt-1">
        {props.address.family.toUpperCase()}
        <Show when={props.address.iface}> · Scanner interface {props.address.iface}</Show>
      </div>
      <div class="small mt-1">
        Address first seen {formatIdentityTime(props.address.firstSeen)} · Address last seen {formatIdentityTime(props.address.lastSeen)}
      </div>

      <Show when={otherMACs().length > 0}>
        <div class="mt-2 small">
          <div class="d-flex flex-wrap align-items-center gap-2">
            <strong>
              This IP was also used by {otherMACs().length} other MAC{otherMACs().length === 1 ? "" : "s"}
            </strong>
            <ActionTooltip
              title="IP reuse history"
              detail="An IP address can be reused by different devices. This alone does not mean the MAC addresses belong to the same physical device."
            >
              <span
                class="host-lifecycle-approx"
                title="IP reuse history"
                aria-label="IP reuse history explanation"
                role="img"
                tabIndex={0}
              >
                <i class="bi bi-info-circle" aria-hidden="true"></i>
              </span>
            </ActionTooltip>
          </div>
          <For each={otherMACs()}>{(observation) =>
            <MACObservation address={props.address.address} observation={observation}></MACObservation>
          }</For>
        </div>
      </Show>
    </div>
  );
}

function MACObservation(props: { address: string; observation: AddressMACObservation }) {
  return (
    <div class="mt-1 d-flex flex-wrap gap-1 align-items-baseline">
      <span class="device-cell-muted">IP</span>
      <span class="font-monospace">{props.address}</span>
      <span class="device-cell-muted">· MAC</span>
      <span class="font-monospace">{props.observation.mac}</span>
      <span class="device-cell-muted">
        · Observed {formatIdentityTime(props.observation.firstSeen)} → {formatIdentityTime(props.observation.lastSeen)}
        {props.observation.active ? " · currently active on this IP" : ""}
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
