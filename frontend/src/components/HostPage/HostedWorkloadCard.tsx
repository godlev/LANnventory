import { createEffect, createSignal, For, Show } from "solid-js";

import {
  apiGetInfrastructureWorkloadMemberships,
  type InfrastructureWorkloadMembership,
} from "../../functions/api";
import type { Host } from "../../functions/exports";

type Props = {
  host: Host;
};

function HostedWorkloadCard(props: Props) {
  const [memberships, setMemberships] = createSignal<InfrastructureWorkloadMembership[]>([]);
  const [loadError, setLoadError] = createSignal("");
  let requestID = 0;

  createEffect(() => {
    const hostID = props.host.ID;
    const activeRequest = ++requestID;
    setMemberships([]);
    setLoadError("");
    if (hostID < 1) return;

    apiGetInfrastructureWorkloadMemberships(hostID)
      .then((items) => {
        if (activeRequest !== requestID) return;
        setMemberships(items ?? []);
      })
      .catch(() => {
        if (activeRequest !== requestID) return;
        setLoadError("Hosted workload relationship could not be loaded.");
      });
  });

  return (
    <Show when={memberships().length > 0 || loadError()}>
      <section class="card wyl-panel host-panel hosted-workload-panel" aria-labelledby="hosted-workload-title">
        <div class="card-header host-panel-header">
          <div>
            <div id="hosted-workload-title" class="host-panel-title">Hosted on Proxmox</div>
            <div class="host-panel-subtitle">
              This LANnventory Host is linked to VM/LXC inventory owned by a Proxmox Host.
            </div>
          </div>
          <span class="host-detail-section-badge">Child relationship</span>
        </div>
        <div class="card-body">
          <Show when={!loadError()} fallback={<div class="host-inline-error" role="alert">{loadError()}</div>}>
            <div class="hosted-workload-list">
              <For each={memberships()}>{(item) => (
                <div class="hosted-workload-row">
                  <div class="hosted-workload-identity">
                    <span class="badge text-bg-secondary">
                      {item.workloadType === "container" ? "LXC" : "VM"} {item.nativeId}
                    </span>
                    <strong>{item.workloadName || "Unnamed workload"}</strong>
                    <span class={"proxmox-status "+hostedWorkloadStatusClass(item.workloadStatus)}>
                      <i
                        class={item.workloadStatus === "running" ? "bi bi-play-circle-fill" : item.workloadStatus === "stopped" ? "bi bi-stop-circle-fill" : "bi bi-question-circle-fill"}
                        aria-hidden="true"
                      ></i>
                      {capitalizeHostedStatus(item.workloadStatus)}
                    </span>
                  </div>
                  <div class="hosted-workload-parent">
                    <span class="device-cell-muted">Parent hypervisor</span>
                    <Show
                      when={item.hypervisorHostId > 0}
                      fallback={<span class="font-monospace">{item.hypervisorMac || "Unknown"}</span>}
                    >
                      <a href={"/host/"+item.hypervisorHostId}>
                        <i class="bi bi-diagram-2 me-1" aria-hidden="true"></i>
                        {item.hypervisorName || item.hypervisorIp || item.hypervisorMac}
                      </a>
                    </Show>
                    <Show when={item.hypervisorIp}>
                      <span class="font-monospace device-cell-muted">{item.hypervisorIp}</span>
                    </Show>
                  </div>
                  <div class="hosted-workload-link-source">
                    <span class={"badge "+(item.linkSource === "exact-mac" ? "text-bg-success" : "text-bg-primary")}>
                      {item.linkSource === "exact-mac" ? "Linked by exact MAC" : "Manually linked"}
                    </span>
                    <Show when={item.retiredAt}>
                      <span class="badge text-bg-secondary">Retired workload</span>
                    </Show>
                  </div>
                </div>
              )}</For>
            </div>
          </Show>
        </div>
      </section>
    </Show>
  );
}

function hostedWorkloadStatusClass(status: string) {
  switch (status) {
    case "running": return "is-running";
    case "stopped": return "is-stopped";
    default: return "is-unknown";
  }
}

function capitalizeHostedStatus(value: string) {
  return value ? value.charAt(0).toUpperCase()+value.slice(1) : "Unknown";
}

export default HostedWorkloadCard;
