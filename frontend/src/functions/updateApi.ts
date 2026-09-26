import { createSignal } from "solid-js";

export type UpdateChannel = "stable" | "beta";

export type UpdateStatus = {
  currentVersion: string;
  channel: UpdateChannel;
  latestVersion: string;
  available: boolean;
  publishedAt: string;
  releaseUrl: string;
  releaseSummary: string;
  installSupported: boolean;
  installReason: string;
  message: string;
  updating: boolean;
  automaticCheck: boolean;
  automatic: boolean;
  intervalHours: number;
  lastChecked: string;
  snapshotBaseVersion: string;
};

export type UpdateProgressStatus = "idle" | "running" | "complete" | "failed";
export type UpdateProgressStage =
  | "idle"
  | "preparing"
  | "downloading"
  | "verifying"
  | "backup"
  | "installing"
  | "restarting"
  | "health-check"
  | "complete";

export type UpdateProgress = {
  attemptId: string;
  status: UpdateProgressStatus;
  stage: UpdateProgressStage;
  previousVersion: string;
  targetVersion: string;
  startedAt: string;
  updatedAt: string;
  completedAt: string;
  backupPath: string;
  backupCreated: boolean;
  failedStage: string;
  error: string;
  serviceRestored: boolean;
  systemdUnit: string;
};

export type UpdateApplyResult = {
  version: string;
  scheduled: boolean;
  message: string;
  backupPath: string;
};

export const [sharedUpdateStatus, setSharedUpdateStatus] = createSignal<UpdateStatus>();
export const [sharedUpdateProgress, setSharedUpdateProgress] = createSignal<UpdateProgress>();

const publishUpdateStatus = (status: UpdateStatus): UpdateStatus => {
  setSharedUpdateStatus(status);
  return status;
};

const publishUpdateProgress = (progress: UpdateProgress): UpdateProgress => {
  setSharedUpdateProgress(progress);
  return progress;
};

const apiJSON = async <T>(url: string, init?: RequestInit): Promise<T> => {
  const response = await fetch(url, init);
  if (!response.ok) {
    let detail = "";
    try {
      const payload = await response.json() as { error?: string; detail?: string };
      detail = [payload.error, payload.detail].filter(Boolean).join(": ");
    } catch {
      detail = await response.text();
    }
    throw new Error(detail || response.statusText || "Update request failed");
  }
  return await response.json() as T;
};

export const apiGetUpdateStatus = async (refresh = false): Promise<UpdateStatus> => {
  const suffix = refresh ? "?refresh=1" : "";
  return publishUpdateStatus(await apiJSON<UpdateStatus>("/api/update/status" + suffix));
};

export const apiGetCachedUpdateStatus = async (): Promise<UpdateStatus> => {
  return publishUpdateStatus(await apiJSON<UpdateStatus>("/api/update/status?cached=1"));
};

export const apiGetUpdateProgress = async (): Promise<UpdateProgress> => {
  return publishUpdateProgress(await apiJSON<UpdateProgress>("/api/update/progress"));
};

export const apiSetUpdateSettings = async (
  channel: UpdateChannel,
  automaticCheck: boolean,
  automatic: boolean,
  intervalHours: number,
): Promise<UpdateStatus> => {
  return publishUpdateStatus(await apiJSON<UpdateStatus>("/api/update/settings", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ channel, automaticCheck, automatic, intervalHours }),
  }));
};

export const apiApplyUpdate = async (version: string): Promise<UpdateApplyResult> => {
  return await apiJSON<UpdateApplyResult>("/api/update/apply", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ version }),
  });
};
