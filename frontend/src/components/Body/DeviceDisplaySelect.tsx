import { For } from "solid-js";

import {
  homeDeviceDisplayMode,
  homeDeviceDisplayOptions,
  setHomeDeviceDisplayMode,
} from "../../functions/deviceIdentity";

function DeviceDisplaySelect() {
  const handleChange = (event: Event & { currentTarget: HTMLSelectElement }) => {
    setHomeDeviceDisplayMode(event.currentTarget.value);
  };

  return (
    <select
      class="form-select form-select-sm device-filter-select home-device-display-select"
      value={homeDeviceDisplayMode()}
      title="Device display"
      aria-label="Device display"
      onChange={handleChange}
    >
      <For each={homeDeviceDisplayOptions}>{(option) =>
        <option value={option.key}>{option.label}</option>
      }</For>
    </select>
  );
}

export default DeviceDisplaySelect;
