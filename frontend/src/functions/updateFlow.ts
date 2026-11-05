import { apiGetVersion } from "./api";
import {
  apiApplyUpdate,
  apiGetUpdateProgress,
  apiGetUpdateStatus,
  UpdateHTTPError,
  type UpdateProgress,
  type UpdateStatus,
} from "./updateApi";

const progressPollIntervalMs = 1000;
const updateFlowTimeoutMs = 5 * 60 * 1000;
let updateSubmissionPending = false;

export type UpdateFlowHandle = {
  cancel: () => void;
  trackAttempt: (attemptID: string) => void;
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
  if (!status.available || !status.installSupported || status.updating || updateSubmissionPending) {
    return undefined;
  }

  updateSubmissionPending = true;
  callbacks.onInstalling?.(true);
  callbacks.onError?.("");
  callbacks.onMessage?.("Preparing update…");

  let baselineAttemptID = "";
  let baselineKnown = false;
  try {
    const baseline = await apiGetUpdateProgress();
    baselineAttemptID = baseline.attemptId;
    baselineKnown = true;
  } catch {
    // Scheduling can still proceed. Without a baseline, the watcher only adopts
    // a running attempt until the scheduling response supplies its attempt ID.
  }

  let observedProgress = false;
  const trackedCallbacks: UpdateFlowCallbacks = {
    ...callbacks,
    onProgress: (progress) => {
      observedProgress = true;
      callbacks.onProgress?.(progress);
    },
  };
  const handle = watchUpdateProgress(
    status.latestVersion,
    trackedCallbacks,
    "",
    baselineAttemptID,
    baselineKnown,
  );

  try {
    const result = await apiApplyUpdate(status.latestVersion);
    handle.trackAttempt(result.attemptId ?? "");
    if (!observedProgress) {
      callbacks.onMessage?.(
        result.backupPath
          ? "Update scheduled. Recovery backup path: " + result.backupPath + "."
          : "Update scheduled. LANnventory is tracking installation progress.",
      );
    }
    return handle;
  } catch (err) {
    if (!(err instanceof UpdateHTTPError)) {
      callbacks.onMessage?.("LANnventory is temporarily unreachable while update scheduling is confirmed…");
      return handle;
    }

    handle.cancel();
    callbacks.onInstalling?.(false);
    callbacks.onMessage?.("");
    callbacks.onError?.(err instanceof Error ? err.message : "Update could not be scheduled");
    return undefined;
  } finally {
    updateSubmissionPending = false;
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

  return watchUpdateProgress(progress.targetVersion, callbacks, progress.attemptId);
}

function watchUpdateProgress(
  targetVersion: string,
  callbacks: UpdateFlowCallbacks,
  initialAttemptID = "",
  baselineAttemptID = "",
  baselineKnown = false,
): UpdateFlowHandle {
  let cancelled = false;
  let timer: number | undefined;
  let activeAttemptID = initialAttemptID;
  let lastProgress: UpdateProgress | undefined;
  const watcherStartedAt = Date.now();

  const handle: UpdateFlowHandle = {
    cancel: () => {
      cancelled = true;
      if (timer !== undefined) {
        window.clearTimeout(timer);
        timer = undefined;
      }
    },
    trackAttempt: (attemptID: string) => {
      if (attemptID) {
        activeAttemptID = attemptID;
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
    if (Date.now() - watcherStartedAt > updateFlowTimeoutMs) {
      finishWithError("Update progress timed out. Check LANnventory service status and the persisted update state before retrying.");
      return;
    }

    try {
      const progress = await apiGetUpdateProgress();
      if (matchesAttempt(progress, targetVersion, activeAttemptID, baselineAttemptID, baselineKnown)) {
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
  activeAttemptID: string,
  baselineAttemptID: string,
  baselineKnown: boolean,
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
  if (!progress.attemptId) {
    return false;
  }
  if (baselineKnown) {
    return progress.attemptId !== baselineAttemptID;
  }
  return progress.status === "running";
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

function sameVersion(left: string, right: string): boolean {
  return left.trim().replace(/^v/, "") === right.trim().replace(/^v/, "");
}
