export type UpdateChannel = "stable" | "beta";

export type UpdateStatus = {
  currentVersion: string;
  channel: UpdateChannel;
  latestVersion: string;
  available: boolean;
  publishedAt: string;
  releaseUrl: string;
  installSupported: boolean;
  installReason: string;
  message: string;
  updating: boolean;
  automatic: boolean;
  intervalHours: number;
  lastChecked: string;
  snapshotBaseVersion: string;
};

export type UpdateApplyResult = {
  version: string;
  scheduled: boolean;
  message: string;
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
  return await apiJSON<UpdateStatus>("/api/update/status" + suffix);
};

export const apiSetUpdateSettings = async (
  channel: UpdateChannel,
  automatic: boolean,
  intervalHours: number,
): Promise<UpdateStatus> => {
  return await apiJSON<UpdateStatus>("/api/update/settings", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ channel, automatic, intervalHours }),
  });
};

export const apiApplyUpdate = async (version: string): Promise<UpdateApplyResult> => {
  return await apiJSON<UpdateApplyResult>("/api/update/apply", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ version }),
  });
};
