import { apiPath } from "./api";

export type CorrelationConfidence = "low" | "medium" | "high" | string;
export type CorrelationDecision = "confirmed" | "rejected";

export type CorrelationReason = {
  code: string;
  detail: string;
  weight: number;
};

export type HostIdentityCandidate = {
  mac: string;
  hostId: number;
  name: string;
  deviceType: string;
  exists: boolean;
  active: boolean;
  score: number;
  confidence: CorrelationConfidence;
  reasons: CorrelationReason[];
};

export type HostIdentityCandidates = {
  mac: string;
  candidates: HostIdentityCandidate[];
};

export type IdentityCorrelationDecision = {
  mac: string;
  decision: CorrelationDecision;
  createdAt: string;
  updatedAt: string;
  hostId: number;
  name: string;
  deviceType: string;
  exists: boolean;
  active: boolean;
};

export type HostIdentityDecisions = {
  mac: string;
  decisions: IdentityCorrelationDecision[];
};

export type ConfirmedIdentityGroupMember = {
  mac: string;
  hostId: number;
  name: string;
  deviceType: string;
  exists: boolean;
  active: boolean;
  addresses: string[];
  firstSeen: string;
  lastSeen: string;
};

export type HostIdentityGroup = {
  mac: string;
  confirmed: boolean;
  members: ConfirmedIdentityGroupMember[];
};

async function correlationFetch(url: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(url, init);
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(detail || response.statusText || "Correlation request failed");
  }
  return response;
}

export async function apiGetHostIdentityCandidates(id: number | string): Promise<HostIdentityCandidates> {
  const url = apiPath + "/api/host/" + encodeURIComponent(String(id)) + "/identity/candidates";
  return await (await correlationFetch(url)).json() as HostIdentityCandidates;
}

export async function apiGetHostIdentityDecisions(id: number | string): Promise<HostIdentityDecisions> {
  const url = apiPath + "/api/host/" + encodeURIComponent(String(id)) + "/identity/decisions";
  return await (await correlationFetch(url)).json() as HostIdentityDecisions;
}

export async function apiGetHostIdentityGroup(id: number | string): Promise<HostIdentityGroup> {
  const url = apiPath + "/api/host/" + encodeURIComponent(String(id)) + "/identity/group";
  return await (await correlationFetch(url)).json() as HostIdentityGroup;
}

export async function apiSetHostIdentityDecision(
  id: number | string,
  mac: string,
  decision: CorrelationDecision,
): Promise<IdentityCorrelationDecision> {
  const url = apiPath + "/api/host/" + encodeURIComponent(String(id)) + "/identity/decisions/" + encodeURIComponent(mac);
  return await (await correlationFetch(url, {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ decision }),
  })).json() as IdentityCorrelationDecision;
}

export async function apiClearHostIdentityDecision(id: number | string, mac: string): Promise<void> {
  const url = apiPath + "/api/host/" + encodeURIComponent(String(id)) + "/identity/decisions/" + encodeURIComponent(mac);
  await correlationFetch(url, { method: "DELETE" });
}
