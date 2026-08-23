import { For, Show } from "solid-js";
import { Host, SortDirection, sortState } from "../../functions/exports";
import { sortByAnyField } from "../../functions/sort";

const headers: { label: string; field: keyof Host; className: string; title?: string }[] = [
  { label: "Known", field: "Known", className: "device-table-known" },
  { label: "Name", field: "Name", className: "device-table-name" },
  { label: "IP", field: "IP", className: "device-table-ip" },
  { label: "Iface", field: "Iface", className: "device-table-iface" },
  { label: "MAC", field: "Mac", className: "device-table-mac" },
  { label: "Hardware", field: "Hw", className: "device-table-hardware" },
  {
    label: "Last Seen",
    field: "Date",
    className: "device-table-last-seen",
    title: "Last time this device was observed on the network",
  },
];

function TableHead() {
  const handleSort = (field: keyof Host) => {
    sortByAnyField(field);
  };

  const handleKeyDown = (event: KeyboardEvent, field: keyof Host) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      handleSort(field);
    }
  };

  const ariaSort = (field: keyof Host): "ascending" | "descending" | "none" => {
    const state = sortState();
    return state.field === field && state.direction ? state.direction : "none";
  };

  const sortIcon = (direction: SortDirection | "") => {
    return direction === "ascending" ? "bi-arrow-up-short" : "bi-arrow-down-short";
  };

  return (
    <thead>
      <tr>
        <th class="device-table-index">#</th>
        <For each={headers}>{(header, index) =>
          <>
            <th
              class={"sortable-th " + header.className}
              style={sortState().field === header.field ? "color: var(--bs-primary);" : ''}
              aria-sort={ariaSort(header.field)}
              tabIndex={0}
              title={header.title ? header.title : "Sort by " + header.label}
              onClick={[handleSort, header.field]}
              onKeyDown={(event) => handleKeyDown(event, header.field)}
            >
              {header.label}
              <Show when={sortState().field === header.field}>
                <i class={"bi " + sortIcon(sortState().direction) + " ms-1"} aria-hidden="true"></i>
              </Show>
            </th>
            <Show when={index() === 0}>
              <th class="device-table-actions" title="Actions" aria-label="Actions">
                <i class="bi bi-pencil-fill" aria-hidden="true"></i>
              </th>
            </Show>
          </>
        }</For>
      </tr>
    </thead>
  )
}

export default TableHead
