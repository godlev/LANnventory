import { createSignal, Show } from "solid-js";
import { editNames, selectedIDs, setSelectedIDs } from "../../functions/exports";
import { apiEditHost } from "../../functions/api";

import { debounce } from "@solid-primitives/scheduled"; 

function TableRow(_props: any) {

  const [name, setName] = createSignal(_props.host.Name);

  const isOnline = () => _props.host.Now === 1;
  const known = () => _props.host.Known === 1;
  const lastSeen = () => formatLastSeen(_props.host.Date);

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
        <div class="form-check form-switch">
          <input class="form-check-input" type="checkbox" checked={known()}
            onClick={handleToggle}></input>
        </div>
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
      <td class="device-table-status">
        <span class={isOnline() ? "status-pill status-pill-online" : "status-pill status-pill-offline"}>
          <i class={isOnline() ? "bi bi-check-circle-fill" : "bi bi-slash-circle-fill"} aria-hidden="true"></i>
          {isOnline() ? "Online" : "Offline"}
        </span>
      </td>
      <td class="device-table-ip"><a href={"http://" + _props.host.IP} target="_blank">{_props.host.IP}</a></td>
      <td class="device-table-iface"><span class="device-cell-muted">{_props.host.Iface}</span></td>
      <td class="device-table-mac"><span class="device-cell-muted">{_props.host.Mac}</span></td>
      <td class="device-table-hardware" title={_props.host.Hw}>
        <span class="device-hardware-text">{_props.host.Hw}</span>
      </td>
      <td class="device-table-last-seen" title={_props.host.Date}>
        <span class="device-cell-muted">{lastSeen()}</span>
      </td>
      <td class="device-table-actions">
        <Show
          when={editNames()}
          fallback={
          <a href={"/host/" + _props.host.ID}>
            <i class="bi bi-three-dots-vertical my-btn p-2" title="More"></i>
          </a>}
        >
          <input
            type="checkbox"
            class="form-check-input"
            checked={selectedIDs().includes(_props.host.ID)}
            onChange={e => handleCheck((e.target as HTMLInputElement).checked)}
          />
        </Show>
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
