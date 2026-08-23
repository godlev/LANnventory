import { For, Show } from "solid-js";
import { Host, SortDirection, sortState } from "../../functions/exports";
import { sortByAnyField } from "../../functions/sort";

const headers: { label: string; field: keyof Host }[] = [
  { label: "Name", field: "Name" },
  { label: "Iface", field: "Iface" },
  { label: "IP", field: "IP" },
  { label: "MAC", field: "Mac" },
  { label: "Hardware", field: "Hw" },
  { label: "Date", field: "Date" },
  { label: "Known", field: "Known" },
  { label: "On", field: "Now" },
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
        <th style="width: 2em;"></th>
        <For each={headers}>{(header) =>
          <th 
            class="sortable-th"
            style={sortState().field === header.field ? "color: var(--bs-primary);" : ''}
            aria-sort={ariaSort(header.field)}
            tabIndex={0}
            title={"Sort by " + header.label}
            onClick={[handleSort, header.field]}
            onKeyDown={(event) => handleKeyDown(event, header.field)}
          >
            {header.label}
            <Show when={sortState().field === header.field}>
              <i class={"bi " + sortIcon(sortState().direction) + " ms-1"} aria-hidden="true"></i>
            </Show>
          </th>
        }</For>
        <th style="width: 2em;" title="Edit"><i class="bi bi-pencil-fill"></i></th>
      </tr>
    </thead>
  )
}

export default TableHead
