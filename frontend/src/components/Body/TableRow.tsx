import { createSignal, Show } from "solid-js";
import { editNames, selectedIDs, setSelectedIDs } from "../../functions/exports";
import { apiEditHost } from "../../functions/api";

import { debounce } from "@solid-primitives/scheduled"; 

function TableRow(_props: any) {

  const [name, setName] = createSignal(_props.host.Name);

  const isOnline = () => _props.host.Now === 1;
  const known = () => _props.host.Known === 1;
  const lastSeen = () => formatLastSeen(_props.host.Date);
  const knownTitle = () => known()
    ? "Known device - click to mark unknown"
    : "Unknown device - click to mark known";
  const statusText = () => isOnline() ? "Online" : "Offline";

  const debouncedApi = debounce(async (val: string) => {
    await apiEditHost(_props.host.ID, val, "");
  }, 300);

  const handleInput = async (n: string) => {
    setName(n);
    debouncedApi(n);
  };
  const handleToggle = async () => {
    await apiEditHost(_props.host.ID, name(), "toggle");
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
    <tr>
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
          <a href={"/host/" + _props.host.ID} class="device-action-link" title="More">
            <i class="bi bi-three-dots-vertical my-btn p-2" aria-hidden="true"></i>
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
          fallback={name()}
        >
          <input type="text" class="form-control" value={name()}
            onInput={e => handleInput(e.target.value)}></input>
        </Show>
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
          <a href={"http://" + _props.host.IP} target="_blank">{_props.host.IP}</a>
        </span>
      </td>
      <td class="device-table-iface"><span class="device-cell-muted">{_props.host.Iface}</span></td>
      <td class="device-table-mac"><span class="device-cell-muted">{_props.host.Mac}</span></td>
      <td class="device-table-hardware" title={_props.host.Hw}>
        <span class="device-hardware-text">{_props.host.Hw}</span>
      </td>
      <td class="device-table-last-seen" title={_props.host.Date}>
        <span class="device-cell-muted">{lastSeen()}</span>
      </td>
    </tr>
  )
}

function formatLastSeen(date: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::\d{2})?$/.exec(date);

  if (!match) {
    return date;
  }

  const [, year, month, day, hour, minute] = match;
  const currentYear = new Date().getFullYear().toString();

  if (year === currentYear) {
    return `${month}-${day} ${hour}:${minute}`;
  }

  return `${year}-${month}-${day} ${hour}:${minute}`;
}

export default TableRow
