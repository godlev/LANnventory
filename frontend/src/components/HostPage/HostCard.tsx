import { createEffect, createSignal, Show } from "solid-js";
import { apiDelHost, apiEditHost, apiSetDeviceType, apiWOL } from "../../functions/api";
import { Host } from "../../functions/exports";
import { formatLastSeen } from "../../functions/dateFormat";
import { deviceDisplayName } from "../../functions/deviceIdentity";
import { getDeviceTypeOption, type DeviceTypeValue } from "../../functions/deviceTypes";
import { updateHostInView } from "../../functions/hostView";
import DeviceTypePicker from "../DeviceTypePicker";

import { debounce } from "@solid-primitives/scheduled";

type HostCardProps = {
  host: Host;
  editMode: boolean;
  onEditModeChange?: (editMode: boolean) => void;
  onHostChange?: (host: Host) => void;
};

function HostCard(_props: HostCardProps) {

  const [name, setName] = createSignal(_props.host.Name);

  createEffect(() => {
    setName(_props.host.Name);
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
  const deviceTypeTitle = () => deviceType().value === "" ? "Device type not set" : "Device type: " + deviceType().label;
  const displayName = () => deviceDisplayName({ ..._props.host, Name: name() });
  const modeTitle = () => _props.editMode ? "Done editing host" : "Edit host";

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
                  title={deviceTypeTitle()}
                  aria-label={deviceTypeTitle()}
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

export default HostCard
