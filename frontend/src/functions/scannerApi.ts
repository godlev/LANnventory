import { createSignal } from "solid-js";

export type ScannerStatusValue = "healthy" | "problem" | "scanning";

export interface ScannerError {
  source: string;
  command: string;
  kind: string;
  message: string;
  output?: string;
}

export interface ScannerDatabaseStatus {
  status: string;
  backend?: string;
  error?: string;
}

export interface ScannerStatus {
  status: ScannerStatusValue;
  scanning: boolean;
  lastScanStartedAt: string | null;
  lastScanAt: string | null;
  lastSuccessfulScanAt: string | null;
  durationMs: number;
  devicesFound: number;
  interfaces: string[];
  lastError: ScannerError | null;
  nextScanAt: string | null;
  serverTime: string;
  database: ScannerDatabaseStatus;
}

export type DiagnosticsStatus = "ok" | "warning" | "error";

export interface DiagnosticsItem {
  check: string;
  status: DiagnosticsStatus;
  details: string;
  suggestedFix?: string;
}

export interface DiagnosticsReport {
  ok: boolean;
  generatedAt: string;
  checks: DiagnosticsItem[];
}

const apiJSON = async <T>(url: string): Promise<T> => {
  const response = await fetch(url);
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(detail || response.statusText || "API request failed");
  }
  return await response.json() as T;
};

export const [sharedScannerStatus, setSharedScannerStatus] = createSignal<ScannerStatus>();

export const refreshSharedScannerStatus = async (): Promise<ScannerStatus> => {
  const status = await apiJSON<ScannerStatus>("/api/scanner/status");
  setSharedScannerStatus(status);
  return status;
};

export const apiGetDiagnostics = async (): Promise<DiagnosticsReport> => {
  return await apiJSON<DiagnosticsReport>("/api/diagnostics");
};

export const scannerClockOffsetMs = (status: ScannerStatus, localNowMs = Date.now()): number => {
  const serverMs = Date.parse(status.serverTime);
  if (!Number.isFinite(serverMs)) {
    return 0;
  }
  return serverMs - localNowMs;
};

export const scannerCountdownLabel = (
  status: ScannerStatus,
  clockOffsetMs: number,
  localNowMs = Date.now(),
): string => {
  if (status.scanning || status.status === "scanning") {
    return "Scanning";
  }
  if (status.status === "problem") {
    return "Scanner problem";
  }
  if (!status.nextScanAt) {
    return "Scanner ready";
  }

  const nextMs = Date.parse(status.nextScanAt);
  if (!Number.isFinite(nextMs)) {
    return "Scanner ready";
  }

  const remainingMs = nextMs - (localNowMs + clockOffsetMs);
  if (remainingMs <= 0) {
    return "Next scan now";
  }

  const totalSeconds = Math.ceil(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  const clock = String(minutes).padStart(2, "0") + ":" + String(seconds).padStart(2, "0");
  return "Next scan " + clock;
};
