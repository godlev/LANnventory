import { createEffect, createMemo, createSignal, Show } from "solid-js";

import {
  apiDeleteHostHypervisorProfile,
  apiGetHostDeviceProfile,
  apiPatchHostDeviceProfile,
  apiPatchHostHypervisorProfile,
  apiPatchHostNetworkDeviceProfile,
  apiPatchHostSystemDeviceProfile,
  type DeviceProfileResponse,
} from "../../functions/api";
import type { Host } from "../../functions/exports";

type ProfileCardProps = {
  host: Host;
  onProfileChange?: (profile: DeviceProfileResponse) => void;
};

type ProfileDraft = {
  manufacturer: string;
  model: string;
  managementAddress: string;
  managementMode: "" | "managed" | "unmanaged";
  physicalPortCount: string;
  portCapabilityNotes: string;
  role: string;
  operatingSystem: string;
  systemVersion: string;
  hypervisorPlatform: "" | "proxmox-ve" | "vmware-esxi" | "hyper-v" | "other";
  hypervisorVersion: string;
  nodeName: string;
  clusterName: string;
};

const emptyProfile: DeviceProfileResponse = {
  managed: null,
  network: null,
  system: null,
  hypervisor: null,
};

function DeviceProfileCard(props: ProfileCardProps) {
  const [profile, setProfile] = createSignal<DeviceProfileResponse>(emptyProfile);
  const [draft, setDraft] = createSignal<ProfileDraft>(draftFromProfile(emptyProfile));
  const [baseline, setBaseline] = createSignal<ProfileDraft>(draftFromProfile(emptyProfile));
  const [editing, setEditing] = createSignal(false);
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal("");
  const [status, setStatus] = createSignal("");
  let requestID = 0;

  const isNetworkType = createMemo(() => ["router", "switch", "access-point"].includes(props.host.DeviceType));
  const isSystemType = createMemo(() => ["server", "nas"].includes(props.host.DeviceType));
  const showNetwork = createMemo(() => isNetworkType() || profile().network !== null);
  const showSystem = createMemo(() => isSystemType() || profile().system !== null || profile().hypervisor !== null);
  const showHypervisor = createMemo(() => props.host.DeviceType === "server" || profile().hypervisor !== null);
  const dirty = createMemo(() => JSON.stringify(draft()) !== JSON.stringify(baseline()));

  createEffect(() => {
    const id = props.host.ID;
    if (id < 1) {
      requestID++;
      setProfile(emptyProfile);
      props.onProfileChange?.(emptyProfile);
      setDraft(draftFromProfile(emptyProfile));
      setBaseline(draftFromProfile(emptyProfile));
      setLoading(false);
      setError("");
      setStatus("");
      setEditing(false);
      return;
    }
    void loadProfile(id);
  });

  const loadProfile = async (id: number) => {
    const active = ++requestID;
    setLoading(true);
    setError("");
    try {
      const next = await apiGetHostDeviceProfile(id);
      if (active !== requestID) return;
      applyProfile(next);
    } catch {
      if (active !== requestID) return;
      setProfile(emptyProfile);
      props.onProfileChange?.(emptyProfile);
      setError("Device profile could not be loaded.");
    } finally {
      if (active === requestID) setLoading(false);
    }
  };

  const applyProfile = (next: DeviceProfileResponse) => {
    setProfile(next);
    props.onProfileChange?.(next);
    const nextDraft = draftFromProfile(next);
    setDraft(nextDraft);
    setBaseline(nextDraft);
  };

  const cancelEdit = () => {
    setDraft(baseline());
    setEditing(false);
    setError("");
    setStatus("");
  };

  const saveProfile = async () => {
    if (props.host.ID < 1 || saving()) return;

    const nextDraft = draft();
    const ports = parsePortCount(nextDraft.physicalPortCount);
    if (ports === null) {
      setError("Physical ports must be a whole number from 0 to 65535.");
      return;
    }

    setSaving(true);
    setError("");
    setStatus("");

    try {
      let next = await apiPatchHostDeviceProfile(props.host.ID, {
        manufacturer: nextDraft.manufacturer,
        model: nextDraft.model,
        managementAddress: nextDraft.managementAddress,
      });

      if (showNetwork()) {
        next = await apiPatchHostNetworkDeviceProfile(props.host.ID, {
          managementMode: nextDraft.managementMode,
          physicalPortCount: ports,
          portCapabilityNotes: nextDraft.portCapabilityNotes,
        });
      }

      if (showSystem()) {
        next = await apiPatchHostSystemDeviceProfile(props.host.ID, {
          role: nextDraft.role,
          operatingSystem: nextDraft.operatingSystem,
          version: nextDraft.systemVersion,
        });
      }

      const hadHypervisor = profile().hypervisor !== null;
      if (showHypervisor()) {
        if (nextDraft.hypervisorPlatform === "") {
          if (hadHypervisor) {
            next = await apiDeleteHostHypervisorProfile(props.host.ID);
          }
        } else {
          next = await apiPatchHostHypervisorProfile(props.host.ID, {
            platform: nextDraft.hypervisorPlatform,
            version: nextDraft.hypervisorVersion,
            nodeName: nextDraft.nodeName,
            clusterName: nextDraft.clusterName,
          });
        }
      }

      applyProfile(next);
      setEditing(false);
      setStatus("Profile saved.");
    } catch {
      setError("The complete profile could not be saved. Persisted values were reloaded.");
      try {
        const persisted = await apiGetHostDeviceProfile(props.host.ID);
        applyProfile(persisted);
      } catch {
        // Keep the original save error visible when reload also fails.
      }
    } finally {
      setSaving(false);
    }
  };

  const updateDraft = <K extends keyof ProfileDraft>(key: K, value: ProfileDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
    setStatus("");
    setError("");
  };

  return (
    <section class="card wyl-panel host-panel" aria-labelledby="device-profile-title">
      <div class="card-header host-panel-header">
        <div>
          <div id="device-profile-title" class="host-panel-title">Device profile</div>
          <div class="host-panel-subtitle">
            Managed inventory only. Imported Proxmox data is kept separate and will not overwrite these values.
          </div>
        </div>
        <div class="d-flex align-items-center gap-2 flex-wrap justify-content-end">
          <span class="host-detail-section-badge">Managed</span>
          <Show
            when={editing()}
            fallback={
              <button type="button" class="btn btn-sm btn-outline-primary" disabled={loading() || props.host.ID < 1} onClick={() => setEditing(true)}>
                <i class="bi bi-pencil-square me-1" aria-hidden="true"></i>
                Edit profile
              </button>
            }
          >
            <button type="button" class="btn btn-sm btn-outline-secondary" disabled={saving()} onClick={cancelEdit}>Cancel</button>
            <button type="button" class="btn btn-sm btn-primary" disabled={saving() || !dirty()} onClick={() => void saveProfile()}>
              <Show when={!saving()} fallback={<><span class="spinner-border spinner-border-sm me-1" aria-hidden="true"></span>Saving</>}>
                <i class="bi bi-check2 me-1" aria-hidden="true"></i>
                Save
              </Show>
            </button>
          </Show>
        </div>
      </div>

      <div class="card-body">
        <DataSourceLegend />
        <Show when={!loading()} fallback={<div class="device-cell-muted">Loading device profile…</div>}>
          <div class="row g-3">
            <div class="col-12">
              <div class="small fw-semibold mb-2">General</div>
              <div class="row g-2">
                <ProfileField label="Manufacturer" editing={editing()} value={draft().manufacturer} onInput={(value) => updateDraft("manufacturer", value)} />
                <ProfileField label="Model" editing={editing()} value={draft().model} onInput={(value) => updateDraft("model", value)} />
                <ProfileField label="Management address" editing={editing()} value={draft().managementAddress} onInput={(value) => updateDraft("managementAddress", value)} monospace />
              </div>
            </div>

            <Show when={showNetwork()}>
              <div class="col-12">
                <hr class="my-1" />
                <div class="small fw-semibold mb-2">Network device</div>
                <div class="row g-2">
                  <div class="col-12 col-lg-4">
                    <FieldLabel label="Management" source="manual" />
                    <Show when={editing()} fallback={<ProfileValue value={managementModeLabel(draft().managementMode)} />}>
                      <select class="form-select form-select-sm wyl-control" value={draft().managementMode} onChange={(event) => updateDraft("managementMode", event.currentTarget.value as ProfileDraft["managementMode"])}>
                        <option value="">Not set</option>
                        <option value="managed">Managed</option>
                        <option value="unmanaged">Unmanaged</option>
                      </select>
                    </Show>
                  </div>
                  <div class="col-12 col-lg-4">
                    <FieldLabel label="Physical ports" source="manual" />
                    <Show when={editing()} fallback={<ProfileValue value={draft().physicalPortCount === "" || draft().physicalPortCount === "0" ? "" : draft().physicalPortCount} />}>
                      <input class="form-control form-control-sm wyl-control" type="number" min="0" max="65535" step="1" value={draft().physicalPortCount} onInput={(event) => updateDraft("physicalPortCount", event.currentTarget.value)} />
                    </Show>
                  </div>
                  <div class="col-12">
                    <FieldLabel label="Port capability notes" source="manual" />
                    <Show when={editing()} fallback={<ProfileValue value={draft().portCapabilityNotes} />}>
                      <textarea class="form-control form-control-sm wyl-control" rows={2} value={draft().portCapabilityNotes} onInput={(event) => updateDraft("portCapabilityNotes", event.currentTarget.value)}></textarea>
                    </Show>
                  </div>
                </div>
              </div>
            </Show>

            <Show when={showSystem()}>
              <div class="col-12">
                <hr class="my-1" />
                <div class="small fw-semibold mb-2">System</div>
                <div class="row g-2">
                  <ProfileField label="Role" editing={editing()} value={draft().role} onInput={(value) => updateDraft("role", value)} />
                  <ProfileField label="Operating system / platform" editing={editing()} value={draft().operatingSystem} onInput={(value) => updateDraft("operatingSystem", value)} />
                  <ProfileField label="Version" editing={editing()} value={draft().systemVersion} onInput={(value) => updateDraft("systemVersion", value)} />
                </div>
              </div>
            </Show>

            <Show when={showHypervisor()}>
              <div class="col-12">
                <hr class="my-1" />
                <div class="d-flex align-items-center justify-content-between gap-2 mb-2">
                  <div>
                    <div class="small fw-semibold">Hypervisor</div>
                    <div class="small device-cell-muted">Capability profile; the Host remains Device Type Server. Platform is selected manually. Version, node and cluster are optional reference values; imported observations appear separately below.</div>
                  </div>
                  <Show when={draft().hypervisorPlatform === "proxmox-ve"}>
                    <span class="badge text-bg-secondary">Proxmox VE</span>
                  </Show>
                </div>
                <div class="row g-2">
                  <div class="col-12 col-lg-4">
                    <FieldLabel label="Platform" source="manual" />
                    <Show when={editing()} fallback={<ProfileValue value={hypervisorPlatformLabel(draft().hypervisorPlatform)} />}>
                      <select class="form-select form-select-sm wyl-control" value={draft().hypervisorPlatform} onChange={(event) => updateDraft("hypervisorPlatform", event.currentTarget.value as ProfileDraft["hypervisorPlatform"])}>
                        <option value="">Not a hypervisor</option>
                        <option value="proxmox-ve">Proxmox VE</option>
                        <option value="vmware-esxi">VMware ESXi</option>
                        <option value="hyper-v">Hyper-V</option>
                        <option value="other">Other</option>
                      </select>
                    </Show>
                  </div>
                  <Show when={draft().hypervisorPlatform !== ""}>
                    <ProfileField label="Hypervisor version" editing={editing()} value={draft().hypervisorVersion} onInput={(value) => updateDraft("hypervisorVersion", value)} hint="Optional manual reference" />
                    <ProfileField label="Node name" editing={editing()} value={draft().nodeName} onInput={(value) => updateDraft("nodeName", value)} monospace hint="Optional manual reference" />
                    <ProfileField label="Cluster name" editing={editing()} value={draft().clusterName} onInput={(value) => updateDraft("clusterName", value)} hint="Optional manual reference" />
                  </Show>
                </div>
              </div>
            </Show>
          </div>

          <Show when={error()}>
            <div class="host-inline-error mt-3" role="alert">{error()}</div>
          </Show>
          <Show when={status()}>
            <div class="small text-success mt-3">{status()}</div>
          </Show>
        </Show>
      </div>
    </section>
  );
}

function ProfileField(props: {
  label: string;
  editing: boolean;
  value: string;
  onInput: (value: string) => void;
  monospace?: boolean;
  hint?: string;
}) {
  return (
    <div class="col-12 col-lg-4">
      <FieldLabel label={props.label} source="manual" />
      <Show when={props.editing} fallback={<ProfileValue value={props.value} monospace={props.monospace} />}>
        <input
          class={"form-control form-control-sm wyl-control"+(props.monospace ? " font-monospace" : "")}
          type="text"
          value={props.value}
          onInput={(event) => props.onInput(event.currentTarget.value)}
        />
      </Show>
      <Show when={props.hint}>
        <div class="profile-field-hint">{props.hint}</div>
      </Show>
    </div>
  );
}

type DataSource = "manual" | "discovered" | "imported";

function DataSourceLegend() {
  return (
    <div class="profile-source-legend" aria-label="Data source legend">
      <span class="profile-source-legend-title">Data source</span>
      <SourceLegendItem source="manual" label="Manual" />
      <SourceLegendItem source="discovered" label="Discovered" />
      <SourceLegendItem source="imported" label="Imported" />
    </div>
  );
}

function SourceLegendItem(props: { source: DataSource; label: string }) {
  return (
    <span class="profile-source-legend-item" title={sourceDescription(props.source)}>
      <SourceIcon source={props.source} />
      {props.label}
    </span>
  );
}

function FieldLabel(props: { label: string; source: DataSource }) {
  return (
    <label class="form-label small mb-1 profile-field-label">
      <span>{props.label}</span>
      <span
        class={"profile-source-icon is-"+props.source}
        title={sourceDescription(props.source)}
        aria-label={sourceDescription(props.source)}
        tabindex="0"
      >
        <SourceIcon source={props.source} />
      </span>
    </label>
  );
}

function SourceIcon(props: { source: DataSource }) {
  const icon = () => {
    if (props.source === "manual") return "bi bi-pencil-square";
    if (props.source === "discovered") return "bi bi-broadcast";
    return "bi bi-box-arrow-in-down";
  };
  return <i class={icon()} aria-hidden="true"></i>;
}

function sourceDescription(source: DataSource) {
  if (source === "manual") return "Manual: entered and maintained by you. Imports will not overwrite this field.";
  if (source === "discovered") return "Discovered: observed automatically by LANnventory scanning.";
  return "Imported: received from an external inventory source such as the Proxmox collector.";
}

function ProfileValue(props: { value: string; monospace?: boolean }) {
  return (
    <div class={"small py-1 text-break"+(props.monospace ? " font-monospace" : "")}>
      {props.value || <span class="device-cell-muted">Not set</span>}
    </div>
  );
}

function draftFromProfile(profile: DeviceProfileResponse): ProfileDraft {
  return {
    manufacturer: profile.managed?.manufacturer ?? "",
    model: profile.managed?.model ?? "",
    managementAddress: profile.managed?.managementAddress ?? "",
    managementMode: profile.network?.managementMode ?? "",
    physicalPortCount: profile.network?.physicalPortCount ? String(profile.network.physicalPortCount) : "",
    portCapabilityNotes: profile.network?.portCapabilityNotes ?? "",
    role: profile.system?.role ?? "",
    operatingSystem: profile.system?.operatingSystem ?? "",
    systemVersion: profile.system?.version ?? "",
    hypervisorPlatform: profile.hypervisor?.platform ?? "",
    hypervisorVersion: profile.hypervisor?.version ?? "",
    nodeName: profile.hypervisor?.nodeName ?? "",
    clusterName: profile.hypervisor?.clusterName ?? "",
  };
}

function parsePortCount(value: string): number | null {
  const trimmed = value.trim();
  if (trimmed === "") return 0;
  if (!/^\d+$/.test(trimmed)) return null;
  const parsed = Number(trimmed);
  return Number.isSafeInteger(parsed) && parsed >= 0 && parsed <= 65535 ? parsed : null;
}

function managementModeLabel(value: ProfileDraft["managementMode"]) {
  if (value === "managed") return "Managed";
  if (value === "unmanaged") return "Unmanaged";
  return "";
}

function hypervisorPlatformLabel(value: ProfileDraft["hypervisorPlatform"]) {
  switch (value) {
    case "proxmox-ve": return "Proxmox VE";
    case "vmware-esxi": return "VMware ESXi";
    case "hyper-v": return "Hyper-V";
    case "other": return "Other";
    default: return "";
  }
}

export default DeviceProfileCard;
