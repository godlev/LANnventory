import { For, Show, type ParentProps } from "solid-js";
import { filterState, hasMultipleIfaces, Host, ifaces, setHistUpdOnFilter } from "../functions/exports";
import { filterFunc, resetFilters } from "../functions/filter";

function HistoryFilters(props: ParentProps) {
  type FilterEvent = Event & {
    currentTarget: HTMLSelectElement;
    target: HTMLSelectElement;
  };

  const handleFilter = (field: keyof Host, event: FilterEvent) => {
    filterFunc(field, event.currentTarget.value);
    setHistUpdOnFilter(true);
  };

  const handleReset = () => {
    resetFilters();
    setHistUpdOnFilter(true);
  };

  const hasActiveFilter = () => {
    const filters = filterState();
    return filters.Iface !== "" || filters.Known !== "" || filters.Now !== "" || filters.Search !== "";
  };

  const selectClass = (active: boolean) => {
    return "form-select form-select-sm device-filter-select history-filter-select" + (active ? " is-active" : "");
  };

  return (
    <div class="history-filter-group">
      <Show when={hasMultipleIfaces()}>
        <select onChange={(event)=>{handleFilter("Iface", event)}} class={selectClass(filterState().Iface !== "") + " history-filter-iface"} title="Filter history by interface" value={filterState().Iface}>
          <option value="">All interfaces</option>
          <For each={ifaces()}>{(iface) =>
            <option value={iface}>{iface}</option>
          }</For>
        </select>
      </Show>
      <select onChange={(event)=>{handleFilter("Known", event)}} class={selectClass(filterState().Known !== "")} title="Filter by recognition state" value={filterState().Known}>
        <option value="">All devices</option>
        <option value="1">Known devices</option>
        <option value="0">Unknown devices</option>
      </select>
      <select onChange={(event)=>{handleFilter("Now", event)}} class={selectClass(filterState().Now !== "")} title="Filter history by online state" value={filterState().Now}>
        <option value="">All statuses</option>
        <option value="1">Online devices</option>
        <option value="0">Offline devices</option>
      </select>
      {props.children}
      <Show when={hasActiveFilter()}>
        <button onClick={handleReset} class="btn btn-sm device-reset-filter" title="Reset history filters">
          <i class="bi bi-arrow-counterclockwise" aria-hidden="true"></i>
          <span>Reset filter</span>
        </button>
      </Show>
    </div>
  )
}

export default HistoryFilters
