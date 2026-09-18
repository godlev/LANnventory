import { createEffect, createMemo, createSignal, For, onCleanup, Show } from "solid-js";

import { apiGetHostServices } from "../../functions/api";
import { formatLastSeen } from "../../functions/dateFormat";
import type { Host, Service } from "../../functions/exports";

type ServicesCardProps = {
  host: Host;
  refreshKey?: number;
};

function ServicesCard(props: ServicesCardProps) {
  const [services, setServices] = createSignal<Service[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal("");
  let requestID = 0;

  createEffect(() => {
    const id = props.host.ID;
    void props.refreshKey;

    if (id < 1) {
      requestID++;
      setServices([]);
      setLoadError("");
      setLoading(false);
      return;
    }

    const activeRequest = ++requestID;
    setLoading(true);
    setLoadError("");

    apiGetHostServices(id)
      .then((result) => {
        if (activeRequest !== requestID) {
          return;
        }
        setServices(result);
      })
      .catch(() => {
        if (activeRequest !== requestID) {
          return;
        }
        setServices([]);
        setLoadError("Service inventory could not be loaded.");
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

  const sortedServices = createMemo(() =>
    [...services()].sort((left, right) => {
      if (left.state !== right.state) {
        return left.state === "open" ? -1 : 1;
      }
      const addressOrder = left.address.localeCompare(right.address);
      if (addressOrder !== 0) {
        return addressOrder;
      }
      const protocolOrder = left.protocol.localeCompare(right.protocol);
      return protocolOrder !== 0 ? protocolOrder : left.port - right.port;
    }),
  );
  const openCount = createMemo(() => services().filter((service) => service.state === "open").length);

  return (
    <section class="card wyl-panel host-panel" aria-labelledby="host-services-title">
      <div class="card-header host-panel-header">
        <div>
          <div id="host-services-title" class="host-panel-title">Services</div>
          <div class="host-panel-subtitle">
            Retained TCP service state is tracked per exact device address.
          </div>
        </div>
        <Show when={services().length > 0}>
          <span class="host-detail-section-badge">
            {openCount()} open · {services().length} retained
          </span>
        </Show>
      </div>

      <div class="card-body">
        <Show when={!loading()} fallback={<div class="device-cell-muted">Loading services…</div>}>
          <Show when={!loadError()} fallback={<div class="host-inline-error" role="alert">{loadError()}</div>}>
            <Show
              when={sortedServices().length > 0}
              fallback={
                <div class="device-cell-muted">
                  No services have been observed yet. Run a port scan to start building service inventory.
                </div>
              }
            >
              <div class="table-responsive">
                <table class="table table-sm align-middle mb-0">
                  <thead>
                    <tr>
                      <th scope="col">Service</th>
                      <th scope="col">State</th>
                      <th scope="col">Address</th>
                      <th scope="col">First detected</th>
                      <th scope="col">Last detected</th>
                      <th scope="col">Last checked</th>
                      <th scope="col">Source</th>
                    </tr>
                  </thead>
                  <tbody>
                    <For each={sortedServices()}>{(service) =>
                      <tr>
                        <td>
                          <div class="fw-semibold">{serviceLabel(service)}</div>
                          <Show when={service.serviceHint}>
                            <div class="small device-cell-muted">{service.serviceHint}</div>
                          </Show>
                        </td>
                        <td>
                          <span class={service.state === "open" ? "badge text-bg-success" : "badge text-bg-secondary"}>
                            {service.state === "open" ? "Open" : "Closed"}
                          </span>
                        </td>
                        <td>
                          <div class="font-monospace text-break">{service.address}</div>
                          <div class="small device-cell-muted">{service.addressFamily.toUpperCase()}</div>
                        </td>
                        <td>{formatServiceTime(service.firstDetected)}</td>
                        <td>{formatServiceTime(service.lastDetected)}</td>
                        <td>{formatServiceTime(service.lastChecked)}</td>
                        <td>{scanSourceLabel(service.lastScanSource)}</td>
                      </tr>
                    }</For>
                  </tbody>
                </table>
              </div>
            </Show>
          </Show>
        </Show>
      </div>
    </section>
  );
}

function serviceLabel(service: Service) {
  return service.protocol.toUpperCase() + "/" + service.port;
}

function formatServiceTime(value: string) {
  return value ? formatLastSeen(value) : "—";
}

function scanSourceLabel(value: string) {
  switch (value) {
    case "manual":
      return "Manual scan";
    case "scheduled":
      return "Scheduled scan";
    default:
      return value || "Unknown";
  }
}

export default ServicesCard;
