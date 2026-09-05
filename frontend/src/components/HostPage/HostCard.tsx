import { createEffect, createSignal, For, Show } from "solid-js";
import { apiDelHost, apiEditHost, apiSetDeviceType, apiSetHostMetadata, apiWOL } from "../../functions/api";
import { Host } from "../../functions/exports";
import { formatLastSeen } from "../../functions/dateFormat";
import { deviceDisplayName } from "../../functions/deviceIdentity";
import { deviceTypeTitle, getDeviceTypeOption, type DeviceTypeValue } from "../../functions/deviceTypes";
import { updateHostInView } from "../../functions/hostView";
import DeviceTypePicker from "../DeviceTypePicker";

import { debounce } from "@solid-primitives/scheduled";

type HostCardProps = {
  host: Host;
  editMode: boolean;
  onEditModeChange?: (editMode: boolean) => void;
  onHostChange?: (host: Host) => void;
};

type MetadataDraft = {
  Owner: string;
  Location: string;
  Notes: string;
  Tags: string[];
  Pinned: boolean;
};

function HostCard(_props: HostCardProps) {

  const [name, setName] = createSignal(_props.host.Name);
  const [metadataDraft, setMetadataDraft] = createSignal<MetadataDraft>(metadataFromHost(_props.host));
  const [metadataBaseline, setMetadataBaseline] = createSignal<MetadataDraft>(metadataFromHost(_props.host));
  const [tagInput, setTagInput] = createSignal("");
  const [metadataSaving, setMetadataSaving] = createSignal(false);
  const [metadataStatus, setMetadataStatus] = createSignal("");
  const [metadataError, setMetadataError] = createSignal("");
  let syncedMetadataHostID = _props.host.ID;

  createEffect(() => {
    setName(_props.host.Name);
  });

  createEffect(() => {
    const hostID = _props.host.ID;
    if (hostID === syncedMetadataHostID) {
      return;
    }

    syncedMetadataHostID = hostID;
    const nextMetadata = metadataFromHost(_props.host);
    setMetadataBaseline(nextMetadata);
    setMetadataDraft(nextMetadata);
    setTagInput("");
    setMetadataStatus("");
    setMetadataError("");
  });

  const isOnline = () => _props.host.Now === 1;
  const isKnown = () => _props.host.Known === 1;
  const knownTitle = () => isKnown()
    ? "Known device - click to mark unknown"
    : "Unknown device - click to mark known";
  const knownText = () => isKnown() ? "Known device" : "Unknown device";
  const statusText = () => isOnline() ? "Online" : "Offline";
  const formattedLastSeen = () => formatLastSeen(_props.host.Date);
  const deviceType = () => getDeviceTypeOption(_props.host.DeviceType);
  const hostDeviceTypeTitle = () => deviceTypeTitle(_props.host.DeviceType);
  const displayName = () => deviceDisplayName({ ..._props.host, Name: name() });
  const modeTitle = () => _props.editMode ? "Done editing host" : "Edit host";
  const metadataDirty = () => !metadataDraftEquals(metadataDraft(), metadataBaseline());

  const debouncedApi = debounce(async (val: string) => {
      await apiEditHost(_props.host.ID, val, "");
    }, 300);

  const handleInput = async (n: string) => {
    const updatedHost = { ..._props.host, Name: n };
    setName(n);
    updateHostInView(updatedHost);
    _props.onHostChange?.(updatedHost);
    debouncedApi(n);
  };

  const handleToggle = async () => {
    const nextName = name();
    const updatedHost = { ..._props.host, Name: nextName, Known: isKnown() ? 0 : 1 };

    await apiEditHost(_props.host.ID, nextName, 'toggle');
    updateHostInView(updatedHost);
    _props.onHostChange?.(updatedHost);
  };

  const handleDeviceTypeChange = async (deviceType: DeviceTypeValue) => {
    const updatedHost = await apiSetDeviceType(_props.host.ID, deviceType);
    updateHostInView(updatedHost);
    _props.onHostChange?.(updatedHost);
  };

  const handleDel = async () => {
    
    await apiDelHost(_props.host.ID);
    window.location.href = '/';
  };

  const handleWOL = async () => {
    
    await apiWOL(_props.host.Mac);
  };

  const handleModeToggle = () => {
    _props.onEditModeChange?.(!_props.editMode);
  };

  const handleMetadataField = (field: keyof Omit<MetadataDraft, "Tags" | "Pinned">, value: string) => {
    setMetadataStatus("");
    setMetadataError("");
    setMetadataDraft((current) => ({ ...current, [field]: value }));
  };

  const handlePinnedDraft = (checked: boolean) => {
    setMetadataStatus("");
    setMetadataError("");
    setMetadataDraft((current) => ({ ...current, Pinned: checked }));
  };

  const handleAddTag = () => {
    const values = tagInput().split(",").map((item) => item.trim()).filter(Boolean);
    if (values.length === 0) {
      setTagInput("");
      return;
    }

    setMetadataStatus("");
    setMetadataError("");
    setMetadataDraft((current) => {
      const seen = new Set(current.Tags.map((tag) => tag.toLowerCase()));
      const nextTags = [...current.Tags];

      for (const value of values) {
        const key = value.toLowerCase();
        if (seen.has(key)) {
          continue;
        }
        seen.add(key);
        nextTags.push(value);
      }

      return { ...current, Tags: nextTags };
    });
    setTagInput("");
  };

  const handleTagKeyDown = (event: KeyboardEvent & { currentTarget: HTMLInputElement }) => {
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      handleAddTag();
    }
  };

  const handleRemoveTag = (index: number) => {
    setMetadataStatus("");
    setMetadataError("");
    setMetadataDraft((current) => ({
      ...current,
      Tags: current.Tags.filter((_, currentIndex) => currentIndex !== index),
    }));
  };

  const handleSaveMetadata = async () => {
    if (metadataSaving() || !metadataDirty()) {
      return;
    }

    setMetadataSaving(true);
    setMetadataStatus("");
    setMetadataError("");

    try {
      const draft = metadataDraft();
      const updatedHost = await apiSetHostMetadata(_props.host.ID, {
        owner: draft.Owner,
        location: draft.Location,
        notes: draft.Notes,
        tags: draft.Tags,
        pinned: draft.Pinned,
      });
      const nextMetadata = metadataFromHost(updatedHost);
      setMetadataBaseline(nextMetadata);
      setMetadataDraft(nextMetadata);
      setTagInput("");
      setMetadataError("");
      setMetadataStatus("Metadata saved");
      updateHostInView(updatedHost);
      _props.onHostChange?.(updatedHost);
    } catch {
      setMetadataError("Metadata could not be saved");
    } finally {
      setMetadataSaving(false);
    }
  };

  return (
    <div class="card wyl-panel host-panel">
      <div class="card-header host-panel-header">
        <div>
          <div class="host-panel-title">Host details</div>
          <div class="host-panel-subtitle">{statusText()}</div>
        </div>
        <button
          type="button"
          class="btn btn-sm wyl-button host-mode-button"
          title={modeTitle()}
          aria-label={modeTitle()}
          disabled={_props.host.ID === 0}
          onClick={handleModeToggle}
        >
          <i class={_props.editMode ? "bi bi-check-lg" : "bi bi-pencil-fill"} aria-hidden="true"></i>
          <span>{_props.editMode ? "Done" : "Edit"}</span>
        </button>
      </div>
      <div class="card-body host-details-body">
        <div class="host-property-grid">
          <div class="host-field-label">ID</div>
          <div class="host-field-value">{_props.host.ID}</div>

          <div class="host-field-label">Name</div>
          <div class="host-field-value">
            <Show
              when={_props.editMode}
              fallback={<span>{displayName()}</span>}
            >
              <input
                id="host-name-input"
                type="text"
                class="form-control form-control-sm wyl-control host-name-input"
                value={name()}
                aria-label="Host name"
                onInput={e => handleInput(e.target.value)}
              ></input>
            </Show>
          </div>

          <div class="host-field-label">Device type</div>
          <div class="host-field-value">
            <Show
              when={_props.editMode}
              fallback={
                <span
                  class="host-static-value host-static-device-type"
                  title={hostDeviceTypeTitle()}
                  aria-label={hostDeviceTypeTitle()}
                  role="img"
                >
                  <i class={"bi " + deviceType().icon} aria-hidden="true"></i>
                  <span>{deviceType().label}</span>
                </span>
              }
            >
              <DeviceTypePicker
                value={_props.host.DeviceType}
                mode="full"
                class="host-device-type-picker"
                disabled={_props.host.ID === 0}
                onChange={handleDeviceTypeChange}
              ></DeviceTypePicker>
            </Show>
          </div>

          <div class="host-field-label">Pinned</div>
          <div class="host-field-value">
            <Show
              when={_props.editMode}
              fallback={
                <span class={_props.host.Pinned ? "host-static-value host-pinned-value" : "device-cell-muted"}>
                  <i class={_props.host.Pinned ? "bi bi-pin-angle-fill" : "bi bi-pin-angle"} aria-hidden="true"></i>
                  <span>{_props.host.Pinned ? "Pinned" : "Not pinned"}</span>
                </span>
              }
            >
              <label class="host-metadata-check">
                <input
                  type="checkbox"
                  class="form-check-input"
                  checked={metadataDraft().Pinned}
                  onChange={(event) => handlePinnedDraft(event.currentTarget.checked)}
                ></input>
                <span>Pinned</span>
              </label>
            </Show>
          </div>

          <div class="host-field-label">Owner</div>
          <div class="host-field-value">
            <Show
              when={_props.editMode}
              fallback={<MetadataValue value={_props.host.Owner}></MetadataValue>}
            >
              <input
                type="text"
                class="form-control form-control-sm wyl-control"
                value={metadataDraft().Owner}
                aria-label="Owner"
                onInput={(event) => handleMetadataField("Owner", event.currentTarget.value)}
              ></input>
            </Show>
          </div>

          <div class="host-field-label">Location</div>
          <div class="host-field-value">
            <Show
              when={_props.editMode}
              fallback={<MetadataValue value={_props.host.Location}></MetadataValue>}
            >
              <input
                type="text"
                class="form-control form-control-sm wyl-control"
                value={metadataDraft().Location}
                aria-label="Location"
                onInput={(event) => handleMetadataField("Location", event.currentTarget.value)}
              ></input>
            </Show>
          </div>

          <div class="host-field-label">Tags</div>
          <div class="host-field-value">
            <Show
              when={_props.editMode}
              fallback={<TagList tags={_props.host.Tags ?? []}></TagList>}
            >
              <div class="host-tag-editor">
                <TagList tags={metadataDraft().Tags} editable onRemove={handleRemoveTag}></TagList>
                <div class="host-tag-input-row">
                  <input
                    type="text"
                    class="form-control form-control-sm wyl-control"
                    value={tagInput()}
                    aria-label="Add tag"
                    placeholder="Add tag"
                    onInput={(event) => setTagInput(event.currentTarget.value)}
                    onKeyDown={handleTagKeyDown}
                  ></input>
                  <button
                    type="button"
                    class="btn btn-sm wyl-button"
                    disabled={tagInput().trim() === ""}
                    onClick={handleAddTag}
                  >
                    Add
                  </button>
                </div>
              </div>
            </Show>
          </div>

          <div class="host-field-label">Notes</div>
          <div class="host-field-value">
            <Show
              when={_props.editMode}
              fallback={<MetadataValue value={_props.host.Notes} multiline></MetadataValue>}
            >
              <textarea
                class="form-control form-control-sm wyl-control host-notes-input"
                value={metadataDraft().Notes}
                aria-label="Notes"
                rows={4}
                onInput={(event) => handleMetadataField("Notes", event.currentTarget.value)}
              ></textarea>
            </Show>
          </div>

          <div class="host-field-label">DNS name</div>
          <div class="host-field-value">{_props.host.DNS || <span class="device-cell-muted">Unknown</span>}</div>

          <div class="host-field-label">Iface</div>
          <div class="host-field-value">{_props.host.Iface || <span class="device-cell-muted">Unknown</span>}</div>

          <div class="host-field-label">IP</div>
          <div class="host-field-value">
            <Show when={isOnline()} fallback={<span class="device-ip-offline">{_props.host.IP}</span>}>
              <a href={"http://" + _props.host.IP} target="_blank" rel="noreferrer">{_props.host.IP}</a>
            </Show>
          </div>

          <div class="host-field-label">MAC</div>
          <div class="host-field-value">{_props.host.Mac}</div>

          <div class="host-field-label">Hardware</div>
          <div class="host-field-value">{_props.host.Hw || <span class="device-cell-muted">Unknown</span>}</div>

          <div class="host-field-label">Last seen</div>
          <div class="host-field-value" title={_props.host.Date}>{formattedLastSeen()}</div>

          <div class="host-field-label">Known</div>
          <div class="host-field-value">
            <button
              type="button"
              class={isKnown() ? "device-known-toggle device-known-toggle-known host-known-toggle" : "device-known-toggle device-known-toggle-unknown host-known-toggle"}
              title={knownTitle()}
              aria-label={knownTitle()}
              aria-pressed={isKnown()}
              disabled={_props.host.ID === 0}
              onClick={handleToggle}
            >
              <i class={isKnown() ? "bi bi-bookmark-check-fill" : "bi bi-question-circle-fill"} aria-hidden="true"></i>
              <span class="host-known-toggle-label">{knownText()}</span>
            </button>
          </div>

          <div class="host-field-label">Status</div>
          <div class="host-field-value host-status-value">
            <span
              class={isOnline() ? "device-status-icon device-status-icon-online" : "device-status-icon device-status-icon-offline"}
              title={statusText()}
              aria-label={statusText()}
              role="img"
            >
              <i class={isOnline() ? "bi bi-check-circle-fill" : "bi bi-x-circle-fill"} aria-hidden="true"></i>
            </span>
            <span>{statusText()}</span>
          </div>
        </div>
        <Show when={_props.editMode}>
          <div class="host-metadata-actions">
            <button
              type="button"
              class="btn btn-sm wyl-button"
              disabled={metadataSaving() || !metadataDirty() || _props.host.ID === 0}
              onClick={handleSaveMetadata}
            >
              <i class={metadataSaving() ? "bi bi-hourglass-split" : "bi bi-save"} aria-hidden="true"></i>
              <span>{metadataSaving() ? "Saving" : "Save metadata"}</span>
            </button>
            <Show when={metadataDirty() && !metadataSaving() && !metadataError()}>
              <span class="config-save-status">Unsaved metadata changes</span>
            </Show>
            <Show when={metadataStatus()}>
              <span class="config-save-status">{metadataStatus()}</span>
            </Show>
            <Show when={metadataError()}>
              <span class="config-save-error" role="alert">{metadataError()}</span>
            </Show>
          </div>
        </Show>
        <div class="host-actions">
          <button type="button" onClick={handleWOL} class="btn btn-sm wyl-button host-wol-button">
            <i class="bi bi-power" aria-hidden="true"></i>
            <span>Wake-on-LAN</span>
          </button>
          <Show when={_props.editMode}>
            <button type="button" onClick={handleDel} class="btn btn-sm wyl-button device-delete-button host-delete-button">
              <i class="bi bi-trash-fill" aria-hidden="true"></i>
              <span>Delete host</span>
            </button>
          </Show>
        </div>
      </div>
    </div>
  )
}

function MetadataValue(props: { value?: string; multiline?: boolean }) {
  const value = () => (props.value ?? "").trim();

  return (
    <Show when={value()} fallback={<span class="device-cell-muted">Not set</span>}>
      <span class={props.multiline ? "host-metadata-notes" : ""}>{props.value}</span>
    </Show>
  );
}

function TagList(props: { tags: string[]; editable?: boolean; onRemove?: (index: number) => void }) {
  return (
    <Show when={props.tags.length > 0} fallback={<span class="device-cell-muted">Not set</span>}>
      <span class="host-tag-list">
        <For each={props.tags}>{(tag, index) =>
          <span class="host-tag-chip">
            <span>{tag}</span>
            <Show when={props.editable}>
              <button
                type="button"
                class="host-tag-remove"
                title={"Remove tag " + tag}
                aria-label={"Remove tag " + tag}
                onClick={() => props.onRemove?.(index())}
              >
                <i class="bi bi-x" aria-hidden="true"></i>
              </button>
            </Show>
          </span>
        }</For>
      </span>
    </Show>
  );
}

function metadataFromHost(host: Host): MetadataDraft {
  return {
    Owner: host.Owner ?? "",
    Location: host.Location ?? "",
    Notes: host.Notes ?? "",
    Tags: [...(host.Tags ?? [])],
    Pinned: host.Pinned === true,
  };
}

function metadataDraftEquals(left: MetadataDraft, right: MetadataDraft) {
  return left.Owner === right.Owner
    && left.Location === right.Location
    && left.Notes === right.Notes
    && left.Pinned === right.Pinned
    && left.Tags.length === right.Tags.length
    && left.Tags.every((tag, index) => tag === right.Tags[index]);
}

export default HostCard
