import { createEffect, createMemo, createSignal, For, onCleanup, Show } from "solid-js";

import {
  apiGetHostServices,
  apiGetServiceScanSettings,
  apiSetServiceScanSettings,
  type ServiceScanSettings,
} from "../../functions/api";
import { formatLastSeen } from "../../functions/dateFormat";
import type { Host, Service } from "../../functions/exports";

type ServicesCardProps = {
  host: Host;
  refreshKey?: number;
};

const defaultScanSettings: ServiceScanSettings = {
  enabled: false,
  intervalMinutes: 1440,
  ports: [],
  nextScanAt: "",
  lastAttemptAt: "",
  lastSuccessfulAt: "",
  lastError: "",
};

function ServicesCard(props: ServicesCardProps) {
  const [services, setServices] = createSignal<Service[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal("");
  const [scanSettings, setScanSettings] = createSignal<ServiceScanSettings>(defaultScanSettings);
  const [settingsLoading, setSettingsLoading] = createSignal(false);
  const [settingsError, setSettingsError] = createSignal("");
  const [settingsSaved, setSettingsSaved] = createSignal("");
  const [settingsDirty, setSettingsDirty] = createSignal(false);
  const [savingSettings, setSavingSettings] = createSignal(false);
  const [scheduledEnabled, setScheduledEnabled] = createSignal(false);
  const [intervalMinutes, setIntervalMinutes] = createSignal("1440");
  const [portsText, setPortsText] = createSignal("");
  let serviceRequestID = 0;
  let settingsRequestID = 0;

  createEffect(() => {
    const id = props.host.ID;
    void props.refreshKey;

    if (id < 1) {
      serviceRequestID++;
      setServices([]);
      setLoadError("");
      setLoading(false);
      return;
    }

    void loadServices(id);
  });

  createEffect(() => {
    const id = props.host.ID;
    if (id < 1) {
      settingsRequestID++;
      setScanSettings(defaultScanSettings);
      applySettingsDraft(defaultScanSettings);
      setSettingsDirty(false);
      setSettingsError("");
      setSettingsSaved("");
      setSettingsLoading(false);
      return;
    }

    setSettingsDirty(false);
    setSettingsSaved("");
    void loadSettings(id, true);
  });

  onCleanup(() => {
    serviceRequestID++;
    settingsRequestID++;
  });

  const loadServices = async (id: number) => {
    const activeRequest = ++serviceRequestID;
    setLoading(true);
    setLoadError("");

    try {
      const result = await apiGetHostServices(id);
      if (activeRequest === serviceRequestID) {
        setServices(result);
      }
    } catch {
      if (activeRequest === serviceRequestID) {
        setServices([]);
        setLoadError("Service inventory could not be loaded.");
      }
    } finally {
      if (activeRequest === serviceRequestID) {
        setLoading(false);
      }
    }
  };

  const loadSettings = async (id: number, replaceDraft: boolean) => {
    const activeRequest = ++settingsRequestID;
    setSettingsLoading(true);
    setSettingsError("");

    try {
      const result = await apiGetServiceScanSettings(id);
      if (activeRequest !== settingsRequestID) {
        return;
      }
      setScanSettings(result);
      if (replaceDraft || !settingsDirty()) {
        applySettingsDraft(result);
        setSettingsDirty(false);
      }
    } catch {
      if (activeRequest === settingsRequestID) {
        setSettingsError("Scheduled scan settings could not be loaded.");
      }
    } finally {
      if (activeRequest === settingsRequestID) {
        setSettingsLoading(false);
      }
    }
  };

  const applySettingsDraft = (settings: ServiceScanSettings) => {
    setScheduledEnabled(settings.enabled);
    setIntervalMinutes(String(settings.intervalMinutes || 1440));
    setPortsText(settings.ports.join(", "));
  };

  const markSettingsDirty = () => {
    setSettingsDirty(true);
    setSettingsSaved("");
    setSettingsError("");
  };

  const handleEnabledChange = (enabled: boolean) => {
    setScheduledEnabled(enabled);
    markSettingsDirty();
  };

  const handleIntervalChange = (value: string) => {
    setIntervalMinutes(value);
    markSettingsDirty();
  };

  const handlePortsChange = (value: string) => {
    setPortsText(value);
    markSettingsDirty();
  };

  const handleSaveSettings = async () => {
    if (props.host.ID < 1 || savingSettings()) {
      return;
    }

    setSettingsError("");
    setSettingsSaved("");

    const interval = Number(intervalMinutes().trim());
    if (!Number.isInteger(interval) || interval <= 0) {
      setSettingsError("Scan interval must be a whole number greater than zero.");
      return;
    }

    const parsedPorts = parsePortList(portsText());
    if (parsedPorts.error) {
      setSettingsError(parsedPorts.error);
      return;
    }
    if (scheduledEnabled() && parsedPorts.ports.length === 0) {
      setSettingsError("Add at least one TCP port before enabling scheduled scanning.");
      return;
    }

    setSavingSettings(true);
    try {
      const saved = await apiSetServiceScanSettings(props.host.ID, {
        enabled: scheduledEnabled(),
        intervalMinutes: interval,
        ports: parsedPorts.ports,
      });
      setScanSettings(saved);
      applySettingsDraft(saved);
      setSettingsDirty(false);
      setSettingsSaved(saved.enabled ? "Scheduled scanning enabled." : "Scheduled scanning disabled.");
    } catch (error) {
      setSettingsError(apiErrorMessage(error, "Scheduled scan settings could not be saved."));
    } finally {
      setSavingSettings(false);
    }
  };

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
  const openServices = createMemo(() => sortedServices().filter((service) => service.state === "open"));
  const historicalServices = createMemo(() => sortedServices().filter((service) => service.state !== "open"));

  return (
    <section class="card wyl-panel host-panel" aria-labelledby="host-services-title">
      <div class="card-header host-panel-header">
        <div>
          <div id="host-services-title" class="host-panel-title">Services</div>
          <div class="host-panel-subtitle">
            Current open services first. Previously observed services and scan scheduling are available below when you need them.
          </div>
        </div>
        <Show when={services().length > 0}>
          <span class="host-detail-section-badge">
            {openCount()} open · {historicalServices().length} historical
          </span>
        </Show>
      </div>

      <div class="card-body host-services-body">
        <Show when={!loading()} fallback={<div class="device-cell-muted">Loading services…</div>}>
          <Show when={!loadError()} fallback={<div class="host-inline-error" role="alert">{loadError()}</div>}>
            <section class="host-services-current" aria-labelledby="host-current-services-title">
              <div class="host-section-summary-heading">
                <div>
                  <div id="host-current-services-title" class="small fw-semibold">Current open services</div>
                  <div class="small device-cell-muted">Services reported open on the most recent check for each exact device address.</div>
                </div>
                <span class={openServices().length > 0 ? "badge text-bg-success" : "badge text-bg-secondary"}>
                  {openServices().length} open
                </span>
              </div>

              <Show
                when={openServices().length > 0}
                fallback={
                  <div class="host-services-empty">
                    <i class="bi bi-shield-check" aria-hidden="true"></i>
                    <span>No open services are currently recorded. Run a port scan to refresh service inventory.</span>
                  </div>
                }
              >
                <ServiceTable services={openServices()} />
              </Show>
            </section>

            <Show when={historicalServices().length > 0}>
              <details class="host-services-secondary">
                <summary>
                  <span>
                    <i class="bi bi-clock-history" aria-hidden="true"></i>
                    Previously observed services
                  </span>
                  <span class="badge text-bg-secondary">{historicalServices().length}</span>
                </summary>
                <div class="host-services-secondary-body">
                  <div class="small device-cell-muted mb-2">
                    Closed services are retained as history. They do not represent services that LANnventory currently sees as open.
                  </div>
                  <ServiceTable services={historicalServices()} />
                </div>
              </details>
            </Show>

            <details class="host-services-secondary">
              <summary>
                <span>
                  <i class="bi bi-clock" aria-hidden="true"></i>
                  Scheduled scanning
                </span>
                <span class={scanSettings().enabled ? "badge text-bg-success" : "badge text-bg-secondary"}>
                  {scanSettings().enabled ? "Enabled" : "Off"}
                </span>
              </summary>

              <div class="host-services-secondary-body">
                <Show when={!settingsLoading()} fallback={<div class="device-cell-muted">Loading scheduled scan settings…</div>}>
                  <div class="row g-3 align-items-end">
                    <div class="col-12 col-md-3 col-xl-2">
                      <div class="form-check form-switch">
                        <input
                          id="host-service-scan-enabled"
                          class="form-check-input"
                          type="checkbox"
                          role="switch"
                          checked={scheduledEnabled()}
                          onChange={(event) => handleEnabledChange(event.currentTarget.checked)}
                        />
                        <label class="form-check-label" for="host-service-scan-enabled">
                          Enable automatic scans
                        </label>
                      </div>
                    </div>

                    <div class="col-12 col-md-3 col-xl-2">
                      <label class="host-port-field w-100">
                        <span>Interval (minutes)</span>
                        <input
                          type="number"
                          min="1"
                          step="1"
                          class="form-control form-control-sm wyl-control"
                          value={intervalMinutes()}
                          onInput={(event) => handleIntervalChange(event.currentTarget.value)}
                        />
                      </label>
                    </div>

                    <div class="col-12 col-md-6 col-xl-6">
                      <label class="host-port-field w-100">
                        <span>TCP ports</span>
                        <input
                          type="text"
                          class="form-control form-control-sm wyl-control"
                          placeholder="22, 80, 443"
                          value={portsText()}
                          onInput={(event) => handlePortsChange(event.currentTarget.value)}
                        />
                      </label>
                    </div>

                    <div class="col-12 col-xl-2">
                      <button
                        type="button"
                        class="btn btn-sm wyl-button w-100"
                        disabled={props.host.ID < 1 || savingSettings() || !settingsDirty()}
                        onClick={handleSaveSettings}
                      >
                        <i class="bi bi-floppy" aria-hidden="true"></i>
                        <span>{savingSettings() ? "Saving…" : "Save"}</span>
                      </button>
                    </div>
                  </div>

                  <div class="small device-cell-muted mt-2">
                    Up to 256 unique TCP ports. Example: 22, 80, 443. 60 minutes = 1 hour; 1440 = 1 day.
                    Scheduled scanning is opt-in and stays off until you enable and save it.
                  </div>

                  <div class="row g-2 mt-2 small">
                    <RuntimeField label="Next scan" value={scanSettings().nextScanAt} />
                    <RuntimeField label="Last attempt" value={scanSettings().lastAttemptAt} />
                    <RuntimeField label="Last success" value={scanSettings().lastSuccessfulAt} />
                  </div>

                  <Show when={settingsError()}>
                    <div class="host-inline-error mt-2" role="alert">{settingsError()}</div>
                  </Show>
                  <Show when={settingsSaved()}>
                    <div class="small text-success mt-2" role="status">{settingsSaved()}</div>
                  </Show>
                  <Show when={scanSettings().lastError}>
                    <div class="alert alert-warning py-2 px-3 small mt-2 mb-0" role="status">
                      <strong>Last scheduled scan:</strong> {scanSettings().lastError}
                    </div>
                  </Show>
                </Show>
              </div>
            </details>
          </Show>
        </Show>
      </div>
    </section>
  );
}

function ServiceTable(props: { services: Service[] }) {
  return (
    <div class="table-responsive host-services-table-wrap">
      <table class="table table-sm align-middle mb-0 host-services-table">
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
          <For each={props.services}>{(service) =>
            <tr>
              <td data-label="Service">
                <div class="fw-semibold">{serviceLabel(service)}</div>
                <Show when={service.serviceHint}>
                  <div class="small device-cell-muted">Hint: {service.serviceHint}</div>
                </Show>
              </td>
              <td data-label="State">
                <span class={service.state === "open" ? "badge text-bg-success" : "badge text-bg-secondary"}>
                  {service.state === "open" ? "Open" : "Closed"}
                </span>
              </td>
              <td data-label="Address">
                <div class="font-monospace host-service-address">{service.address}</div>
                <div class="small device-cell-muted">{service.addressFamily.toUpperCase()}</div>
              </td>
              <td data-label="First detected">{formatServiceTime(service.firstDetected)}</td>
              <td data-label="Last detected">{formatServiceTime(service.lastDetected)}</td>
              <td data-label="Last checked">{formatServiceTime(service.lastChecked)}</td>
              <td data-label="Source">{scanSourceLabel(service.lastScanSource)}</td>
            </tr>
          }</For>
        </tbody>
      </table>
    </div>
  );
}

function RuntimeField(props: { label: string; value: string }) {
  return (
    <div class="col-12 col-md-4">
      <span class="fw-semibold">{props.label}:</span>{" "}
      <span class="device-cell-muted">{props.value ? formatLastSeen(props.value) : "—"}</span>
    </div>
  );
}

function parsePortList(value: string): { ports: number[]; error: string } {
  const raw = value.trim();
  if (raw === "") {
    return { ports: [], error: "" };
  }

  const tokens = raw.split(/[\s,;]+/).filter(Boolean);
  const ports = new Set<number>();
  for (const token of tokens) {
    if (!/^\d+$/.test(token)) {
      return { ports: [], error: `Invalid TCP port "${token}". Use numbers separated by commas.` };
    }
    const port = Number(token);
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      return { ports: [], error: `TCP port ${token} must be between 1 and 65535.` };
    }
    ports.add(port);
  }

  if (ports.size > 256) {
    return { ports: [], error: "Scheduled scans support up to 256 unique TCP ports." };
  }

  return {
    ports: [...ports].sort((left, right) => left - right),
    error: "",
  };
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

function apiErrorMessage(error: unknown, fallback: string) {
  if (!(error instanceof Error)) {
    return fallback;
  }
  const message = error.message.trim();
  if (!message) {
    return fallback;
  }
  try {
    const parsed = JSON.parse(message);
    if (typeof parsed?.error === "string" && parsed.error.trim()) {
      return parsed.error.trim();
    }
  } catch {
    // Keep the original API error text when it is not JSON.
  }
  return message;
}

export default ServicesCard;
