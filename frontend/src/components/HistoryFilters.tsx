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

  return (
    <div class="history-filter-group">
      <Show when={hasMultipleIfaces()}>
        <select onChange={(event)=>{handleFilter("Iface", event)}} class="form-select form-select-sm device-filter-select history-filter-select history-filter-iface" title="Filter history by interface" value={filterState().Iface}>
          <option value="">Iface</option>
          <For each={ifaces()}>{(iface) =>
            <option value={iface}>{iface}</option>
          }</For>
        </select>
      </Show>
      <select onChange={(event)=>{handleFilter("Known", event)}} class="form-select form-select-sm device-filter-select history-filter-select" title="Filter history by known state" value={filterState().Known}>
        <option value="">Known</option>
        <option value="1">Known</option>
        <option value="0">Unknown</option>
      </select>
      <select onChange={(event)=>{handleFilter("Now", event)}} class="form-select form-select-sm device-filter-select history-filter-select" title="Filter history by online state" value={filterState().Now}>
        <option value="">Status</option>
        <option value="1">Online</option>
        <option value="0">Offline</option>
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
