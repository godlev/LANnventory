import { createSignal, onCleanup, onMount, Show } from "solid-js";
import {
  apiApplyUpdate,
  apiGetUpdateStatus,
  apiSetUpdateChannel,
  type UpdateChannel,
  type UpdateStatus,
} from "../../functions/updateApi";

const reconnectIntervalMs = 2000;
const reconnectTimeoutMs = 90000;

function Updates() {
  const [status, setStatus] = createSignal<UpdateStatus>();
  const [loading, setLoading] = createSignal(true);
  const [savingChannel, setSavingChannel] = createSignal(false);
  const [installing, setInstalling] = createSignal(false);
  const [error, setError] = createSignal("");
  const [message, setMessage] = createSignal("");
  let reconnectTimer: number | undefined;
  let reconnectStartedAt = 0;

  const clearReconnectTimer = () => {
    if (reconnectTimer !== undefined) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = undefined;
    }
  };

  const loadStatus = async (refresh = false) => {
    setLoading(true);
    setError("");
    try {
      setStatus(await apiGetUpdateStatus(refresh));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update status could not be loaded");
    } finally {
      setLoading(false);
    }
  };

  const pollAfterUpdate = async (targetVersion: string) => {
    clearReconnectTimer();
    reconnectStartedAt = Date.now();

    const poll = async () => {
      if (Date.now() - reconnectStartedAt > reconnectTimeoutMs) {
        setInstalling(false);
        setError("LANnventory did not come back online within 90 seconds. Check the service status on the host.");
        return;
      }

      try {
        const nextStatus = await apiGetUpdateStatus(true);
        setStatus(nextStatus);
        if (nextStatus.currentVersion === targetVersion || !nextStatus.available) {
          setMessage("Update completed. Reloading LANnventory…");
          window.setTimeout(() => window.location.reload(), 700);
          return;
        }
      } catch {
        // A failed request is expected while the service is restarting.
      }

      reconnectTimer = window.setTimeout(poll, reconnectIntervalMs);
    };

    reconnectTimer = window.setTimeout(poll, reconnectIntervalMs);
  };

  const handleChannelChange = async (channel: UpdateChannel) => {
    if (savingChannel() || installing() || status()?.channel === channel) {
      return;
    }

    setSavingChannel(true);
    setError("");
    setMessage("");
    try {
      setStatus(await apiSetUpdateChannel(channel));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update channel could not be saved");
    } finally {
      setSavingChannel(false);
    }
  };

  const handleInstall = async () => {
    const current = status();
    if (!current?.available || !current.installSupported || installing()) {
      return;
    }

    const confirmed = window.confirm(
      "Update LANnventory from " + current.currentVersion + " to " + current.latestVersion + "?\n\n" +
      "LANnventory will verify the release package, install it, and restart the service. Existing configuration and database files will be preserved.",
    );
    if (!confirmed) {
      return;
    }

    setInstalling(true);
    setError("");
    setMessage("Verifying and scheduling update…");
    try {
      const result = await apiApplyUpdate(current.latestVersion);
      setMessage(result.message + " Waiting for LANnventory to come back online…");
      void pollAfterUpdate(result.version);
    } catch (err) {
      setInstalling(false);
      setMessage("");
      setError(err instanceof Error ? err.message : "Update could not be scheduled");
    }
  };

  onMount(() => void loadStatus(false));
  onCleanup(clearReconnectTimer);

  return (
    <div class="card wyl-panel config-panel update-panel">
      <div class="card-header update-panel-header">
        <div>
          <div>Updates</div>
          <div class="config-panel-subtitle">Stable or Beta release channel</div>
        </div>
        <Show when={status()}>
          {(current) =>
            <span class={"update-state-badge " + (current().available ? "is-available" : "is-current")}>
              {current().available ? "Update available" : "Up to date"}
            </span>
          }
        </Show>
      </div>

      <div class="card-body update-panel-body">
        <div class="update-channel-control" role="group" aria-label="Update channel">
          <button
            type="button"
            class={"btn btn-sm wyl-button update-channel-button" + (status()?.channel === "stable" ? " is-active" : "")}
            aria-pressed={status()?.channel === "stable"}
            disabled={loading() || savingChannel() || installing()}
            onClick={() => void handleChannelChange("stable")}
          >
            Stable
          </button>
          <button
            type="button"
            class={"btn btn-sm wyl-button update-channel-button" + (status()?.channel === "beta" ? " is-active" : "")}
            aria-pressed={status()?.channel === "beta"}
            disabled={loading() || savingChannel() || installing()}
            onClick={() => void handleChannelChange("beta")}
          >
            Beta
          </button>
        </div>

        <Show when={!loading()} fallback={<div class="update-status-message">Checking releases…</div>}>
          <Show when={status()}>
            {(current) =>
              <div class="update-version-grid">
                <span>Installed</span><strong>{current().currentVersion || "Unknown"}</strong>
                <span>Latest {current().channel}</span><strong>{current().latestVersion || "None published"}</strong>
              </div>
            }
          </Show>
        </Show>

        <Show when={status()?.message}>
          <div class="update-status-message">{status()?.message}</div>
        </Show>
        <Show when={status() && !status()!.installSupported && status()!.installReason}>
          <div class="update-support-note">
            <i class="bi bi-info-circle" aria-hidden="true"></i>
            <span>{status()!.installReason}</span>
          </div>
        </Show>
        <Show when={message()}>
          <div class="update-progress-message" role="status">{message()}</div>
        </Show>
        <Show when={error()}>
          <div class="config-save-error" role="alert">{error()}</div>
        </Show>

        <div class="update-actions">
          <button
            type="button"
            class="btn btn-sm wyl-button"
            disabled={loading() || savingChannel() || installing()}
            onClick={() => void loadStatus(true)}
          >
            <i class="bi bi-arrow-clockwise" aria-hidden="true"></i>
            <span>Check now</span>
          </button>
          <button
            type="button"
            class="btn btn-sm wyl-button update-install-button"
            disabled={!status()?.available || !status()?.installSupported || installing() || savingChannel()}
            onClick={() => void handleInstall()}
          >
            <i class={installing() ? "bi bi-hourglass-split" : "bi bi-download"} aria-hidden="true"></i>
            <span>{installing() ? "Updating" : "Update"}</span>
          </button>
        </div>

        <Show when={status()?.releaseUrl}>
          <a class="update-release-link" href={status()!.releaseUrl} target="_blank" rel="noreferrer">View selected release</a>
        </Show>
      </div>
    </div>
  );
}

export default Updates;
