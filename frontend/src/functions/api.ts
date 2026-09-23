import { isDeviceTypeValue, type DeviceTypeValue } from "./deviceTypes";
import type { ActivityDeviceOption, ActivityStats, Conf, DeviceProfile, Host, HostEvent, HypervisorProfile, NetworkDeviceProfile, Service, SystemDeviceProfile } from "./exports";

export const apiPath = '';
export type ActivityCategory = "all" | "connectivity" | "changes";
export type ActivityEventType =
  | "discovered"
  | "online"
  | "offline"
  | "known"
  | "unknown"
  | "device-type-changed"
  | "owner-changed"
  | "location-changed"
  | "notes-changed"
  | "tags-changed"
  | "pinned-changed"
  | "service-opened"
  | "service-closed"
  | "port-open";

type ActivityQuery = {
  category?: ActivityCategory;
  eventTypes?: ActivityEventType[];
  offset?: number;
  beforeDate?: string;
  beforeId?: number;
  mac?: string;
  macs?: string[];
};

export type HostMetadataPayload = {
  owner?: string;
  location?: string;
  notes?: string;
  tags?: string[];
  pinned?: boolean;
};

export type HostInventoryPayload = {
  name?: string;
  known?: boolean;
  deviceType?: DeviceTypeValue;
  owner?: string;
  location?: string;
  notes?: string;
  tags?: string[];
};

export type InventoryOptions = {
  owners: string[];
  locations: string[];
};

export type AddressMACObservation = {
  mac: string;
  firstSeen: string;
  lastSeen: string;
  active: boolean;
};

export type HostIdentityAddress = {
  address: string;
  family: string;
  iface: string;
  firstSeen: string;
  lastSeen: string;
  active: boolean;
  macHistory: AddressMACObservation[];
};

export type DiscoveryEvidenceObservation = {
  address: string;
  source: string;
  kind: string;
  value: string;
  firstSeen: string;
  lastSeen: string;
  active: boolean;
};

export type HostIdentity = {
  mac: string;
  addresses: HostIdentityAddress[];
  evidence: DiscoveryEvidenceObservation[];
  dataSources: string[];
};

export type AddressIdentity = {
  address: string;
  macHistory: AddressMACObservation[];
};

export type DeviceProfileResponse = {
  managed: DeviceProfile | null;
  network: NetworkDeviceProfile | null;
  system: SystemDeviceProfile | null;
  hypervisor: HypervisorProfile | null;
};

export type DeviceProfilePayload = {
  manufacturer?: string;
  model?: string;
  managementAddress?: string;
};

export type NetworkDeviceProfilePayload = {
  managementMode?: "" | "managed" | "unmanaged";
  physicalPortCount?: number;
  portCapabilityNotes?: string;
};

export type SystemDeviceProfilePayload = {
  role?: string;
  operatingSystem?: string;
  version?: string;
};

export type HypervisorProfilePayload = {
  platform?: "proxmox-ve" | "vmware-esxi" | "hyper-v" | "other";
  version?: string;
  nodeName?: string;
  clusterName?: string;
};


export type ProxmoxSourceState = {
  hypervisorMac: string;
  source: "script-import" | "proxmox-api";
  schemaVersion: number;
  collectorVersion: string;
  collectedAt: string;
  complete: boolean;
  nodeHostname: string;
  nodePveVersion: string;
  nodeClusterName: string;
  nodeStatus: "online" | "offline" | "unknown";
  snapshotDigest: string;
  importedAt: string;
};

export type InfrastructureWorkloadInterface = {
  id: number;
  workloadId: number;
  name: string;
  mac: string;
  bridge: string;
  vlanTag: string;
  configuredAddress: string;
  configuredNetwork: string;
  updatedAt: string;
};

export type InfrastructureWorkloadHostLink = {
  workloadId: number;
  hostId: number;
  hostMac: string;
  linkSource: "manual" | "exact-mac";
  linkedAt: string;
  updatedAt: string;
};

export type InfrastructureWorkloadMatchedHost = {
  hostId: number;
  mac: string;
  name: string;
  ip: string;
  deviceType: string;
};

export type InfrastructureWorkload = {
  id: number;
  hypervisorMac: string;
  nativeId: string;
  workloadType: "vm" | "container";
  nodeName?: string;
  name: string;
  status: "unknown" | "running" | "stopped";
  source: "manual" | "script-import" | "proxmox-api";
  firstSeen: string;
  lastSeen: string;
  retiredAt: string;
  updatedAt: string;
  interfaces: InfrastructureWorkloadInterface[];
  link: InfrastructureWorkloadHostLink | null;
  matchedHost: InfrastructureWorkloadMatchedHost | null;
};

export type WorkloadMatchEvidence = {
  code: string;
  detail: string;
  strength: "exact-mac" | "address" | "name";
  active: boolean;
  matchedValue?: string;
  firstSeen?: string;
  lastSeen?: string;
};

export type WorkloadMatchCandidate = {
  hostId: number;
  mac: string;
  name: string;
  ip: string;
  deviceType: string;
  active: boolean;
  strength: "exact-mac" | "address" | "name";
  assessment: "exact-mac" | "possible-ip-conflict" | "address-only" | "name-only" | "unknown" | string;
  possibleIpConflict: boolean;
  matchedAddresses: string[];
  workloadMacs: string[];
  evidenceFingerprint: string;
  rejected: boolean;
  evidence: WorkloadMatchEvidence[];
};

export type InfrastructureWorkloadMatch = {
  workloadId: number;
  nativeId: string;
  workloadType: "vm" | "container";
  name: string;
  currentLink: InfrastructureWorkloadHostLink | null;
  deterministicExactHostId?: number;
  exactAmbiguous: boolean;
  candidates: WorkloadMatchCandidate[];
};


export type InfrastructureWorkloadMembership = {
  workloadId: number;
  nativeId: string;
  workloadType: "vm" | "container";
  workloadName: string;
  workloadStatus: "unknown" | "running" | "stopped";
  retiredAt: string;
  hostId: number;
  hostMac: string;
  linkSource: "manual" | "exact-mac";
  hypervisorHostId: number;
  hypervisorMac: string;
  hypervisorName: string;
  hypervisorIp: string;
};

export type InfrastructureWorkloadSummary = {
  hypervisorHostId: number;
  hypervisorMac: string;
  hypervisorName: string;
  hypervisorIp: string;
  platform: "proxmox-ve" | "vmware-esxi" | "hyper-v" | "other" | "";
  vmCount: number;
  containerCount: number;
  totalCount: number;
};

export type ProxmoxSnapshotInterface = {
  name: string;
  mac?: string;
  bridge?: string;
  vlanTag?: string;
  configuredAddress?: string;
  configuredNetwork?: string;
};

export type ProxmoxSnapshotWorkload = {
  nativeId: string;
  workloadType: "vm" | "container";
  nodeName?: string;
  name: string;
  status: "unknown" | "running" | "stopped";
  interfaces: ProxmoxSnapshotInterface[];
};

export type ProxmoxSnapshot = {
  schemaVersion: number;
  collectorVersion: string;
  source: "script-import" | "proxmox-api";
  collectedAt: string;
  complete: boolean;
  collectionErrors?: string[];
  node: {
    hostname: string;
    pveVersion: string;
    clusterName?: string;
    status: "online" | "offline" | "unknown";
  };
  workloads: ProxmoxSnapshotWorkload[];
};

export type ProxmoxImportPreviewSummary = {
  added: number;
  updated: number;
  unchanged: number;
  retired: number;
  conflicts: number;
};

export type ProxmoxImportNodeView = {
  hostname: string;
  pveVersion: string;
  clusterName: string;
  status: string;
};

export type ProxmoxImportWorkloadView = {
  id?: number;
  nativeId: string;
  workloadType: string;
  nodeName?: string;
  name: string;
  status: string;
  source: string;
  retiredAt?: string;
  interfaces: Array<{
    name: string;
    mac: string;
    bridge: string;
    vlanTag: string;
    configuredAddress: string;
    configuredNetwork: string;
  }>;
};

export type ProxmoxImportPreview = {
  previewToken: string;
  snapshotDigest: string;
  source: string;
  collectedAt: string;
  complete: boolean;
  applyAllowed: boolean;
  blockedReasons: string[];
  warnings: string[];
  summary: ProxmoxImportPreviewSummary;
  node: {
    action: "add" | "update" | "unchanged";
    before: ProxmoxImportNodeView | null;
    after: ProxmoxImportNodeView;
  };
  managedConflicts: Array<{
    field: string;
    managed: string;
    imported: string;
  }>;
  workloads: Array<{
    action: "add" | "update" | "unchanged" | "retire" | "conflict";
    key: string;
    changes: string[];
    before: ProxmoxImportWorkloadView | null;
    after: ProxmoxImportWorkloadView | null;
  }>;
};

export type ProxmoxImportApplyResponse = {
  applied: boolean;
  importedAt: string;
  summary: ProxmoxImportPreviewSummary;
};


export type ProxmoxAPIConfig = {
  hypervisorMac: string;
  enabled: boolean;
  baseUrl: string;
  tokenId: string;
  tokenSecretConfigured: boolean;
  verifyTls: boolean;
  timeoutSeconds: number;
  lastAttemptAt?: string;
  lastSuccessfulSync?: string;
  lastError?: string;
  status: string;
  updatedAt?: string;
};

export type ProxmoxAPIConfigPatch = {
  enabled?: boolean;
  baseUrl?: string;
  tokenId?: string;
  tokenSecret?: string;
  clearTokenSecret?: boolean;
  verifyTls?: boolean;
  timeoutSeconds?: number;
};

export type ProxmoxAPITestConnectionResponse = {
  status: string;
  result: {
    connectionOk: boolean;
    pveVersion: string;
    nodeAccess: boolean;
    vmInventory: boolean;
    lxcInventory: boolean;
    nodeCount: number;
    vmCount: number;
    lxcCount: number;
  };
};

export type ProxmoxAPISyncPreviewResponse = {
  snapshot: ProxmoxSnapshot;
  preview: ProxmoxImportPreview;
};

const apiFetch = async (url: string, init?: RequestInit): Promise<Response> => {
  const response = await fetch(url, init);
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(detail || response.statusText || "API request failed");
  }

  return response;
};

const apiJSON = async <T>(url: string, init?: RequestInit): Promise<T> => {
  return await (await apiFetch(url, init)).json();
};

const getAttachmentFilename = (response: Response, fallback: string): string => {
  const disposition = response.headers.get("content-disposition") ?? "";
  const match = /filename="([^"]+)"/.exec(disposition) ?? /filename=([^;]+)/.exec(disposition);

  return match?.[1]?.trim() || fallback;
};

const downloadResponse = async (url: string, expectedContentType: string, fallbackFilename: string): Promise<void> => {
  const response = await apiFetch(url);
  const contentType = response.headers.get("content-type") ?? "";

  if (!contentType.toLowerCase().includes(expectedContentType)) {
    const detail = await response.text();
    throw new Error(detail || "Unexpected export response");
  }

  const blob = await response.blob();
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = href;
  anchor.download = getAttachmentFilename(response, fallbackFilename);
  anchor.rel = "noreferrer";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(href);
};

export const apiGetAllHosts = async () => {
  const url = apiPath+'/api/all';
  const hosts = await apiJSON<Host[]>(url);

  return hosts;
};

export const apiGetConfig = async () => {

  const url = apiPath+'/api/config';
  const res = await apiJSON<Conf>(url);

  return res;
};

export const apiGetVersion = async () => {

  const url = apiPath+'/api/version';
  const res = await apiJSON<string>(url);

  return res;
};

export const apiGetActivity = async (limit = 20, query: ActivityQuery = {}): Promise<HostEvent[]> => {
  const params = new URLSearchParams({ limit: String(limit) });
  if (query.category !== undefined) {
    params.set("category", query.category);
  }
  query.eventTypes?.forEach((eventType) => {
    params.append("eventType", eventType);
  });
  if (query.offset !== undefined) {
    params.set("offset", String(query.offset));
  }
  if (query.beforeDate !== undefined && query.beforeId !== undefined) {
    params.set("beforeDate", query.beforeDate);
    params.set("beforeId", String(query.beforeId));
  }
  if (query.mac) {
    params.append("mac", query.mac);
  }
  query.macs?.forEach((mac) => {
    if (mac) {
      params.append("mac", mac);
    }
  });

  const url = apiPath+'/api/activity?'+params.toString();
  const events = await apiJSON<HostEvent[]>(url);

  return events;
};

export const apiGetActivityStats = async (query: Pick<ActivityQuery, "mac" | "macs"> = {}): Promise<ActivityStats> => {
  const params = new URLSearchParams();
  if (query.mac) {
    params.append("mac", query.mac);
  }
  query.macs?.forEach((mac) => {
    if (mac) {
      params.append("mac", mac);
    }
  });

  const suffix = params.toString();
  const url = apiPath+'/api/activity/stats'+(suffix === "" ? "" : "?"+suffix);
  const stats = await apiJSON<ActivityStats>(url);

  return stats;
};

export const apiGetActivityDevices = async (): Promise<ActivityDeviceOption[]> => {
  const url = apiPath+'/api/activity/devices';
  const devices = await apiJSON<ActivityDeviceOption[]>(url);

  return devices;
};

export const apiGetHostActivity = async (id: number | string, limit = 10): Promise<HostEvent[]> => {
  const url = apiPath+'/api/host/'+id+'/activity?limit='+limit;
  const events = await apiJSON<HostEvent[]>(url);

  return events;
};

export const apiSetConfigColor = async (color: "dark" | "light"): Promise<Conf> => {
  const url = apiPath+'/api/config/color';
  return await apiJSON<Conf>(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ color }),
  });
};

export const apiSetRetention = async (presenceRetention: number, connectivityRetention: number): Promise<Conf> => {
  const url = apiPath+'/api/config/retention';
  return await apiJSON<Conf>(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ presenceRetention, connectivityRetention }),
  });
};

export const apiDownloadBackup = async (): Promise<void> => {
  await downloadResponse(apiPath+'/api/export/backup', "application/json", "lannventory-backup.json");
};

export const apiDownloadInventoryCSV = async (): Promise<void> => {
  await downloadResponse(apiPath+'/api/export/inventory.csv', "text/csv", "lannventory-inventory.csv");
};

export const apiTestNotify = async () => {

  const url = apiPath+'/api/notify_test';
  await apiFetch(url);
};

export const apiEditHost = async (id:number, name:string, known:string) => {

  const url = apiPath+'/api/edit/'+encodeURIComponent(String(id))+'/'+encodeURIComponent(name)+'/'+encodeURIComponent(known);
  const res = await apiJSON<string>(url);

  return res;
};

export const apiSetDeviceType = async (id: number, deviceType: DeviceTypeValue): Promise<Host> => {
  if (!isDeviceTypeValue(deviceType)) {
    throw new Error("invalid device type");
  }

  const url = apiPath+'/api/host/'+id+'/type';
  return await apiJSON<Host>(url, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ deviceType }),
  });
};

export const apiSetHostMetadata = async (id: number, metadata: HostMetadataPayload): Promise<Host> => {
  const url = apiPath+'/api/host/'+id+'/metadata';
  return await apiJSON<Host>(url, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(metadata),
  });
};

export const apiPatchHost = async (id: number, payload: HostInventoryPayload): Promise<Host> => {
  if (payload.deviceType !== undefined && !isDeviceTypeValue(payload.deviceType)) {
    throw new Error("invalid device type");
  }

  const url = apiPath+'/api/host/'+id;
  return await apiJSON<Host>(url, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  });
};

export const apiGetInventoryOptions = async (): Promise<InventoryOptions> => {
  const url = apiPath+'/api/inventory/options';
  return await apiJSON<InventoryOptions>(url);
};

export const apiGetHost = async (id:string) => {

  const url = apiPath+'/api/host/'+id;
  const res = await apiJSON<Host>(url);

  return res;
};

export const apiGetHostDeviceProfile = async (id: number | string): Promise<DeviceProfileResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/profile';
  return await apiJSON<DeviceProfileResponse>(url);
};

export const apiPatchHostDeviceProfile = async (
  id: number | string,
  payload: DeviceProfilePayload,
): Promise<DeviceProfileResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/profile';
  return await apiJSON<DeviceProfileResponse>(url, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  });
};

const patchHostProfileLayer = async <T>(
  id: number | string,
  layer: string,
  payload: T,
): Promise<DeviceProfileResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/profile/'+layer;
  return await apiJSON<DeviceProfileResponse>(url, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  });
};

export const apiPatchHostNetworkDeviceProfile = async (
  id: number | string,
  payload: NetworkDeviceProfilePayload,
): Promise<DeviceProfileResponse> => patchHostProfileLayer(id, "network", payload);

export const apiPatchHostSystemDeviceProfile = async (
  id: number | string,
  payload: SystemDeviceProfilePayload,
): Promise<DeviceProfileResponse> => patchHostProfileLayer(id, "system", payload);

export const apiPatchHostHypervisorProfile = async (
  id: number | string,
  payload: HypervisorProfilePayload,
): Promise<DeviceProfileResponse> => patchHostProfileLayer(id, "hypervisor", payload);

export const apiDeleteHostHypervisorProfile = async (
  id: number | string,
): Promise<DeviceProfileResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/profile/hypervisor';
  return await apiJSON<DeviceProfileResponse>(url, { method: 'DELETE' });
};


export const apiGetProxmoxSourceState = async (
  id: number | string,
  source: "script-import" | "proxmox-api" = "script-import",
): Promise<ProxmoxSourceState | null> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/source-state?source='+encodeURIComponent(source);
  return await apiJSON<ProxmoxSourceState | null>(url);
};

export const apiGetProxmoxAPIConfig = async (
  id: number | string,
): Promise<ProxmoxAPIConfig> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/api-config';
  return await apiJSON<ProxmoxAPIConfig>(url);
};

export const apiPatchProxmoxAPIConfig = async (
  id: number | string,
  patch: ProxmoxAPIConfigPatch,
): Promise<ProxmoxAPIConfig> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/api-config';
  return await apiJSON<ProxmoxAPIConfig>(url, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(patch),
  });
};

export const apiTestProxmoxAPIConnection = async (
  id: number | string,
): Promise<ProxmoxAPITestConnectionResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/api/test';
  return await apiJSON<ProxmoxAPITestConnectionResponse>(url, { method: 'POST' });
};

export const apiPreviewProxmoxAPISync = async (
  id: number | string,
): Promise<ProxmoxAPISyncPreviewResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/api/sync-preview';
  return await apiJSON<ProxmoxAPISyncPreviewResponse>(url, { method: 'POST' });
};

export const apiApplyProxmoxAPISync = async (
  id: number | string,
  previewToken: string,
  snapshot: ProxmoxSnapshot,
): Promise<ProxmoxImportApplyResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/api/sync-apply';
  return await apiJSON<ProxmoxImportApplyResponse>(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ previewToken, confirmed: true, snapshot }),
  });
};

export const apiGetHostWorkloads = async (
  id: number | string,
): Promise<InfrastructureWorkload[]> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/workloads';
  return await apiJSON<InfrastructureWorkload[]>(url);
};

export const apiGetHostWorkloadMatches = async (
  id: number | string,
): Promise<InfrastructureWorkloadMatch[]> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/workload-matches';
  return await apiJSON<InfrastructureWorkloadMatch[]>(url);
};

export const apiRejectInfrastructureWorkloadCandidate = async (
  id: number | string,
  workloadId: number,
  candidateHostId: number,
  evidenceFingerprint: string,
): Promise<WorkloadMatchCandidate> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+
    '/workloads/'+encodeURIComponent(String(workloadId))+
    '/match-rejections/'+encodeURIComponent(String(candidateHostId));
  return await apiJSON<WorkloadMatchCandidate>(url, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ evidenceFingerprint }),
  });
};

export const apiClearInfrastructureWorkloadCandidateRejection = async (
  id: number | string,
  workloadId: number,
  candidateHostId: number,
): Promise<void> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+
    '/workloads/'+encodeURIComponent(String(workloadId))+
    '/match-rejections/'+encodeURIComponent(String(candidateHostId));
  await apiFetch(url, { method: 'DELETE' });
};

export const apiGetInfrastructureWorkloadMemberships = async (
  hostId?: number | string,
): Promise<InfrastructureWorkloadMembership[]> => {
  const params = new URLSearchParams();
  if (hostId !== undefined && String(hostId).trim() !== "") {
    params.set("hostId", String(hostId));
  }
  const query = params.toString();
  const url = apiPath+'/api/infrastructure/workload-memberships'+(query ? '?'+query : '');
  return await apiJSON<InfrastructureWorkloadMembership[]>(url);
};

export const apiGetInfrastructureWorkloadSummaries = async (): Promise<InfrastructureWorkloadSummary[]> => {
  const url = apiPath+'/api/infrastructure/workload-summaries';
  return await apiJSON<InfrastructureWorkloadSummary[]>(url);
};

export const apiPreviewProxmoxImport = async (
  id: number | string,
  snapshot: ProxmoxSnapshot,
): Promise<ProxmoxImportPreview> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/import/preview';
  return await apiJSON<ProxmoxImportPreview>(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(snapshot),
  });
};

export const apiApplyProxmoxImport = async (
  id: number | string,
  previewToken: string,
  snapshot: ProxmoxSnapshot,
): Promise<ProxmoxImportApplyResponse> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/proxmox/import/apply';
  return await apiJSON<ProxmoxImportApplyResponse>(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ previewToken, confirmed: true, snapshot }),
  });
};

export const apiSetInfrastructureWorkloadLink = async (
  id: number | string,
  workloadId: number,
  hostId: number,
): Promise<InfrastructureWorkload> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/workloads/'+encodeURIComponent(String(workloadId))+'/link';
  return await apiJSON<InfrastructureWorkload>(url, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ hostId }),
  });
};

export const apiDeleteInfrastructureWorkloadLink = async (
  id: number | string,
  workloadId: number,
): Promise<InfrastructureWorkload> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/workloads/'+encodeURIComponent(String(workloadId))+'/link';
  return await apiJSON<InfrastructureWorkload>(url, { method: 'DELETE' });
};

export const apiGetHostServices = async (id: number | string): Promise<Service[]> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/services';
  return await apiJSON<Service[]>(url);
};

export const apiGetServiceScanSettings = async (id: number | string): Promise<ServiceScanSettings> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/service-scan-settings';
  return await apiJSON<ServiceScanSettings>(url);
};

export const apiSetServiceScanSettings = async (
  id: number | string,
  payload: ServiceScanSettingsPayload,
): Promise<ServiceScanSettings> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/service-scan-settings';
  return await apiJSON<ServiceScanSettings>(url, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  });
};

export const apiGetHostIdentity = async (id: number | string): Promise<HostIdentity> => {
  const url = apiPath+'/api/host/'+encodeURIComponent(String(id))+'/identity';
  return await apiJSON<HostIdentity>(url);
};

export const apiGetAddressIdentity = async (address: string): Promise<AddressIdentity> => {
  const params = new URLSearchParams({ address });
  const url = apiPath+'/api/identity/address?'+params.toString();
  return await apiJSON<AddressIdentity>(url);
};

export const apiDelHost = async (id:number) => {

  const url = apiPath+'/api/host/del/'+id;
  const res = await apiJSON<string>(url);

  return res;
};

export const apiPortScan = async (ip:string, port:number) => {
  const url = apiPath+'/api/port/'+ip+'/'+port;
  const res = await apiJSON<boolean>(url);

  return res;
};

export type HostPortScanResult = {
  port: number;
  open: boolean;
  state: "open" | "closed" | "indeterminate";
};

export type ServiceScanSettings = {
  enabled: boolean;
  intervalMinutes: number;
  ports: number[];
  nextScanAt: string;
  lastAttemptAt: string;
  lastSuccessfulAt: string;
  lastError: string;
};

export type ServiceScanSettingsPayload = Pick<ServiceScanSettings, "enabled" | "intervalMinutes" | "ports">;

export const apiScanHostPort = async (id:number, port:number): Promise<HostPortScanResult> => {
  const url = apiPath+'/api/host/'+id+'/port/'+port+'/scan';
  return await apiJSON<HostPortScanResult>(url, {
    method: 'POST',
  });
};

export const apiGetHistory = async (mac:string) => {
  const url = apiPath+'/api/history/'+mac+'/?num=210';
  const hosts = await apiJSON<Host[]>(url);

  return hosts;
};

export const apiGetHistoryByDate = async (mac:string, date: string) => {
  const url = apiPath+'/api/history/'+mac+'/'+date;
  const hosts = await apiJSON<Host[]>(url);

  return hosts;
};

export const apiWOL = async (mac:string) => {

  const url = apiPath+'/api/wol/'+mac;
  const res = await apiJSON<boolean>(url);

  return res;
};