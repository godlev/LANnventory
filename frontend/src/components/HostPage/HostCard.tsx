import { createEffect, createMemo, createSignal, For, onCleanup, Show } from "solid-js";
import { apiDelHost, apiGetInventoryOptions, apiPatchHost, apiSetHostMetadata, apiWOL } from "../../functions/api";
import { Host } from "../../functions/exports";
import { formatLastSeen } from "../../functions/dateFormat";
import { macAssessmentLabel, macConfidenceLabel, macTypeLabel } from "../../functions/macIdentity";
import { deviceTypeTitle, getDeviceTypeOption, normalizeDeviceType, type DeviceTypeValue } from "../../functions/deviceTypes";
import { updateHostInView } from "../../functions/hostView";
import DeviceTypePicker from "../DeviceTypePicker";
import InventoryAutocomplete from "./InventoryAutocomplete";
import ActionTooltip from "../ActionTooltip";

type HostCardProps = {
  host: Host;
  editMode: boolean;
  onEditModeChange?: (editMode: boolean) => void;
  onHostChange?: (host: Host) => void;
  onDirtyChange?: (dirty: boolean) => void;
};

type HostEditDraft = {
  Name: string;
  DeviceType: DeviceTypeValue;
  Owner: string;
  Location: string;
  Notes: string;
  Tags: string[];
};

type InventoryOptionsState = {
  owners: string[];
  locations: string[];
};

const emptyInventoryOptions: InventoryOptionsState = {
  owners: [],
  locations: [],
};

function HostCard(_props: HostCardProps) {

  const [draft, setDraft] = createSignal<HostEditDraft>(draftFromHost(_props.host));
  const [baseline, setBaseline] = createSignal<HostEditDraft>(draftFromHost(_props.host));
  const [tagInput, setTagInput] = createSignal("");
  const [saving, setSaving] = createSignal(false);
  const [saveStatus, setSaveStatus] = createSignal("");
  const [saveError, setSaveError] = createSignal("");
  const [pinSaving, setPinSaving] = createSignal(false);
  const [pinError, setPinError] = createSignal("");
  const [knownSaving, setKnownSaving] = createSignal(false);
  const [knownError, setKnownError] = createSignal("");
  const [inventoryOptions, setInventoryOptions] = createSignal<InventoryOptionsState>(emptyInventoryOptions);
  let syncedHostID = _props.host.ID;
  let optionsRequestID = 0;

  const syncDraftFromHost = (host: Host) => {
    const nextDraft = draftFromHost(host);
    setBaseline(nextDraft);
    setDraft(nextDraft);
    setTagInput("");
    setSaveStatus("");
    setSaveError("");
  };

  createEffect(() => {
    const host = _props.host;
    if (host.ID !== syncedHostID || !_props.editMode) {
      syncedHostID = host.ID;
      syncDraftFromHost(host);
    }
  });

  createEffect(() => {
    const dirty = _props.editMode && editDirty();
    _props.onDirtyChange?.(dirty);
  });

  createEffect(() => {
    if (!_props.editMode) {
      return;
    }

    const activeRequest = ++optionsRequestID;
    apiGetInventoryOptions()
      .then((options) => {
        if (activeRequest === optionsRequestID) {
          setInventoryOptions({
            owners: options.owners ?? [],
            locations: options.locations ?? [],
          });
        }
      })
      .catch(() => {
        if (activeRequest === optionsRequestID) {
          setInventoryOptions(emptyInventoryOptions);
        }
      });
  });

  onCleanup(() => {
    optionsRequestID++;
    _props.onDirtyChange?.(false);
  });

  const isOnline = () => _props.host.Now === 1;
  const isKnown = () => _props.host.Known === 1;
  const knownTitle = () => isKnown() ? "Known device" : "Unknown device";
  const knownDetail = () => isKnown()
    ? "This device is currently marked Known. Activate to mark it Unknown."
    : "This device is currently marked Unknown. Activate to mark it Known.";
  const knownText = () => isKnown() ? "Known" : "Unknown";
  const statusText = () => isOnline() ? "Online" : "Offline";
  const firstSeenRaw = () => _props.host.FirstSeen ?? "";
  const lastSeenRaw = () => _props.host.LastSeen || _props.host.Date;
  const formattedFirstSeen = () => formatLastSeen(firstSeenRaw());
  const formattedLastSeen = () => formatLastSeen(lastSeenRaw());
  const currentDeviceType = () => getDeviceTypeOption(_props.editMode ? draft().DeviceType : _props.host.DeviceType);
  const hostDeviceTypeTitle = () => deviceTypeTitle(_props.editMode ? draft().DeviceType : _props.host.DeviceType);
  const inventoryName = () => (_props.editMode ? draft().Name : _props.host.Name).trim();
  const editDirty = () => !hostDraftEquals(draft(), baseline());
  const canSave = () => _props.editMode && !saving() && editDirty() && _props.host.ID > 0;
  const pinTitle = () => _props.host.Pinned ? "Remove from Home pins" : "Pin on Home";
  const pinDetail = () => _props.host.Pinned
    ? "This device is pinned at the top of the Home device list. Activate to unpin it."
    : "Pin this device at the top of the Home device list.";
  const ownerOptions = createMemo(() => inventoryOptions().owners);
  const locationOptions = createMemo(() => inventoryOptions().locations);

  const handleNameInput = (name: string) => {
    clearSaveMessages();
    setDraft((current) => ({ ...current, Name: name }));
  };

  const handleKnownToggle = async () => {
    if (_props.host.ID === 0 || knownSaving()) {
      return;
    }

    const previousHost = { ..._props.host };
    const nextKnown = previousHost.Known === 1 ? 0 : 1;
    const optimisticHost = { ...previousHost, Known: nextKnown };

    setKnownSaving(true);
    setKnownError("");
    updateHostInView(optimisticHost);
    _props.onHostChange?.(optimisticHost);

    try {
      const updatedHost = await apiPatchHost(previousHost.ID, { known: nextKnown === 1 });
      updateHostInView(updatedHost);
      _props.onHostChange?.(updatedHost);
    } catch {
      updateHostInView(previousHost);
      _props.onHostChange?.(previousHost);
      setKnownError("Known state could not be saved");
    } finally {
      setKnownSaving(false);
    }
  };

  const handleDeviceTypeChange = async (deviceType: DeviceTypeValue) => {
    clearSaveMessages();
    setDraft((current) => ({ ...current, DeviceType: deviceType }));
  };

  const handleDel = async () => {
    await apiDelHost(_props.host.ID);
    window.location.href = '/';
  };

  const handleWOL = async () => {
    await apiWOL(_props.host.Mac);
  };

  const handleModeToggle = () => {
    if (_props.host.ID === 0) {
      return;
    }
    setSaveStatus("");
    setSaveError("");
    _props.onEditModeChange?.(true);
  };

  const handleMetadataField = (field: keyof Omit<HostEditDraft, "Name" | "DeviceType" | "Tags">, value: string) => {
    clearSaveMessages();
    setDraft((current) => ({ ...current, [field]: value }));
  };

  const handleAddTag = () => {
    const values = tagInput().split(",").map((item) => item.trim()).filter(Boolean);
    if (values.length === 0) {
      setTagInput("");
      return;
    }

    clearSaveMessages();
    setDraft((current) => {
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
    clearSaveMessages();
    setDraft((current) => ({
      ...current,
      Tags: current.Tags.filter((_, currentIndex) => currentIndex !== index),
    }));
  };

  const handleSaveChanges = async () => {
    if (!canSave()) {
      return;
    }

    setSaving(true);
    setSaveStatus("");
    setSaveError("");

    try {
      const currentDraft = draft();
      const updatedHost = await apiPatchHost(_props.host.ID, {
        name: currentDraft.Name,
        deviceType: currentDraft.DeviceType,
        owner: currentDraft.Owner,
        location: currentDraft.Location,
        notes: currentDraft.Notes,
        tags: currentDraft.Tags,
      });
      updateHostInView(updatedHost);
      _props.onHostChange?.(updatedHost);
      setOptionsFromHost(updatedHost);
      syncDraftFromHost(updatedHost);
      _props.onDirtyChange?.(false);
      _props.onEditModeChange?.(false);
      setSaveStatus("Changes saved");
    } catch {
      setSaveError("Changes could not be saved");
    } finally {
      setSaving(false);
    }
  };

  const handleCancelChanges = () => {
    syncDraftFromHost(_props.host);
    _props.onDirtyChange?.(false);
    _props.onEditModeChange?.(false);
  };

  const handlePinToggle = async () => {
    if (_props.host.ID === 0 || pinSaving()) {
      return;
    }

    const previousHost = { ..._props.host };
    const nextPinned = !previousHost.Pinned;
    const optimisticHost = { ...previousHost, Pinned: nextPinned };
    setPinSaving(true);
    setPinError("");
    updateHostInView(optimisticHost);
    _props.onHostChange?.(optimisticHost);

    try {
      const updatedHost = await apiSetHostMetadata(previousHost.ID, { pinned: nextPinned });
      updateHostInView(updatedHost);
      _props.onHostChange?.(updatedHost);
      window.dispatchEvent(new CustomEvent("pinned-changed"));
    } catch {
      updateHostInView(previousHost);
      _props.onHostChange?.(previousHost);
      setPinError("Pin state could not be saved");
    } finally {
      setPinSaving(false);
    }
  };

  const clearSaveMessages = () => {
    setSaveStatus("");
    setSaveError("");
  };

  const setOptionsFromHost = (host: Host) => {
    setInventoryOptions((current) => ({
      owners: mergeOption(current.owners, host.Owner),
      locations: mergeOption(current.locations, host.Location),
    }));
  };

  return (
    <div class="card wyl-panel host-panel">
      <div class="card-header host-panel-header">
        <div>
          <div class="host-panel-title">Host details</div>
          <div class="host-panel-subtitle host-panel-status">
            <span
              class={isOnline() ? "device-status-icon device-status-icon-online" : "device-status-icon device-status-icon-offline"}
              title={statusText()}
              aria-label={statusText()}
              role="img"
            >
              <i class={isOnline() ? "bi bi-check-circle-fill" : "bi bi-x-circle-fill"} aria-hidden="true"></i>
            </span>
            <span>{statusText()}</span>
            <span class="host-status-separator" aria-hidden="true">·</span>
            <span class="host-header-id">ID {_props.host.ID}</span>
          </div>
        </div>

        <div class="host-panel-actions">
          <ActionTooltip
            title="Wake on LAN"
            detail={"Send a Wake-on-LAN magic packet to " + (_props.host.Mac || "this device") + "."}
          >
            <button
              type="button"
              class="btn btn-sm wyl-button host-wol-button host-icon-button"
              title="Wake on LAN"
              aria-label="Wake on LAN"
              onClick={handleWOL}
            >
              <i class="bi bi-power" aria-hidden="true"></i>
            </button>
          </ActionTooltip>

          <ActionTooltip title={knownTitle()} detail={knownDetail()}>
            <button
              type="button"
              class={"btn btn-sm wyl-button host-known-button" + (isKnown() ? " is-known" : " is-unknown")}
              title={knownTitle()}
              aria-label={knownDetail()}
              aria-pressed={isKnown()}
              disabled={_props.host.ID === 0 || knownSaving()}
              onClick={handleKnownToggle}
            >
              <i class={knownSaving() ? "bi bi-hourglass-split" : isKnown() ? "bi bi-bookmark-check-fill" : "bi bi-question-circle-fill"} aria-hidden="true"></i>
              <span>{knownText()}</span>
            </button>
          </ActionTooltip>

          <ActionTooltip title={pinTitle()} detail={pinDetail()}>
            <button
              type="button"
              class={"btn btn-sm wyl-button host-pin-button host-icon-pair-button" + (_props.host.Pinned ? " is-active" : "")}
              title={pinTitle()}
              aria-label={pinDetail()}
              aria-pressed={_props.host.Pinned}
              disabled={_props.host.ID === 0 || pinSaving()}
              onClick={handlePinToggle}
            >
              <Show
                when={!pinSaving()}
                fallback={<i class="bi bi-hourglass-split" aria-hidden="true"></i>}
              >
                <span class="host-pin-icons" aria-hidden="true">
                  <i class={_props.host.Pinned ? "bi bi-pin-angle-fill" : "bi bi-pin-angle"}></i>
                  <i class={_props.host.Pinned ? "bi bi-house-fill" : "bi bi-house"}></i>
                </span>
              </Show>
            </button>
          </ActionTooltip>

          <Show
            when={_props.editMode}
            fallback={
              <ActionTooltip title="Edit" detail="Edit the managed inventory fields for this device.">
                <button
                  type="button"
                  class="btn btn-sm wyl-button host-mode-button"
                  title="Edit host"
                  aria-label="Edit host"
                  disabled={_props.host.ID === 0}
                  onClick={handleModeToggle}
                >
                  <i class="bi bi-pencil-fill" aria-hidden="true"></i>
                  <span>Edit</span>
                </button>
              </ActionTooltip>
            }
          >
            <ActionTooltip title="Save" detail="Save the staged inventory changes for this device.">
              <button
                type="button"
                class="btn btn-sm wyl-button host-save-button"
                title="Save changes"
                aria-label="Save changes"
                disabled={!canSave()}
                onClick={handleSaveChanges}
              >
                <i class={saving() ? "bi bi-hourglass-split" : "bi bi-save"} aria-hidden="true"></i>
                <span>{saving() ? "Saving" : "Save"}</span>
              </button>
            </ActionTooltip>
            <ActionTooltip title="Cancel" detail="Discard staged inventory changes and leave edit mode.">
              <button
                type="button"
                class="btn btn-sm wyl-button"
                title="Cancel changes"
                aria-label="Cancel changes"
                disabled={saving()}
                onClick={handleCancelChanges}
              >
                <i class="bi bi-x-lg" aria-hidden="true"></i>
                <span>Cancel</span>
              </button>
            </ActionTooltip>
          </Show>
        </div>
      </div>

      <div class="card-body host-details-body">
        <div class="host-details-layout">
          <section class="host-detail-section host-detail-section-inventory" aria-labelledby="host-inventory-heading">
            <div class="host-detail-section-header">
              <div class="host-detail-section-heading">
                <i class="bi bi-pencil-square" aria-hidden="true"></i>
                <span id="host-inventory-heading">Inventory</span>
              </div>
              <span class="host-detail-section-badge host-detail-section-badge-editable">Editable</span>
            </div>

            <div class="host-property-grid host-property-grid-section">
              <div class="host-field-label">Name</div>
              <div class="host-field-value">
                <Show
                  when={_props.editMode}
                  fallback={
                    <Show when={inventoryName()} fallback={<span class="device-cell-muted">Not set</span>}>
                      <span>{inventoryName()}</span>
                    </Show>
                  }
                >
                  <input
                    id="host-name-input"
                    type="text"
                    class="form-control form-control-sm wyl-control host-name-input"
                    value={draft().Name}
                    aria-label="Host name"
                    onInput={e => handleNameInput(e.target.value)}
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
                      <i class={"bi " + currentDeviceType().icon} aria-hidden="true"></i>
                      <span>{currentDeviceType().label}</span>
                    </span>
                  }
                >
                  <DeviceTypePicker
                    value={draft().DeviceType}
                    mode="full"
                    class="host-device-type-picker"
                    disabled={_props.host.ID === 0}
                    onChange={handleDeviceTypeChange}
                  ></DeviceTypePicker>
                </Show>
              </div>

              <div class="host-field-label">Owner</div>
              <div class="host-field-value">
                <Show when={_props.editMode} fallback={<MetadataValue value={_props.host.Owner}></MetadataValue>}>
                  <InventoryAutocomplete
                    id="host-owner-input"
                    label="Owner"
                    value={draft().Owner}
                    options={ownerOptions()}
                    onInput={(value) => handleMetadataField("Owner", value)}
                  ></InventoryAutocomplete>
                </Show>
              </div>

              <div class="host-field-label">Location</div>
              <div class="host-field-value">
                <Show when={_props.editMode} fallback={<MetadataValue value={_props.host.Location}></MetadataValue>}>
                  <InventoryAutocomplete
                    id="host-location-input"
                    label="Location"
                    value={draft().Location}
                    options={locationOptions()}
                    onInput={(value) => handleMetadataField("Location", value)}
                  ></InventoryAutocomplete>
                </Show>
              </div>

              <div class="host-field-label">Tags</div>
              <div class="host-field-value">
                <Show when={_props.editMode} fallback={<TagList tags={_props.host.Tags ?? []}></TagList>}>
                  <div class="host-tag-editor">
                    <TagList tags={draft().Tags} editable onRemove={handleRemoveTag}></TagList>
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
                <Show when={_props.editMode} fallback={<MetadataValue value={_props.host.Notes} multiline></MetadataValue>}>
                  <textarea
                    class="form-control form-control-sm wyl-control host-notes-input"
                    value={draft().Notes}
                    aria-label="Notes"
                    rows={4}
                    onInput={(event) => handleMetadataField("Notes", event.currentTarget.value)}
                  ></textarea>
                </Show>
              </div>
            </div>
          </section>

          <section class="host-detail-section host-detail-section-discovery" aria-labelledby="host-discovery-heading">
            <div class="host-detail-section-header">
              <div class="host-detail-section-heading">
                <i class="bi bi-router-fill" aria-hidden="true"></i>
                <span id="host-discovery-heading">Network &amp; Discovery</span>
              </div>
              <span class="host-detail-section-badge">Read only</span>
            </div>

            <div class="host-property-grid host-property-grid-section">
              <div class="host-field-label">IP</div>
              <div class="host-field-value">
                <Show when={isOnline()} fallback={<span class="device-ip-offline">{_props.host.IP}</span>}>
                  <a href={"http://" + _props.host.IP} target="_blank" rel="noreferrer">{_props.host.IP}</a>
                </Show>
              </div>

              <div class="host-field-label">MAC</div>
              <div class="host-field-value">{_props.host.Mac}</div>

              <div class="host-field-label">MAC type</div>
              <div class="host-field-value">
                <span class="host-lifecycle-value">
                  <span>{macTypeLabel(_props.host.MacType)}</span>
                  <Show when={_props.host.MacType === "locally-administered"}>
                    <ActionTooltip
                      title="Locally administered MAC"
                      detail="This MAC may be randomized, virtual, or manually assigned."
                    >
                      <span
                        class="host-lifecycle-approx"
                        title="Locally administered MAC"
                        aria-label="This MAC may be randomized, virtual, or manually assigned."
                        role="img"
                        tabIndex={0}
                      >
                        <i class="bi bi-info-circle" aria-hidden="true"></i>
                      </span>
                    </ActionTooltip>
                  </Show>
                </span>
              </div>

              <Show when={_props.host.MacType === "locally-administered"}>
                <div class="host-field-label">MAC assessment</div>
                <div class="host-field-value">
                  <span class="host-lifecycle-value">
                    <span>{macAssessmentLabel(_props.host.MacAssessment)}</span>
                    <Show when={macConfidenceLabel(_props.host.MacAssessmentConfidence)}>
                      <span class="badge rounded-pill text-bg-secondary">
                        {macConfidenceLabel(_props.host.MacAssessmentConfidence)}
                      </span>
                    </Show>
                    <ActionTooltip
                      title="MAC assessment"
                      detail={_props.host.MacAssessmentReason || "A locally administered MAC does not by itself prove privacy randomization."}
                    >
                      <span
                        class="host-lifecycle-approx"
                        title="MAC assessment"
                        aria-label={_props.host.MacAssessmentReason || "MAC assessment details"}
                        role="img"
                        tabIndex={0}
                      >
                        <i class="bi bi-info-circle" aria-hidden="true"></i>
                      </span>
                    </ActionTooltip>
                  </span>
                </div>
              </Show>

              <div class="host-field-label">Interface</div>
              <div class="host-field-value">{_props.host.Iface || <span class="device-cell-muted">Unknown</span>}</div>

              <div class="host-field-label">Hardware</div>
              <div class="host-field-value">{_props.host.Hw || <span class="device-cell-muted">Unknown</span>}</div>

              <div class="host-field-label">DNS name</div>
              <div class="host-field-value">{_props.host.DNS || <span class="device-cell-muted">Unknown</span>}</div>

              <div class="host-field-label">First seen</div>
              <div class="host-field-value" title={firstSeenRaw()}>
                <Show when={firstSeenRaw()} fallback={<span class="device-cell-muted">Not seen yet</span>}>
                  <span class="host-lifecycle-value">
                    <span>{formattedFirstSeen()}</span>
                    <Show when={_props.host.FirstSeenEstimated}>
                      <ActionTooltip
                        title="Approximate first seen"
                        detail="Lifecycle tracking started after this device was already known. This time is the earliest retained evidence LANnventory could find."
                      >
                        <span
                          class="host-lifecycle-approx"
                          title="Approximate first seen"
                          aria-label="Approximate first seen"
                          role="img"
                          tabIndex={0}
                        >
                          <i class="bi bi-info-circle" aria-hidden="true"></i>
                        </span>
                      </ActionTooltip>
                    </Show>
                  </span>
                </Show>
              </div>

              <div class="host-field-label">Last seen</div>
              <div class="host-field-value" title={lastSeenRaw()}>
                <Show when={lastSeenRaw()} fallback={<span class="device-cell-muted">Not seen yet</span>}>
                  {formattedLastSeen()}
                </Show>
              </div>
            </div>
          </section>
        </div>

        <Show when={(_props.editMode && editDirty() && !saving() && !saveError()) || saveStatus() || saveError()}>
          <div class="host-edit-feedback">
            <Show when={_props.editMode && editDirty() && !saving() && !saveError()}>
              <span class="config-save-status">Unsaved changes</span>
            </Show>
            <Show when={saveStatus()}>
              <span class="config-save-status">{saveStatus()}</span>
            </Show>
            <Show when={saveError()}>
              <span class="config-save-error" role="alert">{saveError()}</span>
            </Show>
          </div>
        </Show>

        <Show when={pinError() || knownError()}>
          <div class="host-inline-error" role="alert">{pinError() || knownError()}</div>
        </Show>

        <Show when={_props.editMode}>
          <div class="host-actions">
            <ActionTooltip title="Delete" detail="Remove this device from LANnventory.">
              <button
                type="button"
                onClick={handleDel}
                class="btn btn-sm wyl-button device-delete-button host-delete-button"
                title="Delete"
                aria-label="Delete device"
              >
                <i class="bi bi-trash-fill" aria-hidden="true"></i>
                <span>Delete</span>
              </button>
            </ActionTooltip>
          </div>
        </Show>
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

function draftFromHost(host: Host): HostEditDraft {
  return {
    Name: host.Name ?? "",
    DeviceType: normalizeDeviceType(host.DeviceType),
    Owner: host.Owner ?? "",
    Location: host.Location ?? "",
    Notes: host.Notes ?? "",
    Tags: [...(host.Tags ?? [])],
  };
}

function hostDraftEquals(left: HostEditDraft, right: HostEditDraft) {
  return left.Name === right.Name
    && left.DeviceType === right.DeviceType
    && left.Owner === right.Owner
    && left.Location === right.Location
    && left.Notes === right.Notes
    && left.Tags.length === right.Tags.length
    && left.Tags.every((tag, index) => tag === right.Tags[index]);
}

function mergeOption(options: string[], value: string | undefined) {
  const trimmed = (value ?? "").trim();
  if (trimmed === "") {
    return options;
  }
  if (options.some((option) => option.toLowerCase() === trimmed.toLowerCase())) {
    return options;
  }

  return [...options, trimmed].sort((left, right) => {
    const normalizedLeft = left.toLowerCase();
    const normalizedRight = right.toLowerCase();
    return normalizedLeft === normalizedRight
      ? left.localeCompare(right)
      : normalizedLeft.localeCompare(normalizedRight);
  });
}

export default HostCard
