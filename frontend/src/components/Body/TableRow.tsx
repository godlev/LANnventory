import { createSignal, onCleanup, Show } from "solid-js";
import { editNames, hasMultipleIfaces, selectedIDs, setSelectedIDs } from "../../functions/exports";
import { apiEditHost, apiSetDeviceType } from "../../functions/api";
import { formatLastSeen } from "../../functions/dateFormat";
import { deviceDisplayName } from "../../functions/deviceIdentity";
import { isUnknownHardware } from "../../functions/hardware";
import { updateHostInView } from "../../functions/hostView";
import { getDeviceTypeOption, type DeviceTypeValue } from "../../functions/deviceTypes";
import DeviceTypePicker from "../DeviceTypePicker";

import { debounce } from "@solid-primitives/scheduled";

function TableRow(_props: any) {

  const [name, setName] = createSignal(_props.host.Name);

  const isOnline = () => _props.host.Now === 1;
  const known = () => _props.host.Known === 1;
  const lastSeen = () => formatLastSeen(_props.host.Date);
  const rowClass = () => [
    _props.host.Known === 0 ? "device-row-unknown" : "",
    !isOnline() ? "device-row-offline" : "",
  ].filter(Boolean).join(" ");
  const nameClass = () => isOnline() ? "" : "device-offline-name";
  const knownTitle = () => known()
    ? "Known device - click to mark unknown"
    : "Unknown device - click to mark known";
  const statusText = () => isOnline() ? "Online" : "Offline";
  const displayName = () => deviceDisplayName({ ..._props.host, Name: name() });
  const mobileIdentity = () => {
    const currentName = displayName();
    return _props.host.IP && currentName !== _props.host.IP
      ? currentName + " · " + _props.host.IP
      : currentName;
  };
  const hardwareText = () => (_props.host.Hw ?? "").trim() || "Unknown";
  const hardwareClass = () => "device-hardware-text" + (isUnknownHardware(_props.host.Hw) ? " device-hardware-text-muted" : "");
  const deviceTypeOption = () => getDeviceTypeOption(_props.host.DeviceType);
  const mobileToggleLabel = () => (_props.mobileExpanded ? "Hide" : "Show") + " device details for " + displayName();

  let nameSaveQueue: Promise<unknown> = Promise.resolve();
  let hasPendingNameChange = false;

  const queueNameSave = (val: string) => {
    nameSaveQueue = nameSaveQueue
      .catch(() => undefined)
      .then(() => apiEditHost(_props.host.ID, val, ""));

    return nameSaveQueue;
  };

  const debouncedApi = debounce((val: string) => {
    void queueNameSave(val);
  }, 300);

  const syncNameToView = () => {
    const currentName = name();

    debouncedApi.clear();
    if (hasPendingNameChange) {
      hasPendingNameChange = false;
      void queueNameSave(currentName);
    }
    updateHostInView({ ID: _props.host.ID, Name: currentName });
  };

  onCleanup(() => {
    debouncedApi.clear();
    if (hasPendingNameChange) {
      void queueNameSave(name());
    }
  });

  const handleInput = (n: string) => {
    setName(n);
    hasPendingNameChange = true;
    debouncedApi(n);
  };

  const handleNameKeyDown = (event: KeyboardEvent & { currentTarget: HTMLInputElement }) => {
    if (event.key === "Enter") {
      event.currentTarget.blur();
    }
  };
  const handleToggle = async () => {
    await apiEditHost(_props.host.ID, name(), "toggle");
    updateHostInView({ ID: _props.host.ID, Name: name(), Known: known() ? 0 : 1 });
  };

  const handleDeviceTypeChange = async (deviceType: DeviceTypeValue) => {
    const updatedHost = await apiSetDeviceType(_props.host.ID, deviceType);
    updateHostInView(updatedHost);
  };

  const handleCheck = (checked: boolean) => {
    const id = _props.host.ID;
    setSelectedIDs(prev => {
      if (checked) {
        return prev.includes(id) ? prev : [...prev, id];
      } else {
        return prev.filter(item => item !== id);
      }
    });
  };

  return (
    <tr class={rowClass()}>
      <td class="device-table-mobile-cell">
        <div class="device-mobile-row">
          <span class="device-mobile-known">
            <button
              type="button"
              class={known() ? "device-known-toggle device-known-toggle-known" : "device-known-toggle device-known-toggle-unknown"}
              title={knownTitle()}
              aria-label={knownTitle()}
              aria-pressed={known()}
              onClick={handleToggle}
            >
              <i class={known() ? "bi bi-bookmark-check-fill" : "bi bi-question-circle-fill"} aria-hidden="true"></i>
            </button>
          </span>
          <span class="device-mobile-type">
            <DeviceTypePicker
              value={_props.host.DeviceType}
              mode="icon"
              onChange={handleDeviceTypeChange}
            ></DeviceTypePicker>
          </span>
          <span class="device-mobile-identity">
            <Show
              when={editNames()}
              fallback={<a href={"/host/" + _props.host.ID} class={"device-mobile-name-link " + nameClass()}>{mobileIdentity()}</a>}
            >
              <input
                type="text"
                class="form-control form-control-sm device-mobile-name-input"
                value={name()}
                onInput={e => handleInput(e.target.value)}
                onBlur={syncNameToView}
                onKeyDown={handleNameKeyDown}
              ></input>
            </Show>
          </span>
          <span
            class={isOnline() ? "device-status-icon device-status-icon-online" : "device-status-icon device-status-icon-offline"}
            title={statusText()}
            aria-label={statusText()}
            role="img"
          >
            <i class={isOnline() ? "bi bi-check-circle-fill" : "bi bi-x-circle-fill"} aria-hidden="true"></i>
          </span>
          <button
            type="button"
            class="device-mobile-expand"
            aria-expanded={_props.mobileExpanded ? "true" : "false"}
            aria-label={mobileToggleLabel()}
            title={mobileToggleLabel()}
            onClick={_props.onToggleMobileExpanded}
          >
            <i class={"bi " + (_props.mobileExpanded ? "bi-chevron-up" : "bi-chevron-down")} aria-hidden="true"></i>
          </button>
        </div>
        <Show when={_props.mobileExpanded}>
          <div class="device-mobile-details">
            <span class="device-mobile-detail-label">Name</span>
            <span class="device-mobile-detail-value">{displayName()}</span>
            <span class="device-mobile-detail-label">Type</span>
            <span class="device-mobile-detail-value">{deviceTypeOption().label}</span>
            <span class="device-mobile-detail-label">Status</span>
            <span class="device-mobile-detail-value">{statusText()}</span>
            <span class="device-mobile-detail-label">IP</span>
            <span class="device-mobile-detail-value">{_props.host.IP}</span>
            <span class="device-mobile-detail-label">MAC</span>
            <span class="device-mobile-detail-value">{_props.host.Mac}</span>
            <span class="device-mobile-detail-label">Hardware</span>
            <span class="device-mobile-detail-value">{hardwareText()}</span>
            <Show when={(_props.host.Iface ?? "").trim()}>
              <span class="device-mobile-detail-label">Interface</span>
              <span class="device-mobile-detail-value">{_props.host.Iface}</span>
            </Show>
            <span class="device-mobile-detail-label">Last Seen</span>
            <span class="device-mobile-detail-value">{lastSeen()}</span>
            <span class="device-mobile-detail-label">Known</span>
            <span class="device-mobile-detail-value">{known() ? "Yes" : "No"}</span>
          </div>
        </Show>
      </td>
      <td class="device-table-index opacity-50">{_props.index}.</td>
      <td class="device-table-known">
        <button
          type="button"
          class={known() ? "device-known-toggle device-known-toggle-known" : "device-known-toggle device-known-toggle-unknown"}
          title={knownTitle()}
          aria-label={knownTitle()}
          aria-pressed={known()}
          onClick={handleToggle}
        >
          <i class={known() ? "bi bi-bookmark-check-fill" : "bi bi-question-circle-fill"} aria-hidden="true"></i>
        </button>
      </td>
      <td class="device-table-actions">
        <Show
          when={editNames()}
          fallback={
          <a href={"/host/" + _props.host.ID + "?edit=1"} class="device-action-link" title="Edit host">
            <i class="bi bi-pencil-fill my-btn p-2" aria-hidden="true"></i>
          </a>}
        >
          <input
            type="checkbox"
            class="form-check-input device-action-checkbox"
            checked={selectedIDs().includes(_props.host.ID)}
            onChange={e => handleCheck((e.target as HTMLInputElement).checked)}
          />
        </Show>
      </td>
      <td class="device-table-name">
        <Show
          when={editNames()}
          fallback={<a href={"/host/" + _props.host.ID} class={"device-name-link " + nameClass()}>{displayName()}</a>}
        >
          <input type="text" class="form-control" value={name()}
            onInput={e => handleInput(e.target.value)}
            onBlur={syncNameToView}
            onKeyDown={handleNameKeyDown}></input>
        </Show>
      </td>
      <td class="device-table-type">
        <DeviceTypePicker
          value={_props.host.DeviceType}
          mode="icon"
          onChange={handleDeviceTypeChange}
        ></DeviceTypePicker>
      </td>
      <td class="device-table-ip">
        <span class="device-ip-with-status">
          <span
            class={isOnline() ? "device-status-icon device-status-icon-online" : "device-status-icon device-status-icon-offline"}
            title={statusText()}
            aria-label={statusText()}
            role="img"
          >
            <i class={isOnline() ? "bi bi-check-circle-fill" : "bi bi-x-circle-fill"} aria-hidden="true"></i>
          </span>
          <Show when={isOnline()} fallback={<span class="device-ip-offline">{_props.host.IP}</span>}>
            <a href={"http://" + _props.host.IP} target="_blank" rel="noreferrer">{_props.host.IP}</a>
          </Show>
        </span>
      </td>
      <Show when={hasMultipleIfaces()}>
        <td class="device-table-iface"><span class="device-cell-muted">{_props.host.Iface}</span></td>
      </Show>
      <td class="device-table-mac"><span class="device-cell-muted">{_props.host.Mac}</span></td>
      <td class="device-table-hardware" title={_props.host.Hw}>
        <span class={hardwareClass()}>{hardwareText()}</span>
      </td>
      <td class="device-table-last-seen" title={_props.host.Date}>
        <span class="device-cell-muted">{lastSeen()}</span>
      </td>
    </tr>
  )
}

export default TableRow
