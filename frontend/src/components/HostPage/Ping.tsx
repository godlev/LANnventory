import { createEffect, createMemo, createSignal, For, onCleanup, Show } from "solid-js";
import {
  apiCancelHostPortScan,
  apiGetActiveHostPortScan,
  apiGetHostPortScan,
  apiGetHostPorts,
  apiStartHostPortScan,
  type HostPortScanJob,
  type HostPortState,
} from "../../functions/api";
import type { Host } from "../../functions/exports";

type PingProps = {
  host: Host;
};

const pollIntervalMs = 350;
const httpBrowserPorts = new Set([80, 3000, 8000, 8080, 8840, 9090, 32400]);
const httpsBrowserPorts = new Set([443, 8443]);

function browserPortHref(host: string, port: number): string | undefined {
  if (httpsBrowserPorts.has(port)) {
    return "https://" + host + ":" + port;
  }
  if (httpBrowserPorts.has(port)) {
    return "http://" + host + ":" + port;
  }
  return undefined;
}

function Ping(props: PingProps) {
  const [beginStr, setBegin] = createSignal("");
  const [endStr, setEnd] = createSignal("");
  const [job, setJob] = createSignal<HostPortScanJob>();
  const [knownPorts, setKnownPorts] = createSignal<HostPortState[]>([]);
  const [scanError, setScanError] = createSignal("");
  const [loadingPorts, setLoadingPorts] = createSignal(false);
  let pollTimer: number | undefined;
  let pollToken = 0;

  const clearPoll = () => {
    pollToken++;
    if (pollTimer !== undefined) {
      window.clearTimeout(pollTimer);
      pollTimer = undefined;
    }
  };

  const loadKnownPorts = async (hostID: number) => {
    if (hostID < 1) {
      setKnownPorts([]);
      return;
    }

    setLoadingPorts(true);
    try {
      setKnownPorts(await apiGetHostPorts(hostID));
    } catch {
      setKnownPorts([]);
    } finally {
      setLoadingPorts(false);
    }
  };

  onCleanup(clearPoll);

  const knownByPort = createMemo(() => {
    const map = new Map<number, HostPortState>();
    for (const state of knownPorts()) {
      map.set(state.port, state);
    }
    return map;
  });

  const displayedOpenPorts = createMemo(() => {
    const ports = new Set<number>();
    for (const state of knownPorts()) {
      if (state.open) {
        ports.add(state.port);
      }
    }
    for (const port of job()?.openPorts ?? []) {
      ports.add(port);
    }
    return [...ports].sort((left, right) => left - right);
  });

  const progressPercent = () => {
    const current = job();
    if (!current || current.total <= 0) {
      return 0;
    }
    return Math.min(100, Math.round((current.scanned / current.total) * 1000) / 10);
  };

  const scanStatus = () => {
    const current = job();
    if (!current) {
      return "";
    }
    if (current.running) {
      const latest = current.currentPort > 0 ? " · latest port " + current.currentPort : "";
      return "Scanning " + current.scanned + " / " + current.total + " (" + progressPercent() + "%)" + latest;
    }
    if (current.cancelled) {
      return "Stopped after " + current.scanned + " / " + current.total + " ports";
    }
    if (current.error) {
      return "Scan ended with an error";
    }
    return "Completed " + current.scanned + " / " + current.total + " ports";
  };

  const parseRange = (): [number, number] | undefined => {
    const parsePort = (raw: string, fallback: number): number | undefined => {
      const trimmed = raw.trim();
      if (trimmed === "") {
        return fallback;
      }
      const value = Number(trimmed);
      if (!Number.isInteger(value) || value < 1 || value > 65535) {
        return undefined;
      }
      return value;
    };

    let startPort = parsePort(beginStr(), 1);
    let endPort = parsePort(endStr(), 65535);
    if (startPort === undefined || endPort === undefined) {
      setScanError("Ports must be whole numbers between 1 and 65535.");
      return undefined;
    }
    if (startPort > endPort) {
      [startPort, endPort] = [endPort, startPort];
    }
    return [startPort, endPort];
  };

  const pollScan = (hostID: number, scanID: string, token: number) => {
    const run = async () => {
      if (token !== pollToken) {
        return;
      }

      try {
        const next = await apiGetHostPortScan(hostID, scanID);
        if (token !== pollToken) {
          return;
        }
        setJob(next);

        if (next.running) {
          pollTimer = window.setTimeout(run, pollIntervalMs);
          return;
        }

        if (next.error) {
          setScanError(next.error);
        }
        await loadKnownPorts(hostID);
      } catch (error) {
        if (token !== pollToken) {
          return;
        }
        setScanError(error instanceof Error ? error.message : "Port scan status could not be loaded");
      }
    };

    pollTimer = window.setTimeout(run, pollIntervalMs);
  };

  const resumeActiveScan = async (hostID: number, token: number) => {
    if (hostID < 1) {
      return;
    }

    try {
      const active = await apiGetActiveHostPortScan(hostID);
      if (token !== pollToken || !active) {
        return;
      }

      setJob(active);
      pollScan(hostID, active.id, token);
    } catch {
      // Port-state loading remains useful even if active-job recovery fails.
    }
  };

  createEffect(() => {
    const hostID = props.host.ID;
    clearPoll();
    setJob(undefined);
    setScanError("");
    void loadKnownPorts(hostID);
    void resumeActiveScan(hostID, pollToken);
  });

  const handleScan = async () => {
    if (props.host.ID < 1 || !props.host.IP || job()?.running) {
      return;
    }

    const range = parseRange();
    if (!range) {
      return;
    }

    clearPoll();
    setScanError("");
    try {
      const next = await apiStartHostPortScan(props.host.ID, range[0], range[1]);
      setJob(next);
      const token = pollToken;
      pollScan(props.host.ID, next.id, token);
    } catch (error) {
      setScanError(error instanceof Error ? error.message : "Port scan could not be started");
    }
  };

  const handleStop = async () => {
    const current = job();
    if (!current?.running) {
      return;
    }

    try {
      const next = await apiCancelHostPortScan(props.host.ID, current.id);
      setJob(next);
    } catch (error) {
      setScanError(error instanceof Error ? error.message : "Port scan could not be stopped");
    }
  };

  const portTitle = (port: number) => {
    const state = knownByPort().get(port);
    const parts = ["TCP " + port];
    if (state?.service) {
      parts.push(state.service);
    }
    if (state?.lastScanned) {
      parts.push("last scanned " + state.lastScanned);
    }
    return parts.join(" · ");
  };

  return (
    <div class="card wyl-panel host-panel">
      <div class="card-header host-panel-header">
        <div>
          <div class="host-panel-title">Port scan</div>
          <div class="host-panel-subtitle">{props.host.IP || "Waiting for host"}</div>
        </div>
      </div>
      <div class="card-body host-port-body">
        <form class="host-port-controls" onSubmit={(event) => event.preventDefault()}>
          <label class="host-port-field">
            <span>Start port</span>
            <input
              type="text"
              inputmode="numeric"
              class="form-control form-control-sm wyl-control host-port-input"
              placeholder="1"
              value={beginStr()}
              disabled={job()?.running}
              onInput={(event) => setBegin(event.currentTarget.value)}
            />
          </label>
          <label class="host-port-field">
            <span>End port</span>
            <input
              type="text"
              inputmode="numeric"
              class="form-control form-control-sm wyl-control host-port-input"
              placeholder="65535"
              value={endStr()}
              disabled={job()?.running}
              onInput={(event) => setEnd(event.currentTarget.value)}
            />
          </label>
          <button
            type="button"
            onClick={() => void handleScan()}
            class="btn btn-sm wyl-button host-scan-button"
            disabled={props.host.ID < 1 || !props.host.IP || job()?.running}
          >
            <i class="bi bi-search" aria-hidden="true"></i>
            <span>Scan</span>
          </button>
        </form>

        <Show when={job()}>
          {(current) =>
            <div class="host-scan-state">
              <div class="host-scan-progress-wrap">
                <progress
                  class="host-port-progress"
                  max={Math.max(1, current().total)}
                  value={current().scanned}
                  aria-label="Port scan progress"
                />
                <div class="host-scan-status">{scanStatus()}</div>
              </div>
              <Show when={current().running}>
                <button type="button" onClick={() => void handleStop()} class="btn btn-sm wyl-button host-stop-button">
                  Stop
                </button>
              </Show>
            </div>
          }
        </Show>

        <Show when={scanError()}>
          <div class="host-inline-error" role="alert">{scanError()}</div>
        </Show>

        <div class="host-found-ports-header">
          <span>Known open ports</span>
          <span>{loadingPorts() ? "…" : displayedOpenPorts().length}</span>
        </div>
        <div class="host-found-ports">
          <For each={displayedOpenPorts()}>{(port) => {
            const state = () => knownByPort().get(port);
            const href = () => browserPortHref(props.host.IP, port);
            const content = () => (
              <>
                <span>{port}</span>
                <Show when={state()?.service}>
                  <span class="host-port-service">{state()!.service}</span>
                </Show>
              </>
            );

            return (
              <Show
                when={href()}
                fallback={<span class="host-port-chip" title={portTitle(port)}>{content()}</span>}
              >
                {(url) =>
                  <a
                    class="host-port-chip"
                    href={url()}
                    target="_blank"
                    rel="noreferrer"
                    title={portTitle(port)}
                  >
                    {content()}
                  </a>
                }
              </Show>
            );
          }}</For>
          <Show when={!loadingPorts() && displayedOpenPorts().length === 0}>
            <span class="host-port-empty">No open ports recorded yet</span>
          </Show>
        </div>
      </div>
    </div>
  );
}

export default Ping;
