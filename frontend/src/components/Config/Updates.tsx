import { createMemo, createSignal, onCleanup, onMount, Show } from "solid-js";
import {
  apiGetUpdateStatus,
  apiSetUpdateSettings,
  type UpdateChannel,
  type UpdateStatus,
} from "../../functions/updateApi";
import { confirmUpdate, startUpdateFlow } from "../../functions/updateFlow";

const updateIntervals = [
  { label: "6 hours", value: 6 },
  { label: "12 hours", value: 12 },
  { label: "24 hours", value: 24 },
  { label: "7 days", value: 168 },
];

function Updates() {
  const [status, setStatus] = createSignal<UpdateStatus>();
  const [loading, setLoading] = createSignal(true);
  const [savingSettings, setSavingSettings] = createSignal(false);
  const [installing, setInstalling] = createSignal(false);
  const [error, setError] = createSignal("");
  const [message, setMessage] = createSignal("");
  let reconnectTimer: number | undefined;

  const current = () => status();
  const channel = () => current()?.channel ?? "beta";
  const automatic = () => current()?.automatic ?? false;
  const intervalHours = () => current()?.intervalHours ?? 24;
  const channelLabel = () => channel() === "stable" ? "Stable" : "Beta";
  const latestLabel = () => "Latest published " + channelLabel();
  const statusMessage = createMemo(() => {
    const currentStatus = current();
    if (!currentStatus) {
      return "";
    }

    if (currentStatus.message) {
      return currentStatus.message;
    }
    if (!currentStatus.available) {
      return "No newer " + channelLabel() + " release is available.";
    }
    return "Update " + currentStatus.latestVersion + " is available.";
  });
  const snapshotMessage = createMemo(() => {
    const base = current()?.snapshotBaseVersion;
    return base ? "This bootstrap build is based on " + base + "." : "";
  });

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

  const saveSettings = async (nextChannel: UpdateChannel, nextAutomatic: boolean, nextIntervalHours: number) => {
    if (savingSettings() || installing()) {
      return;
    }

    setSavingSettings(true);
    setError("");
    setMessage("");
    try {
      setStatus(await apiSetUpdateSettings(nextChannel, nextAutomatic, nextIntervalHours));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update settings could not be saved");
    } finally {
      setSavingSettings(false);
    }
  };

  const handleInstall = async () => {
    const currentStatus = current();
    if (!currentStatus?.available || !currentStatus.installSupported || installing()) {
      return;
    }
    if (!confirmUpdate(currentStatus)) {
      return;
    }

    clearReconnectTimer();
    reconnectTimer = await startUpdateFlow(currentStatus, {
      onStatus: setStatus,
      onMessage: setMessage,
      onError: setError,
      onInstalling: setInstalling,
    });
  };

  onMount(() => void loadStatus(false));
  onCleanup(clearReconnectTimer);

  return (
    <div class="card wyl-panel config-panel update-panel">
      <div class="card-header update-panel-header">
        <div class="update-panel-title">Updates</div>
        <Show when={status()}>
          {(currentStatus) =>
            <span class={"update-state-badge " + (currentStatus().available ? "is-available" : "is-current")}>
              {currentStatus().available ? "Update available" : "No update"}
            </span>
          }
        </Show>
      </div>

      <div class="card-body update-panel-body">
        <label class="update-field">
          <span class="update-field-label">Update channel</span>
          <select
            class="form-select form-select-sm update-select"
            value={channel()}
            disabled={loading() || savingSettings() || installing()}
            onChange={(event) => void saveSettings(event.currentTarget.value as UpdateChannel, automatic(), intervalHours())}
          >
            <option value="stable">Stable</option>
            <option value="beta">Beta</option>
          </select>
        </label>

        <Show when={!loading()} fallback={<div class="update-status-message">Checking releases...</div>}>
          <Show when={status()}>
            {(currentStatus) =>
              <div class="update-version-stack">
                <div class="update-version-item">
                  <span>Installed version</span>
                  <strong>{currentStatus().currentVersion || "Unknown"}</strong>
                </div>
                <div class="update-version-item">
                  <span>{latestLabel()}</span>
                  <strong>{currentStatus().latestVersion || "None published"}</strong>
                </div>
              </div>
            }
          </Show>
        </Show>

        <label class="form-check form-switch update-auto-toggle">
          <input
            class="form-check-input"
            type="checkbox"
            checked={automatic()}
            disabled={loading() || savingSettings() || installing()}
            onChange={(event) => void saveSettings(channel(), event.currentTarget.checked, intervalHours())}
          />
          <span class="form-check-label">Automatic updates</span>
        </label>

        <Show when={automatic()}>
          <label class="update-field">
            <span class="update-field-label">Check for updates</span>
            <select
              class="form-select form-select-sm update-select"
              value={String(intervalHours())}
              disabled={loading() || savingSettings() || installing()}
              onChange={(event) => void saveSettings(channel(), automatic(), Number(event.currentTarget.value))}
            >
              {updateIntervals.map((interval) =>
                <option value={interval.value}>{interval.label}</option>
              )}
            </select>
          </label>
        </Show>

        <Show when={current()?.lastChecked}>
          <div class="update-meta-row">
            <span>Last checked</span>
            <strong>{formatUpdateTime(current()!.lastChecked)}</strong>
          </div>
        </Show>

        <Show when={statusMessage()}>
          <div class="update-status-message">{statusMessage()}</div>
        </Show>
        <Show when={snapshotMessage()}>
          <div class="update-support-note">
            <i class="bi bi-info-circle" aria-hidden="true"></i>
            <span>{snapshotMessage()}</span>
          </div>
        </Show>
        <Show when={status()?.available && !status()!.installSupported && status()!.installReason}>
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
            disabled={loading() || savingSettings() || installing()}
            onClick={() => void loadStatus(true)}
          >
            <i class="bi bi-arrow-clockwise" aria-hidden="true"></i>
            <span>{loading() ? "Checking" : "Check now"}</span>
          </button>
          <Show when={status()?.available && status()!.installSupported}>
            <button
              type="button"
              class="btn btn-sm wyl-button update-install-button"
              disabled={installing() || savingSettings()}
              onClick={() => void handleInstall()}
            >
              <i class={installing() ? "bi bi-hourglass-split" : "bi bi-download"} aria-hidden="true"></i>
              <span>{installing() ? "Updating" : "Update"}</span>
            </button>
          </Show>
        </div>

        <Show when={status()?.releaseUrl}>
          <a class="update-release-link" href={status()!.releaseUrl} target="_blank" rel="noreferrer">
            <span>Release notes</span>
            <i class="bi bi-box-arrow-up-right" aria-hidden="true"></i>
          </a>
        </Show>
      </div>
    </div>
  );
}

function formatUpdateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const day = String(date.getDate()).padStart(2, "0");
  const month = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"][date.getMonth()];
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return day + " " + month + " " + hour + ":" + minute;
}

export default Updates;
