import { apiGetVersion } from "./api";
import {
  apiApplyUpdate,
  apiGetUpdateProgress,
  apiGetUpdateStatus,
  type UpdateProgress,
  type UpdateStatus,
} from "./updateApi";

const progressPollIntervalMs = 1000;
const updateFlowTimeoutMs = 5 * 60 * 1000;
const attemptClockToleranceMs = 5000;

export type UpdateFlowHandle = {
  cancel: () => void;
};

export type UpdateFlowCallbacks = {
  onStatus?: (status: UpdateStatus) => void;
  onProgress?: (progress: UpdateProgress) => void;
  onMessage?: (message: string) => void;
  onError?: (message: string) => void;
  onInstalling?: (installing: boolean) => void;
  onComplete?: (progress: UpdateProgress) => void;
};

export function confirmUpdate(status: UpdateStatus): boolean {
  return window.confirm(
    "Update LANnventory from " + status.currentVersion + " to " + status.latestVersion + "?\n\n" +
    "LANnventory will verify the release package, create a recovery backup, install it, restart the service, and verify health. Existing configuration and database files will be preserved.",
  );
}

export async function startUpdateFlow(
  status: UpdateStatus,
  callbacks: UpdateFlowCallbacks = {},
): Promise<UpdateFlowHandle | undefined> {
  if (!status.available || !status.installSupported || status.updating) {
    return undefined;
  }

  callbacks.onInstalling?.(true);
  callbacks.onError?.("");
  callbacks.onMessage?.("Preparing update…");

  const startedAt = Date.now();
  const handle = watchUpdateProgress(status.latestVersion, startedAt, callbacks);

  try {
    const result = await apiApplyUpdate(status.latestVersion);
    callbacks.onMessage?.(
      result.backupPath
        ? "Update scheduled. Recovery backup path: " + result.backupPath + "."
        : "Update scheduled. LANnventory is tracking installation progress.",
    );
    return handle;
  } catch (err) {
    try {
      const progress = await apiGetUpdateProgress();
      if (progress.status === "running" && sameVersion(progress.targetVersion, status.latestVersion)) {
        callbacks.onProgress?.(progress);
        callbacks.onMessage?.(stageMessage(progress));
        return handle;
      }
    } catch {
      // If the backend disappeared before replying, the progress watcher will handle
      // a genuine restart only when it has already observed the matching attempt.
    }

    handle.cancel();
    callbacks.onInstalling?.(false);
    callbacks.onMessage?.("");
    callbacks.onError?.(err instanceof Error ? err.message : "Update could not be scheduled");
    return undefined;
  }
}

export function resumeUpdateFlow(
  progress: UpdateProgress,
  callbacks: UpdateFlowCallbacks = {},
): UpdateFlowHandle | undefined {
  if (progress.status !== "running") {
    return undefined;
  }

  callbacks.onInstalling?.(true);
  callbacks.onError?.("");
  callbacks.onProgress?.(progress);
  callbacks.onMessage?.(stageMessage(progress));

  const startedAt = parseTime(progress.startedAt) ?? Date.now();
  return watchUpdateProgress(progress.targetVersion, startedAt, callbacks, progress.attemptId);
}

function watchUpdateProgress(
  targetVersion: string,
  startedAt: number,
  callbacks: UpdateFlowCallbacks,
  initialAttemptID = "",
): UpdateFlowHandle {
  let cancelled = false;
  let timer: number | undefined;
  let activeAttemptID = initialAttemptID;
  let lastProgress: UpdateProgress | undefined;

  const handle: UpdateFlowHandle = {
    cancel: () => {
      cancelled = true;
      if (timer !== undefined) {
        window.clearTimeout(timer);
        timer = undefined;
      }
    },
  };

  const scheduleNext = () => {
    if (!cancelled) {
      timer = window.setTimeout(() => void poll(), progressPollIntervalMs);
    }
  };

  const finishWithError = (message: string) => {
    handle.cancel();
    callbacks.onInstalling?.(false);
    callbacks.onMessage?.("");
    callbacks.onError?.(message);
  };

  const poll = async () => {
    if (cancelled) {
      return;
    }
    if (Date.now() - startedAt > updateFlowTimeoutMs) {
      finishWithError("Update progress timed out. Check LANnventory service status and the persisted update state before retrying.");
      return;
    }

    try {
      const progress = await apiGetUpdateProgress();
      if (matchesAttempt(progress, targetVersion, startedAt, activeAttemptID)) {
        if (!activeAttemptID && progress.attemptId) {
          activeAttemptID = progress.attemptId;
        }
        lastProgress = progress;
        callbacks.onProgress?.(progress);

        if (progress.status === "failed") {
          finishWithError(progress.error || "LANnventory update failed.");
          return;
        }

        if (progress.status === "complete") {
          try {
            const installedVersion = await apiGetVersion();
            if (targetVersion && !sameVersion(installedVersion, targetVersion)) {
              finishWithError(
                "LANnventory came back online, but the installed version is " +
                installedVersion + " instead of " + targetVersion + ".",
              );
              return;
            }

            try {
              callbacks.onStatus?.(await apiGetUpdateStatus(false));
            } catch {
              // Version verification is authoritative for completion. Release metadata
              // can be refreshed separately if its cached state was reset by restart.
            }

            handle.cancel();
            callbacks.onInstalling?.(false);
            callbacks.onMessage?.("Update complete.");
            callbacks.onError?.("");
            callbacks.onComplete?.(progress);
            return;
          } catch {
            callbacks.onMessage?.("LANnventory reports update completion; waiting for the restarted API to become reachable…");
            scheduleNext();
            return;
          }
        }

        callbacks.onMessage?.(stageMessage(progress));
      }

      scheduleNext();
    } catch {
      if (activeAttemptID || lastProgress?.status === "running") {
        callbacks.onMessage?.(disconnectMessage(lastProgress));
      }
      scheduleNext();
    }
  };

  void poll();
  return handle;
}

function matchesAttempt(
  progress: UpdateProgress,
  targetVersion: string,
  flowStartedAt: number,
  activeAttemptID: string,
): boolean {
  if (activeAttemptID) {
    return progress.attemptId === activeAttemptID;
  }
  if (progress.status === "idle") {
    return false;
  }
  if (targetVersion && !sameVersion(progress.targetVersion, targetVersion)) {
    return false;
  }

  const attemptStartedAt = parseTime(progress.startedAt);
  return attemptStartedAt !== null && attemptStartedAt >= flowStartedAt - attemptClockToleranceMs;
}

export function stageMessage(progress: UpdateProgress): string {
  switch (progress.stage) {
    case "preparing":
      return "Preparing update…";
    case "downloading":
      return "Downloading package…";
    case "verifying":
      return "Verifying package…";
    case "backup":
      return "Creating recovery backup…";
    case "installing":
      return "Installing package…";
    case "restarting":
      return "Restarting LANnventory…";
    case "health-check":
      return "Verifying LANnventory health…";
    case "complete":
      return "Update complete.";
    default:
      return progress.status === "running" ? "Updating LANnventory…" : "";
  }
}

function disconnectMessage(progress?: UpdateProgress): string {
  if (progress?.stage === "restarting") {
    return "Restarting LANnventory… temporary connection loss is expected.";
  }
  if (progress?.stage === "health-check") {
    return "LANnventory restarted; waiting for the health check to complete…";
  }
  return "LANnventory is temporarily unreachable while the update continues…";
}

function parseTime(value: string): number | null {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function sameVersion(left: string, right: string): boolean {
  return left.trim().replace(/^v/, "") === right.trim().replace(/^v/, "");
}
