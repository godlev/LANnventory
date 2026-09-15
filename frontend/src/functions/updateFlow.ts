import { apiApplyUpdate, apiGetUpdateStatus, type UpdateStatus } from "./updateApi";

const reconnectIntervalMs = 2000;
const reconnectTimeoutMs = 90000;

export type UpdateFlowCallbacks = {
  onStatus?: (status: UpdateStatus) => void;
  onMessage?: (message: string) => void;
  onError?: (message: string) => void;
  onInstalling?: (installing: boolean) => void;
};

export function confirmUpdate(status: UpdateStatus): boolean {
  return window.confirm(
    "Update LANnventory from " + status.currentVersion + " to " + status.latestVersion + "?\n\n" +
    "LANnventory will verify the release package, install it, and restart the service. Existing configuration and database files will be preserved.",
  );
}

export async function startUpdateFlow(status: UpdateStatus, callbacks: UpdateFlowCallbacks = {}): Promise<number | undefined> {
  if (!status.available || !status.installSupported || status.updating) {
    return undefined;
  }

  callbacks.onInstalling?.(true);
  callbacks.onError?.("");
  callbacks.onMessage?.("Verifying and scheduling update...");

  try {
    const result = await apiApplyUpdate(status.latestVersion);
    const backupNote = result.backupPath ? " Recovery backup: " + result.backupPath + "." : "";
    callbacks.onMessage?.(result.message + backupNote + " Waiting for LANnventory to come back online...");
    return pollAfterUpdate(result.version, callbacks);
  } catch (err) {
    callbacks.onInstalling?.(false);
    callbacks.onMessage?.("");
    callbacks.onError?.(err instanceof Error ? err.message : "Update could not be scheduled");
    return undefined;
  }
}

function pollAfterUpdate(targetVersion: string, callbacks: UpdateFlowCallbacks): number {
  const startedAt = Date.now();
  let timer: number | undefined;

  const poll = async () => {
    if (Date.now() - startedAt > reconnectTimeoutMs) {
      callbacks.onInstalling?.(false);
      callbacks.onError?.("LANnventory did not come back online within 90 seconds. Check the service status on the host.");
      return;
    }

    try {
      const nextStatus = await apiGetUpdateStatus(true);
      callbacks.onStatus?.(nextStatus);
      if (nextStatus.currentVersion === targetVersion || !nextStatus.available) {
        callbacks.onMessage?.("Update completed. Reloading LANnventory...");
        window.setTimeout(() => window.location.reload(), 700);
        return;
      }
    } catch {
      // A failed request is expected while the service is restarting.
    }

    timer = window.setTimeout(poll, reconnectIntervalMs);
  };

  timer = window.setTimeout(poll, reconnectIntervalMs);
  return timer;
}
