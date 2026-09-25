import { createMemo, createSignal, onCleanup, onMount, Show } from "solid-js";
import {
  apiGetUpdateProgress,
  apiGetUpdateStatus,
  apiSetUpdateSettings,
  type UpdateChannel,
  type UpdateProgress,
  type UpdateStatus,
} from "../../functions/updateApi";
import {
  confirmUpdate,
  resumeUpdateFlow,
  startUpdateFlow,
  type UpdateFlowCallbacks,
  type UpdateFlowHandle,
} from "../../functions/updateFlow";
import ReleaseNotesDialog from "../ReleaseNotesDialog";
import UpdateProgressPanel from "./UpdateProgressPanel";

const updateIntervals = [
  { label: "6 hours", value: 6 },
  { label: "12 hours", value: 12 },
  { label: "24 hours", value: 24 },
  { label: "7 days", value: 168 },
];

const dismissedCompleteKey = "lannventory-update-complete-dismissed";

function Updates() {
  const [status, setStatus] = createSignal<UpdateStatus>();
  const [progress, setProgress] = createSignal<UpdateProgress>();
  const [showProgress, setShowProgress] = createSignal(false);
  const [loading, setLoading] = createSignal(true);
  const [savingSettings, setSavingSettings] = createSignal(false);
  const [installing, setInstalling] = createSignal(false);
  const [error, setError] = createSignal("");
  const [message, setMessage] = createSignal("");
  const [releaseNotesOpen, setReleaseNotesOpen] = createSignal(false);
  let updateFlow: UpdateFlowHandle | undefined;

  const current = () => status();
  const channel = () => current()?.channel ?? "beta";
  const automatic = () => current()?.automatic ?? false;
  const automaticCheck = () => current()?.automaticCheck ?? automatic();
  const intervalHours = () => current()?.intervalHours ?? 24;
  const channelLabel = () => channel() === "stable" ? "Stable" : "Beta";
  const latestLabel = () => "Latest published " + channelLabel();
  const updateBusy = () => installing() || progress()?.status === "running";
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

  const clearUpdateFlow = () => {
    updateFlow?.cancel();
    updateFlow = undefined;
  };

  const isDismissedComplete = (next: UpdateProgress) => {
    if (next.status !== "complete" || !next.attemptId) {
      return false;
    }
    try {
      return window.sessionStorage.getItem(dismissedCompleteKey) === next.attemptId;
    } catch {
      return false;
    }
  };

  const publishProgress = (next: UpdateProgress) => {
    setProgress(next);
    if (next.status === "idle" || isDismissedComplete(next)) {
      return;
    }
    setShowProgress(true);
  };

  const flowCallbacks = (): UpdateFlowCallbacks => ({
    onStatus: setStatus,
    onProgress: publishProgress,
    onMessage: setMessage,
    onError: setError,
    onInstalling: setInstalling,
    onComplete: publishProgress,
  });

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

  const loadInitialState = async () => {
    setLoading(true);
    setError("");

    try {
      setStatus(await apiGetUpdateStatus(false));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update status could not be loaded");
    }

    try {
      const nextProgress = await apiGetUpdateProgress();
      publishProgress(nextProgress);
      if (nextProgress.status === "running") {
        clearUpdateFlow();
        updateFlow = resumeUpdateFlow(nextProgress, flowCallbacks());
      }
    } catch (err) {
      if (!error()) {
        setError(err instanceof Error ? err.message : "Update progress could not be loaded");
      }
    } finally {
      setLoading(false);
    }
  };

  const saveSettings = async (
    nextChannel: UpdateChannel,
    nextAutomaticCheck: boolean,
    nextAutomatic: boolean,
    nextIntervalHours: number,
  ) => {
    if (savingSettings() || updateBusy()) {
      return;
    }

    setSavingSettings(true);
    setError("");
    setMessage("");
    try {
      setStatus(await apiSetUpdateSettings(nextChannel, nextAutomaticCheck || nextAutomatic, nextAutomatic, nextIntervalHours));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update settings could not be saved");
    } finally {
      setSavingSettings(false);
    }
  };

  const handleInstall = async () => {
    const currentStatus = current();
    if (!currentStatus?.available || !currentStatus.installSupported || updateBusy()) {
      return;
    }
    if (!confirmUpdate(currentStatus)) {
      return;
    }

    clearUpdateFlow();
    try {
      window.sessionStorage.removeItem(dismissedCompleteKey);
    } catch {
      // Session storage is optional; progress remains authoritative without it.
    }
    setProgress(undefined);
    setShowProgress(true);
    setError("");
    setMessage("Preparing update…");

    updateFlow = await startUpdateFlow(currentStatus, flowCallbacks());
  };

  const handleProgressClose = () => {
    const currentProgress = progress();
    if (currentProgress?.status === "complete" && currentProgress.attemptId) {
      try {
        window.sessionStorage.setItem(dismissedCompleteKey, currentProgress.attemptId);
      } catch {
        // The reload still works when session storage is unavailable.
      }
      window.location.reload();
      return;
    }
    setShowProgress(false);
  };

  onMount(() => void loadInitialState());
  onCleanup(clearUpdateFlow);

  return (
    <div class="card wyl-panel config-panel update-panel">
      <div class="card-header update-panel-header">
        <div class="update-panel-title">Updates</div>
        <Show when={status()}>
          {(currentStatus) =>
            <span
              class={
                "update-state-badge " +
                (updateBusy() ? "is-updating" : currentStatus().available ? "is-available" : "is-current")
              }
            >
              {updateBusy() ? "Updating" : currentStatus().available ? "Update available" : "Up to date"}
            </span>
          }
        </Show>
      </div>

      <div class="card-body update-panel-body">
        <div class="settings-behavior-note">
          <i class="bi bi-lightning-charge-fill" aria-hidden="true"></i>
          <span>Update preferences save immediately. Installing a release still requires explicit confirmation.</span>
        </div>

        <Show when={showProgress() && progress()}>
          {(currentProgress) =>
            <UpdateProgressPanel
              progress={currentProgress()}
              retryEnabled={Boolean(status()?.available && status()?.installSupported && !updateBusy())}
              onRetry={() => void handleInstall()}
              onClose={handleProgressClose}
            />
          }
        </Show>

        <Show when={showProgress() && !progress() && message()}>
          <div class="update-progress-message" role="status">{message()}</div>
        </Show>

        <label class="update-field">
          <span class="update-field-label">Update channel</span>
          <select
            class="form-select form-select-sm update-select"
            value={channel()}
            disabled={loading() || savingSettings() || updateBusy()}
            onChange={(event) => void saveSettings(event.currentTarget.value as UpdateChannel, automaticCheck(), automatic(), intervalHours())}
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

        <div class="update-automation-options">
          <label class="form-check form-switch update-auto-toggle">
            <input
              class="form-check-input"
              type="checkbox"
              checked={automaticCheck()}
              disabled={loading() || savingSettings() || updateBusy()}
              onChange={(event) => {
                const enabled = event.currentTarget.checked;
                void saveSettings(channel(), enabled, enabled ? automatic() : false, intervalHours());
              }}
            />
            <span class="form-check-label">Check automatically</span>
          </label>

          <label class="form-check form-switch update-auto-toggle">
            <input
              class="form-check-input"
              type="checkbox"
              checked={automatic()}
              disabled={loading() || savingSettings() || updateBusy() || !automaticCheck()}
              onChange={(event) => void saveSettings(channel(), true, event.currentTarget.checked, intervalHours())}
            />
            <span class="form-check-label">Install updates automatically</span>
          </label>
        </div>

        <Show when={automaticCheck()}>
          <label class="update-field">
            <span class="update-field-label">Check for updates</span>
            <select
              class="form-select form-select-sm update-select"
              value={String(intervalHours())}
              disabled={loading() || savingSettings() || updateBusy()}
              onChange={(event) => void saveSettings(channel(), automaticCheck(), automatic(), Number(event.currentTarget.value))}
            >
              {updateIntervals.map((interval) =>
                <option value={interval.value}>{interval.label}</option>
              )}
            </select>
          </label>
        </Show>

        <Show when={automaticCheck() && !automatic()}>
          <div class="update-support-note">
            <i class="bi bi-bell" aria-hidden="true"></i>
            <span>LANnventory will check in the background and show an update reminder in the top bar, but it will not install anything automatically.</span>
          </div>
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
        <Show when={message() && (!showProgress() || !progress())}>
          <div class="update-progress-message" role="status">{message()}</div>
        </Show>
        <Show when={error() && !(showProgress() && progress()?.status === "failed")}>
          <div class="config-save-error" role="alert">{error()}</div>
        </Show>

        <div class="update-actions">
          <button
            type="button"
            class="btn btn-sm wyl-button"
            disabled={loading() || savingSettings() || updateBusy()}
            onClick={() => void loadStatus(true)}
          >
            <i class="bi bi-arrow-clockwise" aria-hidden="true"></i>
            <span>{loading() ? "Checking" : "Check now"}</span>
          </button>
          <Show when={status()?.available && status()!.installSupported}>
            <button
              type="button"
              class="btn btn-sm wyl-button update-install-button"
              disabled={updateBusy() || savingSettings()}
              onClick={() => void handleInstall()}
            >
              <i class={updateBusy() ? "bi bi-hourglass-split" : "bi bi-download"} aria-hidden="true"></i>
              <span>{updateBusy() ? "Updating" : "Update"}</span>
            </button>
          </Show>
        </div>

        <Show when={status()?.releaseUrl}>
          <button
            type="button"
            class="update-release-link"
            onClick={() => setReleaseNotesOpen(true)}
          >
            <span>Release notes</span>
            <i class="bi bi-card-text" aria-hidden="true"></i>
          </button>
        </Show>
      </div>

      <ReleaseNotesDialog
        open={releaseNotesOpen()}
        version={status()?.latestVersion || status()?.currentVersion || ""}
        summary={status()?.releaseSummary || ""}
        releaseUrl={status()?.releaseUrl || ""}
        onClose={() => setReleaseNotesOpen(false)}
      />
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
