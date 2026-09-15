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
};

export type UpdateApplyResult = {
  version: string;
  scheduled: boolean;
  message: string;
  backupPath: string;
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

export const apiSetUpdateChannel = async (channel: UpdateChannel): Promise<UpdateStatus> => {
  return await apiJSON<UpdateStatus>("/api/update/channel", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ channel }),
  });
};

export const apiApplyUpdate = async (version: string): Promise<UpdateApplyResult> => {
  return await apiJSON<UpdateApplyResult>("/api/update/apply", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ version }),
  });
};
