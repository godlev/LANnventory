import { createReadStream, existsSync } from 'node:fs';
import { createServer } from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const host = '127.0.0.1';
const port = 8840;
const now = '2026-08-23 10:15:00';
const mockUpdateAvailable = process.env.LANNVENTORY_MOCK_UPDATE_AVAILABLE === '1';
let nextActivityId = 1;
const deviceTypes = new Set([
  '',
  'router',
  'switch',
  'access-point',
  'firewall',
  'server',
  'nas',
  'desktop',
  'laptop',
  'phone',
  'tablet',
  'tv',
  'printer',
  'camera',
  'iot',
  'virtual-machine',
  'container',
  'game-console',
  'other',
]);
const connectivityEvents = new Set(['online', 'offline']);
const metadataEvents = new Set(['owner-changed', 'location-changed', 'notes-changed', 'tags-changed', 'pinned-changed']);
const changeEvents = new Set(['discovered', 'known', 'unknown', 'device-type-changed', ...metadataEvents]);
const validActivityEvents = new Set([...connectivityEvents, ...changeEvents]);

const fakeHosts = [
  {
    ID: 1,
    Name: 'router',
    DNS: 'router.local',
    Iface: 'eth0',
    IP: '192.168.1.1',
    Mac: 'AA:BB:CC:00:00:01',
    Hw: 'LG Electronics',
    Date: now,
    Known: 1,
    Now: 1,
    DeviceType: 'router',
    FirstSeen: '2026-07-18 08:10:00',
    LastSeen: now,
    FirstSeenEstimated: false,
  },
  {
    ID: 2,
    Name: 'NAS',
    DNS: 'nas.local',
    Iface: 'eth0',
    IP: '192.168.1.20',
    Mac: 'AA:BB:CC:00:00:20',
    Hw: 'Unknown',
    Date: now,
    Known: 1,
    Now: 1,
    DeviceType: 'nas',
    FirstSeen: '2026-08-01 12:00:00',
    LastSeen: now,
    FirstSeenEstimated: true,
  },
  {
    ID: 3,
    Name: 'desktop',
    DNS: 'desktop.local',
    Iface: 'eth0',
    IP: '192.168.1.42',
    Mac: 'AA:BB:CC:00:00:42',
    Hw: '(Unknown)',
    Date: now,
    Known: 1,
    Now: 1,
    DeviceType: 'desktop',
    FirstSeen: '2026-08-10 09:30:00',
    LastSeen: now,
    FirstSeenEstimated: false,
  },
  {
    ID: 4,
    Name: 'phone',
    DNS: 'phone.local',
    Iface: 'wifi0',
    IP: '192.168.1.83',
    Mac: 'AA:BB:CC:00:00:83',
    Hw: 'Unknown: locally administered',
    Date: now,
    Known: 0,
    Now: 1,
    DeviceType: 'phone',
    FirstSeen: '2026-08-20 18:12:00',
    LastSeen: now,
    FirstSeenEstimated: false,
  },
  {
    ID: 5,
    Name: 'offline device',
    DNS: '',
    Iface: 'eth0',
    IP: '192.168.1.120',
    Mac: 'AA:BB:CC:00:01:20',
    Hw: '(Unknown: locally administered)',
    Date: '2025-12-30 18:42:09',
    Known: 1,
    Now: 0,
    DeviceType: '',
    FirstSeen: '2025-12-30 18:42:09',
    LastSeen: '2025-12-30 18:42:09',
    FirstSeenEstimated: true,
  },
  {
    ID: 6,
    Name: 'proxmox',
    DNS: 'proxmox.local',
    Iface: 'eth0',
    IP: '192.168.1.6',
    Mac: 'AA:BB:CC:00:00:60',
    Hw: 'Fujitsu',
    Date: now,
    Known: 1,
    Now: 1,
    DeviceType: 'server',
    FirstSeen: '2026-07-01 07:30:00',
    LastSeen: now,
    FirstSeenEstimated: false,
  },
  {
    ID: 7,
    Name: 'media-vm',
    DNS: 'media-vm.local',
    Iface: 'eth0',
    IP: '192.168.1.27',
    Mac: 'AA:BB:CC:00:00:70',
    Hw: 'QEMU virtual NIC',
    Date: now,
    Known: 1,
    Now: 1,
    DeviceType: 'virtual-machine',
    FirstSeen: '2026-08-05 11:20:00',
    LastSeen: now,
    FirstSeenEstimated: false,
  },
];

const hostMetadata = new Map([
  ['AA:BB:CC:00:00:01', {
    Owner: 'Network Team',
    Location: 'Utility closet',
    Notes: 'Default gateway and DHCP edge.',
    Tags: ['gateway', 'critical'],
    Pinned: true,
  }],
  ['AA:BB:CC:00:00:20', {
    Owner: 'Storage Team',
    Location: 'Rack 1',
    Notes: 'Primary media and backup NAS.',
    Tags: ['storage', 'backup'],
    Pinned: true,
  }],
  ['AA:BB:CC:00:00:42', {
    Owner: 'John Smith',
    Location: 'Office',
    Notes: '',
    Tags: ['workstation'],
    Pinned: false,
  }],
  ['AA:BB:CC:00:00:60', {
    Owner: 'Lab',
    Location: 'Rack 1',
    Notes: 'Mock Proxmox node for Phase 35 UI.',
    Tags: ['server', 'proxmox'],
    Pinned: true,
  }],
  ['AA:BB:CC:00:00:70', {
    Owner: 'Lab',
    Location: 'Virtual',
    Notes: 'Mock VM with deterministic exact-MAC match.',
    Tags: ['vm'],
    Pinned: false,
  }],
]);

const deviceProfiles = new Map([
  ['AA:BB:CC:00:00:01', {
    managed: {
      mac: 'AA:BB:CC:00:00:01',
      manufacturer: 'GL.iNet',
      model: 'Example Router',
      managementAddress: 'router.local',
      updatedAt: new Date().toISOString(),
    },
    network: {
      mac: 'AA:BB:CC:00:00:01',
      managementMode: 'managed',
      physicalPortCount: 5,
      portCapabilityNotes: 'Example managed router profile.',
      updatedAt: new Date().toISOString(),
    },
    system: null,
    hypervisor: null,
  }],
  ['AA:BB:CC:00:00:20', {
    managed: {
      mac: 'AA:BB:CC:00:00:20',
      manufacturer: 'Example',
      model: 'NAS',
      managementAddress: 'nas.local',
      updatedAt: new Date().toISOString(),
    },
    network: null,
    system: {
      mac: 'AA:BB:CC:00:00:20',
      role: 'Storage',
      operatingSystem: 'TrueNAS SCALE',
      version: '25.04',
      updatedAt: new Date().toISOString(),
    },
    hypervisor: null,
  }],
  ['AA:BB:CC:00:00:60', {
    managed: {
      mac: 'AA:BB:CC:00:00:60',
      manufacturer: 'Fujitsu',
      model: 'CELSIUS W550P',
      managementAddress: 'proxmox.local',
      updatedAt: new Date().toISOString(),
    },
    network: null,
    system: {
      mac: 'AA:BB:CC:00:00:60',
      role: 'Virtualization',
      operatingSystem: 'Proxmox VE',
      version: '9.2.10',
      updatedAt: new Date().toISOString(),
    },
    hypervisor: {
      mac: 'AA:BB:CC:00:00:60',
      platform: 'proxmox-ve',
      version: '9.2.10',
      nodeName: 'pve-managed',
      clusterName: 'home-lab',
      updatedAt: new Date().toISOString(),
    },
  }],
]);

const proxmoxSourceStates = new Map([
  [6, {
    hypervisorMac: 'AA:BB:CC:00:00:60',
    source: 'script-import',
    schemaVersion: 1,
    collectorVersion: '1.0.0',
    collectedAt: '2026-09-19T15:20:00Z',
    complete: true,
    nodeHostname: 'pve-mock',
    nodePveVersion: 'pve-manager/9.2.10',
    nodeClusterName: 'home-lab',
    nodeStatus: 'online',
    snapshotDigest: 'mock-snapshot-digest',
    importedAt: '2026-09-19T15:21:00Z',
  }],
]);

let nextMockWorkloadId = 3100;
const proxmoxWorkloads = new Map([
  [6, [
    {
      id: 3001,
      hypervisorMac: 'AA:BB:CC:00:00:60',
      nativeId: '119',
      workloadType: 'vm',
      name: 'media-vm',
      status: 'running',
      source: 'script-import',
      firstSeen: '2026-09-15T09:00:00Z',
      lastSeen: '2026-09-19T15:20:00Z',
      retiredAt: '',
      updatedAt: '2026-09-19T15:21:00Z',
      interfaces: [{
        id: 1,
        workloadId: 3001,
        name: 'net0',
        mac: 'AA:BB:CC:00:00:70',
        bridge: 'vmbr0',
        vlanTag: '',
        configuredAddress: '192.168.1.27/24',
        configuredNetwork: '192.168.1.0/24',
        updatedAt: '2026-09-19T15:21:00Z',
      }],
      link: {
        workloadId: 3001,
        hostId: 7,
        hostMac: 'AA:BB:CC:00:00:70',
        linkSource: 'exact-mac',
        linkedAt: '2026-09-19T15:21:00Z',
        updatedAt: '2026-09-19T15:21:00Z',
      },
      matchedHost: null,
    },
    {
      id: 3002,
      hypervisorMac: 'AA:BB:CC:00:00:60',
      nativeId: '127',
      workloadType: 'container',
      name: 'desktop-helper',
      status: 'stopped',
      source: 'script-import',
      firstSeen: '2026-09-15T09:00:00Z',
      lastSeen: '2026-09-19T15:20:00Z',
      retiredAt: '',
      updatedAt: '2026-09-19T15:21:00Z',
      interfaces: [{
        id: 2,
        workloadId: 3002,
        name: 'eth0',
        mac: 'AA:BB:CC:00:00:7F',
        bridge: 'vmbr0',
        vlanTag: '',
        configuredAddress: '192.168.1.42/24',
        configuredNetwork: '192.168.1.0/24',
        updatedAt: '2026-09-19T15:21:00Z',
      }],
      link: null,
      matchedHost: null,
    },
    {
      id: 3003,
      hypervisorMac: 'AA:BB:CC:00:00:60',
      nativeId: '108',
      workloadType: 'container',
      name: 'old-service',
      status: 'stopped',
      source: 'script-import',
      firstSeen: '2026-09-10T09:00:00Z',
      lastSeen: '2026-09-17T15:20:00Z',
      retiredAt: '2026-09-18T15:20:00Z',
      updatedAt: '2026-09-18T15:21:00Z',
      interfaces: [],
      link: null,
      matchedHost: null,
    },
  ]],
]);

const mockProxmoxPreviews = new Map();
const workloadCandidateRejections = new Map();


const config = {
  Host: host,
  Port: String(port),
  Theme: 'sand',
  Color: 'dark',
  DirPath: './dev/.mock-data',
  ConfPath: './dev/.mock-data/config_v2.yaml',
  DBPath: './dev/.mock-data/scan.db',
  NodePath: '',
  LogLevel: 'info',
  Ifaces: 'eth0 wifi0',
  ArpArgs: '',
  ArpStrs: [],
  Timeout: 600,
  TrimHist: 48,
  ConnectivityRetention: 168,
  ShoutURL: '',
  ShoutURLConfigured: false,
  Version: '0.1.0-beta.3-dev-mock',
  UseDB: 'sqlite',
  PGConnect: '',
  PGConnectConfigured: false,
  InfluxEnable: false,
  InfluxAddr: '',
  InfluxToken: '',
  InfluxTokenConfigured: false,
  InfluxOrg: '',
  InfluxBucket: '',
  InfluxSkipTLS: false,
  PrometheusEnable: false,
  UpdateChannel: 'beta',
  UpdateCheckAuto: false,
  UpdateAuto: false,
  UpdateCheckIntervalHours: 24,
};
let mockUpdateLastChecked = new Date().toISOString();

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendPublicPath = path.resolve(__dirname, '../public/fs/public');
const backendPublicPath = path.resolve(__dirname, '../../backend/internal/web/public');
const localPublicAssets = new Set([
  'favicon.png',
  'lanventory.ico',
  'lanventory-16x16.png',
  'lanventory-32x32.png',
  'lanventory-48x48.png',
  'lanventory-64x64.png',
  'lanventory-128x128.png',
  'lanventory-180x180.png',
  'lanventory-192x192.png',
  'lanventory-256x256.png',
  'lanventory-512x512.png',
  'lanventory-navbar.png',
]);
const activityEvents = [];

function profileForHost(hostEntry) {
  const existing = deviceProfiles.get(hostEntry.Mac);
  if (existing) {
    return structuredClone(existing);
  }
  return { managed: null, network: null, system: null, hypervisor: null };
}


function hostSummary(hostEntry) {
  if (!hostEntry) return null;
  return {
    hostId: hostEntry.ID,
    mac: hostEntry.Mac,
    name: hostEntry.Name,
    ip: hostEntry.IP,
    deviceType: hostEntry.DeviceType,
  };
}

function workloadMatchesForHost(hostId) {
  const workloads = proxmoxWorkloads.get(hostId) ?? [];
  return workloads.map((workload) => {
    const candidates = new Map();
    const addCandidate = (hostEntry, strength, code, detail, matchedValue = '') => {
      if (!hostEntry || hostEntry.ID === hostId) return;
      const existing = candidates.get(hostEntry.ID) ?? {
        hostId: hostEntry.ID,
        mac: hostEntry.Mac,
        name: hostEntry.Name,
        ip: hostEntry.IP,
        deviceType: hostEntry.DeviceType,
        active: hostEntry.Now === 1,
        strength,
        assessment: 'unknown',
        possibleIpConflict: false,
        matchedAddresses: [],
        workloadMacs: [],
        evidenceFingerprint: '',
        rejected: false,
        evidence: [],
      };
      const rank = { 'exact-mac': 0, address: 1, name: 2 };
      if (rank[strength] < rank[existing.strength]) existing.strength = strength;
      existing.evidence.push({ code, detail, strength, active: hostEntry.Now === 1, matchedValue });
      candidates.set(hostEntry.ID, existing);
    };

    const workloadMacs = [...new Set((workload.interfaces ?? [])
      .map((iface) => String(iface.mac ?? '').trim().toUpperCase())
      .filter(Boolean))].sort();

    for (const iface of workload.interfaces ?? []) {
      const ifaceMac = String(iface.mac ?? '').toUpperCase();
      if (ifaceMac) {
        for (const hostEntry of fakeHosts.filter((item) => item.Mac.toUpperCase() === ifaceMac)) {
          addCandidate(hostEntry, 'exact-mac', 'exact-interface-mac', (iface.name || 'interface')+' MAC '+ifaceMac+' exactly matches the Host MAC', ifaceMac);
        }
      }
      const address = String(iface.configuredAddress ?? '').split('/')[0];
      if (address) {
        for (const hostEntry of fakeHosts.filter((item) => item.IP === address)) {
          addCandidate(hostEntry, 'address', 'current-address', (iface.name || 'interface')+' address '+address+' matches the Host current address', address);
        }
      }
    }

    const normalizedName = String(workload.name ?? '').trim().toLowerCase();
    if (normalizedName) {
      for (const hostEntry of fakeHosts) {
        if (hostEntry.ID === hostId) continue;
        if (String(hostEntry.Name ?? '').trim().toLowerCase() === normalizedName) {
          addCandidate(hostEntry, 'name', 'host-name', 'Workload name matches Host name', normalizedName);
        }
        if (String(hostEntry.DNS ?? '').trim().toLowerCase().replace(/\.$/, '') === normalizedName.replace(/\.$/, '')) {
          addCandidate(hostEntry, 'name', 'host-dns', 'Workload name matches Host DNS name', normalizedName);
        }
      }
    }

    const ordered = [...candidates.values()].map((candidate) => {
      candidate.workloadMacs = workloadMacs;
      candidate.matchedAddresses = [...new Set(candidate.evidence
        .filter((item) => item.strength === 'address' && item.matchedValue)
        .map((item) => item.matchedValue))].sort();
      const hasExact = candidate.evidence.some((item) => item.strength === 'exact-mac');
      const hasAddress = candidate.evidence.some((item) => item.strength === 'address');
      const hasName = candidate.evidence.some((item) => item.strength === 'name');
      candidate.possibleIpConflict = hasAddress && !hasExact && Boolean(candidate.mac) &&
        workloadMacs.length > 0 && !workloadMacs.includes(String(candidate.mac).toUpperCase());
      candidate.assessment = hasExact
        ? 'exact-mac'
        : candidate.possibleIpConflict
          ? 'possible-ip-conflict'
          : hasAddress
            ? 'address-only'
            : hasName
              ? 'name-only'
              : 'unknown';
      candidate.evidenceFingerprint = [
        candidate.mac,
        candidate.strength,
        candidate.assessment,
        workloadMacs.join(','),
        ...candidate.evidence.map((item) => item.code+'|'+(item.matchedValue ?? '')+'|'+String(item.active)),
      ].join('\\n');
      candidate.rejected = candidate.strength !== 'exact-mac' &&
        workloadCandidateRejections.get(workload.id+':'+candidate.hostId) === candidate.evidenceFingerprint;
      return candidate;
    }).sort((a, b) => {
      const rank = { 'exact-mac': 0, address: 1, name: 2 };
      return rank[a.strength]-rank[b.strength] || Number(b.active)-Number(a.active) || a.hostId-b.hostId;
    });
    const exact = ordered.filter((item) => item.strength === 'exact-mac');
    return {
      workloadId: workload.id,
      nativeId: workload.nativeId,
      workloadType: workload.workloadType,
      name: workload.name,
      currentLink: workload.link,
      deterministicExactHostId: exact.length === 1 ? exact[0].hostId : undefined,
      exactAmbiguous: exact.length > 1,
      candidates: ordered,
    };
  });
}

function mockWorkloadMemberships(hostId = 0) {
  const rows = [];
  for (const [hypervisorHostId, workloads] of proxmoxWorkloads.entries()) {
    const hypervisor = findHostByID(hypervisorHostId);
    for (const workload of workloads) {
      if (!workload.link) continue;
      if (hostId > 0 && workload.link.hostId !== hostId) continue;
      const target = findHostByID(workload.link.hostId);
      if (!target || target.Mac !== workload.link.hostMac) continue;
      rows.push({
        workloadId: workload.id,
        nativeId: workload.nativeId,
        workloadType: workload.workloadType,
        workloadName: workload.name,
        workloadStatus: workload.status,
        retiredAt: workload.retiredAt ?? '',
        hostId: target.ID,
        hostMac: target.Mac,
        linkSource: workload.link.linkSource,
        hypervisorHostId: hypervisor?.ID ?? 0,
        hypervisorMac: workload.hypervisorMac,
        hypervisorName: hypervisor?.Name ?? '',
        hypervisorIp: hypervisor?.IP ?? '',
      });
    }
  }
  return rows;
}

function hydrateMockWorkloads(hostId) {
  const workloads = structuredClone(proxmoxWorkloads.get(hostId) ?? []);
  for (const workload of workloads) {
    workload.matchedHost = workload.link ? hostSummary(findHostByID(workload.link.hostId)) : null;
  }
  return workloads;
}

function mockImportPreview(hostId, snapshot) {
  const current = proxmoxWorkloads.get(hostId) ?? [];
  const currentByKey = new Map(current.map((item) => [item.workloadType+':'+item.nativeId, item]));
  const seen = new Set();
  const diffs = [];
  const summary = { added: 0, updated: 0, unchanged: 0, retired: 0, conflicts: 0 };
  for (const incoming of snapshot.workloads ?? []) {
    const key = incoming.workloadType+':'+incoming.nativeId;
    seen.add(key);
    const before = currentByKey.get(key);
    const after = {
      nativeId: incoming.nativeId,
      workloadType: incoming.workloadType,
      name: incoming.name ?? '',
      status: incoming.status ?? 'unknown',
      source: 'script-import',
      interfaces: incoming.interfaces ?? [],
    };
    if (!before) {
      summary.added++;
      diffs.push({ action: 'add', key, changes: ['workload'], before: null, after });
      continue;
    }
    const changes = [];
    if (before.name !== after.name) changes.push('name');
    if (before.status !== after.status) changes.push('status');
    const beforeInterfaces = (before.interfaces ?? []).map(({ name, mac, bridge, vlanTag, configuredAddress, configuredNetwork }) => ({ name, mac, bridge, vlanTag, configuredAddress, configuredNetwork }));
    if (JSON.stringify(beforeInterfaces) !== JSON.stringify(after.interfaces ?? [])) changes.push('interfaces');
    if (before.retiredAt) changes.push('retired');
    if (changes.length) {
      summary.updated++;
      diffs.push({ action: 'update', key, changes, before, after });
    } else {
      summary.unchanged++;
      diffs.push({ action: 'unchanged', key, changes: [], before, after });
    }
  }
  if (snapshot.complete) {
    for (const before of current) {
      const key = before.workloadType+':'+before.nativeId;
      if (before.source === 'script-import' && !before.retiredAt && !seen.has(key)) {
        summary.retired++;
        diffs.push({ action: 'retire', key, changes: ['retired'], before, after: null });
      }
    }
  }

  const previous = proxmoxSourceStates.get(hostId);
  const beforeNode = previous ? {
    hostname: previous.nodeHostname,
    pveVersion: previous.nodePveVersion,
    clusterName: previous.nodeClusterName,
    status: previous.nodeStatus,
  } : null;
  const afterNode = {
    hostname: snapshot.node?.hostname ?? '',
    pveVersion: snapshot.node?.pveVersion ?? '',
    clusterName: snapshot.node?.clusterName ?? '',
    status: snapshot.node?.status ?? 'unknown',
  };
  const nodeAction = !beforeNode ? 'add' : JSON.stringify(beforeNode) === JSON.stringify(afterNode) ? 'unchanged' : 'update';
  const token = 'mock-preview-'+hostId+'-'+JSON.stringify(snapshot).length+'-'+String(snapshot.collectedAt ?? '');
  mockProxmoxPreviews.set(hostId, { token, snapshotJSON: JSON.stringify(snapshot) });

  return {
    previewToken: token,
    snapshotDigest: 'mock-'+JSON.stringify(snapshot).length,
    source: snapshot.source ?? '',
    collectedAt: snapshot.collectedAt ?? '',
    complete: Boolean(snapshot.complete),
    applyAllowed: Boolean(snapshot.complete),
    blockedReasons: snapshot.complete ? [] : ['snapshot is incomplete; collect a complete snapshot before applying'],
    warnings: snapshot.collectionErrors ?? [],
    summary,
    node: { action: nodeAction, before: beforeNode, after: afterNode },
    managedConflicts: [],
    workloads: diffs,
  };
}

function applyMockProxmoxSnapshot(hostId, snapshot, previewToken) {
  const previewState = mockProxmoxPreviews.get(hostId);
  if (!previewState || previewState.token !== previewToken || previewState.snapshotJSON !== JSON.stringify(snapshot)) {
    return { error: 'preview is stale; generate a new preview before applying', status: 409 };
  }
  if (!snapshot.complete) {
    return { error: 'preview cannot be applied until blocking issues are resolved', status: 400 };
  }

  const hypervisor = findHostByID(hostId);
  const importedAt = new Date().toISOString();
  const current = proxmoxWorkloads.get(hostId) ?? [];
  const currentByKey = new Map(current.map((item) => [item.workloadType+':'+item.nativeId, item]));
  const seen = new Set();
  const next = [];

  for (const incoming of snapshot.workloads ?? []) {
    const key = incoming.workloadType+':'+incoming.nativeId;
    seen.add(key);
    const previous = currentByKey.get(key);
    const id = previous?.id ?? nextMockWorkloadId++;
    const interfaces = (incoming.interfaces ?? []).map((iface, index) => ({
      id: previous?.interfaces?.[index]?.id ?? id*100+index,
      workloadId: id,
      name: iface.name ?? '',
      mac: String(iface.mac ?? '').toUpperCase(),
      bridge: iface.bridge ?? '',
      vlanTag: iface.vlanTag ?? '',
      configuredAddress: iface.configuredAddress ?? '',
      configuredNetwork: iface.configuredNetwork ?? '',
      updatedAt: importedAt,
    }));
    let link = previous?.link?.linkSource === 'manual' ? previous.link : null;
    if (!link) {
      const exactHosts = new Set();
      for (const iface of interfaces) {
        if (!iface.mac) continue;
        for (const hostEntry of fakeHosts.filter((item) => item.ID !== hostId && item.Mac.toUpperCase() === iface.mac)) {
          exactHosts.add(hostEntry.ID);
        }
      }
      if (exactHosts.size === 1) {
        const target = findHostByID([...exactHosts][0]);
        link = {
          workloadId: id,
          hostId: target.ID,
          hostMac: target.Mac,
          linkSource: 'exact-mac',
          linkedAt: importedAt,
          updatedAt: importedAt,
        };
      }
    }
    next.push({
      id,
      hypervisorMac: hypervisor?.Mac ?? '',
      nativeId: String(incoming.nativeId ?? ''),
      workloadType: incoming.workloadType ?? 'vm',
      name: incoming.name ?? '',
      status: incoming.status ?? 'unknown',
      source: 'script-import',
      firstSeen: previous?.firstSeen ?? snapshot.collectedAt,
      lastSeen: snapshot.collectedAt,
      retiredAt: '',
      updatedAt: importedAt,
      interfaces,
      link,
      matchedHost: null,
    });
  }
  for (const previous of current) {
    const key = previous.workloadType+':'+previous.nativeId;
    if (seen.has(key) || previous.source !== 'script-import' || previous.retiredAt) continue;
    next.push({ ...previous, retiredAt: snapshot.collectedAt, updatedAt: importedAt });
  }
  proxmoxWorkloads.set(hostId, next);
  proxmoxSourceStates.set(hostId, {
    hypervisorMac: hypervisor?.Mac ?? '',
    source: 'script-import',
    schemaVersion: snapshot.schemaVersion ?? 1,
    collectorVersion: snapshot.collectorVersion ?? '',
    collectedAt: snapshot.collectedAt ?? '',
    complete: true,
    nodeHostname: snapshot.node?.hostname ?? '',
    nodePveVersion: snapshot.node?.pveVersion ?? '',
    nodeClusterName: snapshot.node?.clusterName ?? '',
    nodeStatus: snapshot.node?.status ?? 'unknown',
    snapshotDigest: 'mock-'+JSON.stringify(snapshot).length,
    importedAt,
  });
  mockProxmoxPreviews.delete(hostId);
  return { applied: true, importedAt, summary: mockImportPreview(hostId, snapshot).summary };
}

function sendJSON(res, value, statusCode = 200) {
  res.writeHead(statusCode, {
    'content-type': 'application/json; charset=utf-8',
    'cache-control': 'no-store',
  });
  res.end(JSON.stringify(value));
}

function sendText(res, value, statusCode = 200) {
  res.writeHead(statusCode, {
    'content-type': 'text/plain; charset=utf-8',
    'cache-control': 'no-store',
  });
  res.end(value);
}

function sendDownload(res, value, contentType, filename) {
  res.writeHead(200, {
    'content-type': contentType,
    'content-disposition': `attachment; filename="${filename}"`,
    'cache-control': 'no-store',
  });
  res.end(value);
}

function getLocalPublicAsset(pathname) {
  const match = /^\/fs\/public\/([^/]+)$/.exec(pathname);
  if (!match || !localPublicAssets.has(match[1])) {
    return '';
  }

  for (const root of [frontendPublicPath, backendPublicPath]) {
    const assetPath = path.join(root, match[1]);
    if (existsSync(assetPath)) {
      return assetPath;
    }
  }

  return '';
}

function getAssetContentType(assetPath) {
  return assetPath.endsWith('.ico') ? 'image/x-icon' : 'image/png';
}

function readBody(req) {
  return new Promise((resolve) => {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => resolve(body));
    req.on('error', () => resolve(body));
  });
}

function isColorMode(color) {
  return color === 'dark' || color === 'light';
}

function isDeviceType(deviceType) {
  return typeof deviceType === 'string' && deviceTypes.has(deviceType);
}

function metadataDefaults() {
  return {
    Owner: '',
    Location: '',
    Notes: '',
    Tags: [],
    Pinned: false,
  };
}

function metadataFor(hostEntry) {
  return hostMetadata.get(hostEntry.Mac) ?? metadataDefaults();
}

function enrichHost(hostEntry) {
  const metadata = metadataFor(hostEntry);
  return {
    ...hostEntry,
    Owner: metadata.Owner,
    Location: metadata.Location,
    Notes: metadata.Notes,
    Tags: [...metadata.Tags],
    Pinned: metadata.Pinned,
    FirstSeen: hostEntry.FirstSeen ?? '',
    LastSeen: hostEntry.LastSeen ?? hostEntry.Date ?? '',
    FirstSeenEstimated: hostEntry.FirstSeenEstimated === true,
  };
}

function metadataEntries() {
  return [...hostMetadata.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([mac, metadata]) => ({
      mac,
      owner: metadata.Owner,
      location: metadata.Location,
      notes: metadata.Notes,
      tags: [...metadata.Tags],
      pinned: metadata.Pinned,
    }));
}

function lifecycleEntries() {
  return [...fakeHosts]
    .filter((hostEntry) => hostEntry.Mac)
    .sort((left, right) => left.Mac.localeCompare(right.Mac))
    .map((hostEntry) => ({
      mac: hostEntry.Mac,
      firstSeen: hostEntry.FirstSeen ?? '',
      lastSeen: hostEntry.LastSeen ?? '',
      firstSeenEstimated: hostEntry.FirstSeenEstimated === true,
    }));
}

function normalizeMetadataTags(tags) {
  if (!Array.isArray(tags)) {
    return [];
  }

  const seen = new Set();
  const normalized = [];
  for (const rawTag of tags) {
    const tag = String(rawTag ?? '').trim();
    if (!tag) {
      continue;
    }
    const key = tag.toLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    normalized.push(tag);
  }

  return normalized.slice(0, 20);
}

function normalizeInventoryOptions(values) {
  const seen = new Set();
  const normalized = [];
  for (const value of values) {
    const trimmed = String(value ?? '').trim();
    if (!trimmed) {
      continue;
    }
    const key = trimmed.toLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    normalized.push(trimmed);
  }

  return normalized.sort((left, right) => {
    const normalizedLeft = left.toLowerCase();
    const normalizedRight = right.toLowerCase();
    return normalizedLeft === normalizedRight
      ? left.localeCompare(right)
      : normalizedLeft.localeCompare(normalizedRight);
  });
}

function inventoryOptions() {
  return {
    owners: normalizeInventoryOptions(fakeHosts.map((hostEntry) => metadataFor(hostEntry).Owner)),
    locations: normalizeInventoryOptions(fakeHosts.map((hostEntry) => metadataFor(hostEntry).Location)),
  };
}

function applyMetadataPatch(hostEntry, patch) {
  const current = metadataFor(hostEntry);
  const next = {
    Owner: current.Owner,
    Location: current.Location,
    Notes: current.Notes,
    Tags: [...current.Tags],
    Pinned: current.Pinned,
  };

  if (typeof patch.owner === 'string') {
    next.Owner = patch.owner.trim();
  }
  if (typeof patch.location === 'string') {
    next.Location = patch.location.trim();
  }
  if (typeof patch.notes === 'string') {
    next.Notes = patch.notes;
  }
  if (Array.isArray(patch.tags)) {
    next.Tags = normalizeMetadataTags(patch.tags);
  }
  if (typeof patch.pinned === 'boolean') {
    next.Pinned = patch.pinned;
  }

  const eventDate = new Date();
  if (current.Owner !== next.Owner) {
    addActivity(hostEntry, 'owner-changed', { oldValue: current.Owner, newValue: next.Owner, date: eventDate });
  }
  if (current.Location !== next.Location) {
    addActivity(hostEntry, 'location-changed', { oldValue: current.Location, newValue: next.Location, date: eventDate });
  }
  if (current.Notes !== next.Notes) {
    addActivity(hostEntry, 'notes-changed', { oldValue: current.Notes, newValue: next.Notes, date: eventDate });
  }
  const oldTags = JSON.stringify(current.Tags);
  const newTags = JSON.stringify(next.Tags);
  if (oldTags !== newTags) {
    addActivity(hostEntry, 'tags-changed', { oldValue: oldTags, newValue: newTags, date: eventDate });
  }
  if (current.Pinned !== next.Pinned) {
    addActivity(hostEntry, 'pinned-changed', { oldValue: String(current.Pinned), newValue: String(next.Pinned), date: eventDate });
  }

  hostMetadata.set(hostEntry.Mac, next);
  return enrichHost(hostEntry);
}

function applyInventoryPatch(hostEntry, patch) {
  const allowedFields = new Set(['name', 'known', 'deviceType', 'owner', 'location', 'notes', 'tags']);
  for (const field of Object.keys(patch)) {
    if (!allowedFields.has(field)) {
      return { error: 'invalid request body' };
    }
  }
  if (patch.deviceType !== undefined && !isDeviceType(patch.deviceType)) {
    return { error: 'invalid deviceType' };
  }

  const eventDate = new Date();
  const oldKnown = hostEntry.Known;
  const oldDeviceType = hostEntry.DeviceType;
  if (typeof patch.name === 'string') {
    hostEntry.Name = patch.name;
  }
  if (typeof patch.known === 'boolean') {
    hostEntry.Known = patch.known ? 1 : 0;
  }
  if (patch.deviceType !== undefined) {
    hostEntry.DeviceType = patch.deviceType;
  }
  if (oldKnown !== hostEntry.Known) {
    addActivity(hostEntry, hostEntry.Known === 1 ? 'known' : 'unknown', { date: eventDate });
  }
  if (oldDeviceType !== hostEntry.DeviceType) {
    addActivity(hostEntry, 'device-type-changed', {
      oldValue: oldDeviceType,
      newValue: hostEntry.DeviceType,
      date: eventDate,
    });
  }

  return {
    host: applyMetadataPatch(hostEntry, {
      owner: patch.owner,
      location: patch.location,
      notes: patch.notes,
      tags: patch.tags,
    }),
  };
}

function isPositiveIntegerValue(value) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0;
}

function parseRequestBody(body) {
  try {
    const parsed = JSON.parse(body);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return Object.fromEntries(new URLSearchParams(body));
  }
}

function publicConfig() {
  const { ConfPath, DBPath, Version, ...publicFields } = config;

  return {
    ...publicFields,
    ShoutURL: '',
    ShoutURLConfigured: config.ShoutURL !== '',
    PGConnect: '',
    PGConnectConfigured: config.PGConnect !== '',
    InfluxToken: '',
    InfluxTokenConfigured: config.InfluxToken !== '',
  };
}

function channelLabel(channel) {
  return channel === 'stable' ? 'Stable' : 'Beta';
}

function isUpdateChannel(channel) {
  return channel === 'stable' || channel === 'beta';
}

function isUpdateIntervalHours(value) {
  return [6, 12, 24, 168].includes(Number(value));
}

function mockUpdateStatus(refresh = false) {
  if (refresh) {
    mockUpdateLastChecked = new Date().toISOString();
  }

  const latestVersion = mockUpdateAvailable ? '0.1.0-beta.4' : '0.1.0-beta.3';
  return {
    currentVersion: config.Version,
    channel: config.UpdateChannel,
    latestVersion,
    available: mockUpdateAvailable,
    publishedAt: '2026-09-16T00:00:00Z',
    releaseUrl: 'https://github.com/godlev/LANnventory/releases/tag/v' + latestVersion,
    releaseSummary: [
      'Update experience:',
      '- Separate background update checks from automatic installation.',
      '- Show a persistent update reminder in the top navigation.',
      '- Open a concise release summary inside LANnventory before visiting GitHub.',
    ].join('\n'),
    installSupported: false,
    installReason: 'Mock API does not install updates.',
    message: mockUpdateAvailable
      ? 'Update ' + latestVersion + ' is available.'
      : 'No newer ' + channelLabel(config.UpdateChannel) + ' release is available.',
    updating: false,
    automaticCheck: config.UpdateCheckAuto || config.UpdateAuto,
    automatic: config.UpdateAuto,
    intervalHours: config.UpdateCheckIntervalHours,
    lastChecked: mockUpdateLastChecked,
    snapshotBaseVersion: '',
  };
}

function isTruthyFormValue(value) {
  return ['1', 'on', 'true', 'yes'].includes(String(value ?? '').trim().toLowerCase());
}

function applySecretUpdate(currentValue, submittedValue, clearValue) {
  if (isTruthyFormValue(clearValue)) {
    return '';
  }
  if (typeof submittedValue === 'string' && submittedValue !== '') {
    return submittedValue;
  }

  return currentValue;
}

function applyBasicConfigForm(body) {
  const form = parseRequestBody(body);

  if (typeof form.host === 'string') {
    config.Host = form.host;
  }
  if (typeof form.port === 'string') {
    config.Port = form.port;
  }
  if (typeof form.theme === 'string' && /^[a-z0-9-]+$/.test(form.theme)) {
    config.Theme = form.theme;
  }
  if (isColorMode(form.color)) {
    config.Color = form.color;
  }
  if (typeof form.node === 'string') {
    config.NodePath = form.node;
  }
  config.ShoutURL = applySecretUpdate(config.ShoutURL, form.shout, form.clear_shout);
}

function applySettingsConfigForm(body) {
  const form = parseRequestBody(body);

  if (typeof form.ifaces === 'string') {
    config.Ifaces = form.ifaces;
  }
  if (typeof form.arpargs === 'string') {
    config.ArpArgs = form.arpargs;
  }
  if (typeof form.log === 'string') {
    config.LogLevel = form.log;
  }
  if (isPositiveIntegerValue(form.timeout)) {
    config.Timeout = Number(form.timeout);
  }
  if (isPositiveIntegerValue(form.trim)) {
    config.TrimHist = Number(form.trim);
  }
  if (isPositiveIntegerValue(form.connectivity_retention)) {
    config.ConnectivityRetention = Number(form.connectivity_retention);
  }
  if (typeof form.usedb === 'string') {
    config.UseDB = form.usedb;
  }
  config.PGConnect = applySecretUpdate(config.PGConnect, form.pgconnect, form.clear_pgconnect);
}

function applyInfluxConfigForm(body) {
  const form = parseRequestBody(body);

  if (typeof form.addr === 'string') {
    config.InfluxAddr = form.addr;
  }
  config.InfluxToken = applySecretUpdate(config.InfluxToken, form.token, form.clear_influx_token);
  if (typeof form.org === 'string') {
    config.InfluxOrg = form.org;
  }
  if (typeof form.bucket === 'string') {
    config.InfluxBucket = form.bucket;
  }
  config.InfluxEnable = form.enable === 'on';
  config.InfluxSkipTLS = form.skip === 'on';
}

function applyPrometheusConfigForm(body) {
  const form = parseRequestBody(body);

  config.PrometheusEnable = form.enable === 'on';
}

function applyRetentionConfigBody(body) {
  const params = parseRequestBody(body);
  const presenceRetention = Number(params.presenceRetention);
  const connectivityRetention = Number(params.connectivityRetention);

  if (!isPositiveIntegerValue(presenceRetention)) {
    return { error: 'invalid presenceRetention' };
  }
  if (!isPositiveIntegerValue(connectivityRetention)) {
    return { error: 'invalid connectivityRetention' };
  }

  config.TrimHist = presenceRetention;
  config.ConnectivityRetention = connectivityRetention;

  return { config: publicConfig() };
}

function historyFor(mac, datePrefix = '') {
  const hostEntry = fakeHosts.find((item) => item.Mac === mac) ?? fakeHosts[0];
  const rows = [];
  const baseDate = new Date('2026-08-23T10:15:00');

  for (let i = 0; i < 210; i += 1) {
    const sampleDate = new Date(baseDate.getTime() - i * config.Timeout * 1000);
    rows.push({
      ...hostEntry,
      ID: i + 1,
      Date: formatDate(sampleDate),
      Now: hostEntry.Now === 0 ? 0 : i % 13 === 0 ? 0 : 1,
    });
  }

  return datePrefix === '' ? rows : rows.filter((item) => item.Date.startsWith(datePrefix));
}

function findHostByID(id) {
  return fakeHosts.find((item) => item.ID === id);
}

function formatDate(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hour = String(date.getHours()).padStart(2, '0');
  const minute = String(date.getMinutes()).padStart(2, '0');
  const second = String(date.getSeconds()).padStart(2, '0');

  return `${year}-${month}-${day} ${hour}:${minute}:${second}`;
}

function formatDateUTC(date) {
  return date.toISOString().replace(/\.\d{3}Z$/, 'Z');
}

function downloadTimestamp(date = new Date()) {
  return date.toISOString().replace(/[-:]/g, '').replace(/\.\d{3}Z$/, 'Z');
}

function backupHostFromMock(hostEntry) {
  return {
    id: hostEntry.ID,
    name: hostEntry.Name,
    dns: hostEntry.DNS,
    iface: hostEntry.Iface,
    ip: hostEntry.IP,
    mac: hostEntry.Mac,
    hw: hostEntry.Hw,
    date: hostEntry.Date,
    known: hostEntry.Known,
    now: hostEntry.Now,
    deviceType: hostEntry.DeviceType,
  };
}

function backupEventFromMock(event) {
  return {
    id: event.ID,
    hostId: event.HostID,
    mac: event.Mac,
    name: event.Name,
    eventType: event.EventType,
    date: event.Date,
    ip: event.IP,
    iface: event.Iface,
    deviceType: event.DeviceType,
    oldValue: event.OldValue,
    newValue: event.NewValue,
  };
}

function backupDocument(createdAt = new Date()) {
  const historyRows = fakeHosts
    .flatMap((hostEntry) => historyFor(hostEntry.Mac).slice(0, 4))
    .map((hostEntry, index) => ({ ...hostEntry, ID: index + 1 }));

  return {
    format: 'lannventory-backup',
    formatVersion: 3,
    createdAt: formatDateUTC(createdAt),
    appVersion: config.Version,
    data: {
      currentHosts: [...fakeHosts].sort((left, right) => left.ID - right.ID).map(backupHostFromMock),
      history: historyRows.sort((left, right) => left.ID - right.ID).map(backupHostFromMock),
      events: [...activityEvents].sort((left, right) => left.ID - right.ID).map(backupEventFromMock),
      hostMetadata: metadataEntries(),
      hostLifecycle: lifecycleEntries(),
    },
  };
}

function csvCell(value) {
  const text = String(value ?? '');
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
}

function inventoryCSV() {
  const header = ['ID', 'Name', 'DNS', 'Iface', 'IP', 'Mac', 'Hw', 'Date', 'Known', 'Now', 'DeviceType', 'Owner', 'Location', 'Notes', 'Tags', 'Pinned', 'FirstSeen', 'FirstSeenEstimated', 'LastSeen'];
  const rows = [...fakeHosts]
    .sort((left, right) => left.ID - right.ID)
    .map((hostEntry) => {
      const enriched = enrichHost(hostEntry);
      return [
        enriched.ID,
        enriched.Name,
        enriched.DNS,
        enriched.Iface,
        enriched.IP,
        enriched.Mac,
        enriched.Hw,
        enriched.Date,
        enriched.Known,
        enriched.Now,
        enriched.DeviceType,
        enriched.Owner,
        enriched.Location,
        enriched.Notes,
        enriched.Tags.join('; '),
        enriched.Pinned,
        enriched.FirstSeen,
        enriched.FirstSeenEstimated,
        enriched.LastSeen,
      ];
    });

  return [header, ...rows].map((row) => row.map(csvCell).join(',')).join('\r\n') + '\r\n';
}

function addActivity(hostEntry, eventType, options = {}) {
  const eventDate = options.date ?? new Date();

  activityEvents.push({
    ID: nextActivityId,
    HostID: hostEntry.ID,
    Mac: hostEntry.Mac,
    Name: hostEntry.Name,
    EventType: eventType,
    Date: formatDate(eventDate),
    DateUTC: formatDateUTC(eventDate),
    IP: hostEntry.IP ?? '',
    Iface: hostEntry.Iface ?? '',
    DeviceType: hostEntry.DeviceType ?? '',
    OldValue: options.oldValue ?? '',
    NewValue: options.newValue ?? '',
  });
  nextActivityId += 1;
}

function addActivityMinutesAgo(hostEntry, eventType, minutesAgo, options = {}) {
  addActivity(hostEntry, eventType, {
    ...options,
    date: new Date(Date.now() - minutesAgo * 60000),
  });
}

function seedActivity() {
  const deletedHostSnapshot = {
    ID: 99,
    Name: 'old tablet',
    Iface: 'wifi0',
    IP: '192.168.1.70',
    Mac: 'AA:BB:CC:00:00:70',
    DeviceType: 'tablet',
  };

  addActivityMinutesAgo(fakeHosts[0], 'discovered', 1560);
  addActivityMinutesAgo(fakeHosts[1], 'discovered', 1515);
  addActivityMinutesAgo(fakeHosts[1], 'device-type-changed', 65, { oldValue: '', newValue: 'nas' });
  const metadataBatchDate = new Date(Date.now() - 75 * 60000);
  addActivity(fakeHosts[1], 'owner-changed', { oldValue: '', newValue: 'Storage Team', date: metadataBatchDate });
  addActivity(fakeHosts[1], 'location-changed', { oldValue: '', newValue: 'Rack 1', date: metadataBatchDate });
  addActivity(fakeHosts[1], 'notes-changed', { oldValue: '', newValue: 'Primary media and backup NAS.', date: metadataBatchDate });
  addActivity(fakeHosts[1], 'tags-changed', { oldValue: '[]', newValue: '["storage","backup"]', date: metadataBatchDate });
  addActivity(fakeHosts[1], 'pinned-changed', { oldValue: 'false', newValue: 'true', date: metadataBatchDate });
  addActivityMinutesAgo(fakeHosts[0], 'known', 28);
  addActivityMinutesAgo(fakeHosts[4], 'discovered', 12);
  addActivityMinutesAgo(fakeHosts[4], 'offline', 10);
  addActivityMinutesAgo(fakeHosts[3], 'offline', 8);
  addActivityMinutesAgo(fakeHosts[3], 'online', 2);
  addActivityMinutesAgo(deletedHostSnapshot, 'offline', 1440);

  for (let i = 0; i < 120; i += 1) {
    const hostEntry = i % 3 === 0 ? fakeHosts[3] : i % 3 === 1 ? fakeHosts[2] : fakeHosts[1];
    addActivityMinutesAgo(hostEntry, i % 2 === 0 ? 'online' : 'offline', 20 + i);
  }

  const changeTypes = ['discovered', 'known', 'unknown', 'device-type-changed', 'owner-changed', 'location-changed', 'notes-changed', 'tags-changed', 'pinned-changed'];
  for (let i = 0; i < 28; i += 1) {
    const hostEntry = fakeHosts[i % fakeHosts.length];
    const eventType = changeTypes[i % changeTypes.length];
    addActivityMinutesAgo(hostEntry, eventType, 90 + i * 3, {
      oldValue: eventType === 'device-type-changed' ? '' : undefined,
      newValue: eventType === 'device-type-changed'
        ? hostEntry.DeviceType
        : eventType === 'pinned-changed'
          ? String(i % 2 === 0)
          : eventType === 'tags-changed'
            ? '["mock","event"]'
            : eventType.endsWith('-changed')
              ? 'Mock value'
              : undefined,
    });
  }
}

function sortedActivityEvents() {
  return [...activityEvents].sort((left, right) => {
    const byDate = right.Date.localeCompare(left.Date);
    return byDate === 0 ? right.ID - left.ID : byDate;
  });
}

function parseActivityLimit(url) {
  const rawLimit = url.searchParams.get('limit') ?? '20';
  const limit = Number(rawLimit);
  return Number.isInteger(limit) && limit >= 1 && limit <= 100 ? limit : 0;
}

function parseActivityOffset(url) {
  const rawOffset = url.searchParams.get('offset') ?? '0';
  const offset = Number(rawOffset);
  return Number.isInteger(offset) && offset >= 0 ? offset : -1;
}

function isActivityCursorDate(value) {
  const match = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2}):(\d{2})$/.exec(value);
  if (!match) {
    return false;
  }

  const [, year, month, day, hour, minute, second] = match.map(Number);
  const date = new Date(year, month - 1, day, hour, minute, second);

  return date.getFullYear() === year
    && date.getMonth() === month - 1
    && date.getDate() === day
    && date.getHours() === hour
    && date.getMinutes() === minute
    && date.getSeconds() === second;
}

function parseActivityCursor(url, offset) {
  const hasBeforeDate = url.searchParams.has('beforeDate');
  const hasBeforeId = url.searchParams.has('beforeId');
  if (!hasBeforeDate && !hasBeforeId) {
    return { cursor: null };
  }
  if (!hasBeforeDate || !hasBeforeId) {
    return { error: 'invalid cursor' };
  }
  if (offset > 0) {
    return { error: 'offset cannot be combined with cursor' };
  }

  const beforeDate = url.searchParams.get('beforeDate') ?? '';
  const beforeId = Number(url.searchParams.get('beforeId'));
  if (!isActivityCursorDate(beforeDate) || !Number.isInteger(beforeId) || beforeId < 1) {
    return { error: 'invalid cursor' };
  }

  return { cursor: { beforeDate, beforeId } };
}

function parseActivityCategory(url) {
  const category = url.searchParams.get('category') ?? 'all';
  if (category === 'all') {
    return () => true;
  }
  if (category === 'connectivity') {
    return (event) => connectivityEvents.has(event.EventType);
  }
  if (category === 'changes') {
    return (event) => changeEvents.has(event.EventType);
  }

  return null;
}

function parseActivityEventTypeSet(url) {
  const values = url.searchParams.getAll('eventType');
  if (values.length === 0) {
    return { set: null };
  }

  for (const value of values) {
    if (!validActivityEvents.has(value)) {
      return { error: 'invalid eventType' };
    }
  }

  return { set: new Set(values) };
}

function activityMacSet(url) {
  const macs = url.searchParams.getAll('mac').filter(Boolean);
  return macs.length === 0 ? null : new Set(macs);
}

function activityFor(url, predicate = () => true) {
  const limit = parseActivityLimit(url);
  if (limit === 0) {
    return { error: 'invalid limit' };
  }

  const offset = parseActivityOffset(url);
  if (offset < 0) {
    return { error: 'invalid offset' };
  }
  const cursorResult = parseActivityCursor(url, offset);
  if (cursorResult.error) {
    return { error: cursorResult.error };
  }

  const categoryPredicate = parseActivityCategory(url);
  if (categoryPredicate === null) {
    return { error: 'invalid category' };
  }

  const eventTypeFilter = parseActivityEventTypeSet(url);
  if (eventTypeFilter.error) {
    return { error: eventTypeFilter.error };
  }

  let events = sortedActivityEvents()
    .filter(categoryPredicate)
    .filter((event) => eventTypeFilter.set === null || eventTypeFilter.set.has(event.EventType))
    .filter(predicate);

  if (cursorResult.cursor) {
    const { beforeDate, beforeId } = cursorResult.cursor;
    events = events.filter((event) => event.Date < beforeDate || (event.Date === beforeDate && event.ID < beforeId));
    return events.slice(0, limit);
  }

  return events.slice(offset, offset + limit);
}

function activityStatsFor(url) {
  const macs = activityMacSet(url);
  const stats = {
    Total: 0,
    Online: 0,
    Offline: 0,
    Discovered: 0,
    Known: 0,
    Unknown: 0,
    DeviceTypeChanged: 0,
    MetadataChanged: 0,
  };

  for (const event of activityEvents) {
    if (macs !== null && !macs.has(event.Mac)) {
      continue;
    }

    stats.Total += 1;
    if (event.EventType === 'online') stats.Online += 1;
    if (event.EventType === 'offline') stats.Offline += 1;
    if (event.EventType === 'discovered') stats.Discovered += 1;
    if (event.EventType === 'known') stats.Known += 1;
    if (event.EventType === 'unknown') stats.Unknown += 1;
    if (event.EventType === 'device-type-changed') stats.DeviceTypeChanged += 1;
    if (metadataEvents.has(event.EventType)) stats.MetadataChanged += 1;
  }

  return stats;
}

function activityDeviceOptions() {
  const seen = new Set();
  const options = [];

  for (const hostEntry of fakeHosts) {
    if (!hostEntry.Mac || seen.has(hostEntry.Mac)) {
      continue;
    }

    seen.add(hostEntry.Mac);
    options.push({
      HostID: hostEntry.ID,
      Mac: hostEntry.Mac,
      Name: hostEntry.Name,
      IP: hostEntry.IP,
      DeviceType: hostEntry.DeviceType,
      Exists: true,
    });
  }

  for (const event of sortedActivityEvents()) {
    if (!event.Mac || seen.has(event.Mac)) {
      continue;
    }

    seen.add(event.Mac);
    options.push({
      HostID: event.HostID,
      Mac: event.Mac,
      Name: event.Name,
      IP: event.IP,
      DeviceType: event.DeviceType,
      Exists: false,
    });
  }

  return options;
}


function mockScannerStatus() {
  const serverTime = new Date();
  const lastScanAt = new Date(serverTime.getTime() - 78 * 1000);
  const nextScanAt = new Date(serverTime.getTime() + 42 * 1000);
  return {
    status: 'healthy',
    scanning: false,
    lastScanStartedAt: new Date(lastScanAt.getTime() - 2800).toISOString(),
    lastScanAt: lastScanAt.toISOString(),
    lastSuccessfulScanAt: lastScanAt.toISOString(),
    durationMs: 2800,
    devicesFound: fakeHosts.filter((hostEntry) => hostEntry.Now > 0).length,
    interfaces: String(config.Ifaces || '').split(/\s+/).filter(Boolean),
    lastError: null,
    nextScanAt: nextScanAt.toISOString(),
    serverTime: serverTime.toISOString(),
    database: {
      status: 'connected',
      backend: 'sqlite',
    },
  };
}

function mockDiagnostics() {
  const scanner = mockScannerStatus();
  return {
    ok: true,
    generatedAt: scanner.serverTime,
    checks: [
      { check: 'LANnventory', status: 'ok', details: 'Running v' + String(config.Version).replace(/^v/, '') },
      { check: 'Database', status: 'ok', details: 'SQLite connected' },
      { check: 'arp-scan', status: 'ok', details: '/usr/bin/arp-scan' },
      { check: 'Permissions', status: 'ok', details: 'Mock development environment' },
      { check: 'Interface', status: 'ok', details: (scanner.interfaces.join(', ') || 'eth0') + ' available' },
      { check: 'Scanner', status: 'ok', details: 'Healthy' },
      { check: 'Last scan', status: 'ok', details: scanner.devicesFound + ' devices; completed ' + scanner.lastScanAt },
      { check: 'Last successful scan', status: 'ok', details: scanner.lastSuccessfulScanAt },
      { check: 'Scan duration', status: 'ok', details: '2.8 sec' },
      { check: 'Scanner errors', status: 'ok', details: 'No scanner errors captured' },
    ],
  };
}

function routeReadOnly(req, res, url) {
  const pathname = decodeURIComponent(url.pathname);

  if (req.method === 'GET' && pathname === '/api/config') {
    sendJSON(res, publicConfig());
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/health') {
    sendText(res, 'OK');
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/version') {
    sendJSON(res, config.Version);
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/scanner/status') {
    sendJSON(res, mockScannerStatus());
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/diagnostics') {
    sendJSON(res, mockDiagnostics());
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/update/status') {
    sendJSON(res, mockUpdateStatus(url.searchParams.get('refresh') === '1' || url.searchParams.get('refresh') === 'true'));
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/all') {
    sendJSON(res, fakeHosts.map(enrichHost));
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/infrastructure/workload-memberships') {
    const hostId = Number(url.searchParams.get('hostId') ?? 0);
    sendJSON(res, mockWorkloadMemberships(Number.isFinite(hostId) ? hostId : 0));
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/inventory/options') {
    sendJSON(res, inventoryOptions());
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/export/backup') {
    const createdAt = new Date();
    sendDownload(
      res,
      JSON.stringify(backupDocument(createdAt), null, 2) + '\n',
      'application/json; charset=utf-8',
      `lannventory-backup-${downloadTimestamp(createdAt)}.json`,
    );
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/export/inventory.csv') {
    const createdAt = new Date();
    sendDownload(
      res,
      inventoryCSV(),
      'text/csv; charset=utf-8',
      `lannventory-inventory-${downloadTimestamp(createdAt)}.csv`,
    );
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/activity/stats') {
    sendJSON(res, activityStatsFor(url));
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/activity/devices') {
    sendJSON(res, activityDeviceOptions());
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/activity') {
    const macs = activityMacSet(url);
    const events = activityFor(url, (event) => macs === null || macs.has(event.Mac));
    if (events.error) {
      sendJSON(res, { error: events.error }, 400);
      return true;
    }

    sendJSON(res, events);
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/history') {
    sendJSON(res, fakeHosts.flatMap((item) => historyFor(item.Mac)));
    return true;
  }

  const hostActivityMatch = pathname.match(/^\/api\/host\/(\d+)\/activity$/);
  if (req.method === 'GET' && hostActivityMatch) {
    const id = Number(hostActivityMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry) {
      sendJSON(res, { error: 'invalid host id' }, 400);
      return true;
    }

    const events = activityFor(url, (event) => event.HostID === id);
    if (events.error) {
      sendJSON(res, { error: events.error }, 400);
      return true;
    }

    sendJSON(res, events);
    return true;
  }

  const proxmoxSourceMatch = pathname.match(/^\/api\/host\/(\d+)\/proxmox\/source-state$/);
  if (req.method === 'GET' && proxmoxSourceMatch) {
    const id = Number(proxmoxSourceMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry || profileForHost(hostEntry).hypervisor?.platform !== 'proxmox-ve') {
      sendJSON(res, { error: 'Proxmox source state requires a Proxmox VE hypervisor profile' }, 400);
      return true;
    }
    sendJSON(res, proxmoxSourceStates.get(id) ?? null);
    return true;
  }

  const proxmoxWorkloadsMatch = pathname.match(/^\/api\/host\/(\d+)\/workloads$/);
  if (req.method === 'GET' && proxmoxWorkloadsMatch) {
    const id = Number(proxmoxWorkloadsMatch[1]);
    sendJSON(res, hydrateMockWorkloads(id));
    return true;
  }

  const workloadMatchesMatch = pathname.match(/^\/api\/host\/(\d+)\/workload-matches$/);
  if (req.method === 'GET' && workloadMatchesMatch) {
    const id = Number(workloadMatchesMatch[1]);
    sendJSON(res, workloadMatchesForHost(id));
    return true;
  }

  const hostProfileMatch = pathname.match(/^\/api\/host\/(\d+)\/profile$/);
  if (req.method === 'GET' && hostProfileMatch) {
    const id = Number(hostProfileMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry) {
      sendJSON(res, { error: 'invalid host id' }, 400);
      return true;
    }
    sendJSON(res, profileForHost(hostEntry));
    return true;
  }

  const hostMatch = pathname.match(/^\/api\/host\/(\d+)$/);
  if (req.method === 'GET' && hostMatch) {
    const id = Number(hostMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry) {
      sendJSON(res, { error: 'invalid host id' }, 400);
      return true;
    }

    sendJSON(res, enrichHost(hostEntry));
    return true;
  }

  const historyMatch = pathname.match(/^\/api\/history\/([^/]+)\/?$/);
  if (req.method === 'GET' && historyMatch) {
    sendJSON(res, historyFor(historyMatch[1]));
    return true;
  }

  const historyDateMatch = pathname.match(/^\/api\/history\/([^/]+)\/(.+)$/);
  if (req.method === 'GET' && historyDateMatch) {
    sendJSON(res, historyFor(historyDateMatch[1], historyDateMatch[2]));
    return true;
  }

  const publicAssetPath = req.method === 'GET' ? getLocalPublicAsset(pathname) : '';
  if (publicAssetPath) {
    res.writeHead(200, {
      'content-type': getAssetContentType(publicAssetPath),
      'cache-control': 'no-store',
    });
    createReadStream(publicAssetPath).pipe(res);
    return true;
  }

  return false;
}

async function routeSafeAction(req, res, url) {
  const pathname = decodeURIComponent(url.pathname);

  if (req.method === 'GET' && pathname === '/api/notify_test') {
    sendText(res, 'mock notification skipped');
    return true;
  }

  if (req.method === 'GET' && pathname === '/api/rescan') {
    sendText(res, 'mock rescan skipped');
    return true;
  }

  if (req.method === 'GET' && pathname.startsWith('/api/edit/')) {
    const editMatch = pathname.match(/^\/api\/edit\/(\d+)\/([^/]*)(?:\/(.*))?$/);

    if (editMatch) {
      const id = Number(editMatch[1]);
      const name = editMatch[2];
      const action = editMatch[3] ?? '';
      const hostEntry = findHostByID(id);

      if (hostEntry) {
        const oldKnown = hostEntry.Known;
        hostEntry.Name = name;

        if (action === 'toggle') {
          hostEntry.Known = 1 - hostEntry.Known;
          if (oldKnown !== hostEntry.Known) {
            addActivity(hostEntry, hostEntry.Known === 1 ? 'known' : 'unknown');
          }
        }
      }
    }

    sendJSON(res, 'OK');
    return true;
  }

  const deviceTypeMatch = pathname.match(/^\/api\/host\/(\d+)\/type$/);
  if (req.method === 'PATCH' && deviceTypeMatch) {
    const id = Number(deviceTypeMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry) {
      sendJSON(res, { error: 'invalid host id' }, 400);
      return true;
    }

    const body = await readBody(req);
    const params = parseRequestBody(body);
    if (!isDeviceType(params.deviceType)) {
      sendJSON(res, { error: 'invalid deviceType' }, 400);
      return true;
    }

    const oldDeviceType = hostEntry.DeviceType;
    hostEntry.DeviceType = params.deviceType;
    if (oldDeviceType !== hostEntry.DeviceType) {
      addActivity(hostEntry, 'device-type-changed', {
        oldValue: oldDeviceType,
        newValue: hostEntry.DeviceType,
      });
    }
    sendJSON(res, enrichHost(hostEntry));
    return true;
  }

  const metadataMatch = pathname.match(/^\/api\/host\/(\d+)\/metadata$/);
  if (req.method === 'PATCH' && metadataMatch) {
    const id = Number(metadataMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry) {
      sendJSON(res, { error: 'invalid host id' }, 400);
      return true;
    }

    const body = await readBody(req);
    const params = parseRequestBody(body);
    sendJSON(res, applyMetadataPatch(hostEntry, params));
    return true;
  }

  const profileMatch = pathname.match(/^\/api\/host\/(\d+)\/profile(?:\/(network|system|hypervisor))?$/);
  if ((req.method === 'PATCH' || req.method === 'DELETE') && profileMatch) {
    const id = Number(profileMatch[1]);
    const layer = profileMatch[2] ?? 'managed';
    const hostEntry = findHostByID(id);
    if (!hostEntry) {
      sendJSON(res, { error: 'invalid host id' }, 400);
      return true;
    }

    const profile = profileForHost(hostEntry);
    if (req.method === 'DELETE') {
      if (layer !== 'hypervisor') {
        sendJSON(res, { error: 'unsupported profile delete' }, 400);
        return true;
      }
      profile.hypervisor = null;
      deviceProfiles.set(hostEntry.Mac, profile);
      sendJSON(res, profile);
      return true;
    }

    const params = parseRequestBody(await readBody(req));
    const updatedAt = new Date().toISOString();
    if (layer === 'managed') {
      const next = {
        mac: hostEntry.Mac,
        manufacturer: String(params.manufacturer ?? profile.managed?.manufacturer ?? ''),
        model: String(params.model ?? profile.managed?.model ?? ''),
        managementAddress: String(params.managementAddress ?? profile.managed?.managementAddress ?? ''),
        updatedAt,
      };
      profile.managed = next.manufacturer || next.model || next.managementAddress ? next : null;
    } else if (layer === 'network') {
      const next = {
        mac: hostEntry.Mac,
        managementMode: String(params.managementMode ?? profile.network?.managementMode ?? ''),
        physicalPortCount: Number(params.physicalPortCount ?? profile.network?.physicalPortCount ?? 0),
        portCapabilityNotes: String(params.portCapabilityNotes ?? profile.network?.portCapabilityNotes ?? ''),
        updatedAt,
      };
      profile.network = next.managementMode || next.physicalPortCount || next.portCapabilityNotes ? next : null;
    } else if (layer === 'system') {
      const next = {
        mac: hostEntry.Mac,
        role: String(params.role ?? profile.system?.role ?? ''),
        operatingSystem: String(params.operatingSystem ?? profile.system?.operatingSystem ?? ''),
        version: String(params.version ?? profile.system?.version ?? ''),
        updatedAt,
      };
      profile.system = next.role || next.operatingSystem || next.version ? next : null;
    } else {
      const platform = String(params.platform ?? profile.hypervisor?.platform ?? '');
      if (!['proxmox-ve', 'vmware-esxi', 'hyper-v', 'other'].includes(platform)) {
        sendJSON(res, { error: 'invalid hypervisor platform' }, 400);
        return true;
      }
      profile.hypervisor = {
        mac: hostEntry.Mac,
        platform,
        version: String(params.version ?? profile.hypervisor?.version ?? ''),
        nodeName: String(params.nodeName ?? profile.hypervisor?.nodeName ?? ''),
        clusterName: String(params.clusterName ?? profile.hypervisor?.clusterName ?? ''),
        updatedAt,
      };
    }

    deviceProfiles.set(hostEntry.Mac, profile);
    sendJSON(res, profile);
    return true;
  }

  const workloadRejectMatch = pathname.match(/^\/api\/host\/(\d+)\/workloads\/(\d+)\/match-rejections\/(\d+)$/);
  if ((req.method === 'PUT' || req.method === 'DELETE') && workloadRejectMatch) {
    const hostId = Number(workloadRejectMatch[1]);
    const workloadId = Number(workloadRejectMatch[2]);
    const candidateHostId = Number(workloadRejectMatch[3]);
    const match = workloadMatchesForHost(hostId).find((item) => item.workloadId === workloadId);
    const candidate = match?.candidates.find((item) => item.hostId === candidateHostId);
    const key = workloadId+':'+candidateHostId;
    if (req.method === 'DELETE') {
      workloadCandidateRejections.delete(key);
      res.writeHead(204);
      res.end();
      return true;
    }
    if (!candidate) {
      sendJSON(res, { error: 'candidate is no longer supported by current evidence' }, 409);
      return true;
    }
    if (candidate.strength === 'exact-mac') {
      sendJSON(res, { error: 'exact MAC candidates cannot be rejected as weak matches' }, 400);
      return true;
    }
    const params = parseRequestBody(await readBody(req));
    if (String(params.evidenceFingerprint ?? '') !== candidate.evidenceFingerprint) {
      sendJSON(res, { error: 'candidate evidence changed; review the refreshed match before rejecting it' }, 409);
      return true;
    }
    workloadCandidateRejections.set(key, candidate.evidenceFingerprint);
    sendJSON(res, { ...candidate, rejected: true });
    return true;
  }

  const workloadLinkMatch = pathname.match(/^\/api\/host\/(\d+)\/workloads\/(\d+)\/link$/);
  if ((req.method === 'PUT' || req.method === 'DELETE') && workloadLinkMatch) {
    const hostId = Number(workloadLinkMatch[1]);
    const workloadId = Number(workloadLinkMatch[2]);
    const items = proxmoxWorkloads.get(hostId) ?? [];
    const workload = items.find((item) => item.id === workloadId);
    if (!workload) {
      sendJSON(res, { error: 'workload does not belong to this hypervisor' }, 400);
      return true;
    }
    if (req.method === 'DELETE') {
      workload.link = null;
      workload.matchedHost = null;
      sendJSON(res, structuredClone(workload));
      return true;
    }
    const params = parseRequestBody(await readBody(req));
    const target = findHostByID(Number(params.hostId));
    if (!target || target.ID === hostId) {
      sendJSON(res, { error: 'target host does not exist' }, 400);
      return true;
    }
    const changedAt = new Date().toISOString();
    workload.link = {
      workloadId,
      hostId: target.ID,
      hostMac: target.Mac,
      linkSource: 'manual',
      linkedAt: changedAt,
      updatedAt: changedAt,
    };
    workload.matchedHost = hostSummary(target);
    sendJSON(res, structuredClone(workload));
    return true;
  }

  const proxmoxPreviewMatch = pathname.match(/^\/api\/host\/(\d+)\/proxmox\/import\/preview$/);
  if (req.method === 'POST' && proxmoxPreviewMatch) {
    const id = Number(proxmoxPreviewMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry || profileForHost(hostEntry).hypervisor?.platform !== 'proxmox-ve') {
      sendJSON(res, { error: 'Proxmox import requires a Proxmox VE hypervisor profile' }, 400);
      return true;
    }
    const snapshot = parseRequestBody(await readBody(req));
    if (snapshot.schemaVersion !== 1 || snapshot.source !== 'script-import' || !snapshot.node || !Array.isArray(snapshot.workloads)) {
      sendJSON(res, { error: 'invalid Proxmox import JSON' }, 400);
      return true;
    }
    sendJSON(res, mockImportPreview(id, snapshot));
    return true;
  }

  const proxmoxApplyMatch = pathname.match(/^\/api\/host\/(\d+)\/proxmox\/import\/apply$/);
  if (req.method === 'POST' && proxmoxApplyMatch) {
    const id = Number(proxmoxApplyMatch[1]);
    const params = parseRequestBody(await readBody(req));
    if (params.confirmed !== true) {
      sendJSON(res, { error: 'explicit confirmation is required' }, 400);
      return true;
    }
    const result = applyMockProxmoxSnapshot(id, params.snapshot ?? {}, String(params.previewToken ?? ''));
    if (result.error) {
      sendJSON(res, { error: result.error }, result.status ?? 400);
      return true;
    }
    sendJSON(res, result);
    return true;
  }

  const inventoryMatch = pathname.match(/^\/api\/host\/(\d+)$/);
  if (req.method === 'PATCH' && inventoryMatch) {
    const id = Number(inventoryMatch[1]);
    const hostEntry = findHostByID(id);
    if (!hostEntry) {
      sendJSON(res, { error: 'invalid host id' }, 400);
      return true;
    }

    const body = await readBody(req);
    const params = parseRequestBody(body);
    const result = applyInventoryPatch(hostEntry, params);
    if (result.error) {
      sendJSON(res, { error: result.error }, 400);
      return true;
    }

    sendJSON(res, result.host);
    return true;
  }

  if (req.method === 'GET' && pathname.startsWith('/api/host/del/')) {
    sendJSON(res, 'OK');
    return true;
  }

  if (req.method === 'GET' && pathname.startsWith('/api/host/add/')) {
    sendJSON(res, enrichHost(fakeHosts[0]));
    return true;
  }

  if (req.method === 'GET' && pathname.startsWith('/api/wol/')) {
    sendJSON(res, true);
    return true;
  }

  if (req.method === 'GET' && pathname.startsWith('/api/port/')) {
    sendJSON(res, false);
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/update/channel') {
    const body = await readBody(req);
    const params = parseRequestBody(body);
    const channel = String(params.channel ?? '').trim().toLowerCase();
    if (!isUpdateChannel(channel)) {
      sendJSON(res, { error: 'update channel must be stable or beta' }, 400);
      return true;
    }

    config.UpdateChannel = channel;
    sendJSON(res, mockUpdateStatus(true));
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/update/settings') {
    const body = await readBody(req);
    const params = parseRequestBody(body);
    const channel = String(params.channel ?? '').trim().toLowerCase();
    const intervalHours = Number(params.intervalHours);
    if (!isUpdateChannel(channel)) {
      sendJSON(res, { error: 'update channel must be stable or beta' }, 400);
      return true;
    }
    if (!isUpdateIntervalHours(intervalHours)) {
      sendJSON(res, { error: 'update interval must be 6, 12, 24, or 168 hours' }, 400);
      return true;
    }

    config.UpdateChannel = channel;
    const automatic = params.automatic === true || params.automatic === 'true' || params.automatic === 'on';
    const automaticCheck = params.automaticCheck === true || params.automaticCheck === 'true' || params.automaticCheck === 'on';
    config.UpdateCheckAuto = automaticCheck || automatic;
    config.UpdateAuto = automatic && config.UpdateCheckAuto;
    config.UpdateCheckIntervalHours = intervalHours;
    sendJSON(res, mockUpdateStatus(true));
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/update/apply') {
    await readBody(req);
    sendJSON(res, { error: 'mock update skipped' }, 409);
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/config/color') {
    const body = await readBody(req);
    const params = parseRequestBody(body);
    const color = params.color ?? params.Color;

    if (!isColorMode(color)) {
      sendJSON(res, { error: 'invalid color' }, 400);
      return true;
    }

    config.Color = color;
    sendJSON(res, publicConfig());
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/config/retention') {
    const body = await readBody(req);
    const result = applyRetentionConfigBody(body);
    if (result.error) {
      sendJSON(res, { error: result.error }, 400);
      return true;
    }

    sendJSON(res, result.config);
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/config/') {
    const body = await readBody(req);
    applyBasicConfigForm(body);
    const referer = req.headers.referer || '/config';
    res.writeHead(303, { location: referer });
    res.end();
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/config_settings/') {
    const body = await readBody(req);
    applySettingsConfigForm(body);
    const referer = req.headers.referer || '/config';
    res.writeHead(303, { location: referer });
    res.end();
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/config_influx/') {
    const body = await readBody(req);
    applyInfluxConfigForm(body);
    const referer = req.headers.referer || '/config';
    res.writeHead(303, { location: referer });
    res.end();
    return true;
  }

  if (req.method === 'POST' && pathname === '/api/config_prometheus/') {
    const body = await readBody(req);
    applyPrometheusConfigForm(body);
    const referer = req.headers.referer || '/config';
    res.writeHead(303, { location: referer });
    res.end();
    return true;
  }

  if (req.method === 'POST' && pathname.startsWith('/api/config')) {
    await readBody(req);
    const referer = req.headers.referer || '/config';
    res.writeHead(303, { location: referer });
    res.end();
    return true;
  }

  return false;
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url ?? '/', `http://${host}:${port}`);

  if (routeReadOnly(req, res, url)) {
    return;
  }

  if (await routeSafeAction(req, res, url)) {
    return;
  }

  sendJSON(res, { error: 'mock endpoint not found' }, 404);
});

seedActivity();

server.listen(port, host, () => {
  console.log(`LANnventory mock API listening at http://${host}:${port}`);
});
