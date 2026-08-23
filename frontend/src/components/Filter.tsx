import { For } from "solid-js";
import { filterState, Host, ifaces, setHistUpdOnFilter } from "../functions/exports";
import { filterFunc, resetFilters } from "../functions/filter";


function Filter() {
  type FilterEvent = Event & {
    currentTarget: HTMLSelectElement;
    target: HTMLSelectElement;
  };

  const handleFilter = (field: keyof Host, event: FilterEvent) => {
    const value = event.currentTarget.value;
    filterFunc(field, value);
    setHistUpdOnFilter(true);
  };

  const handleReset = () => {
    resetFilters();
    setHistUpdOnFilter(true);
  };

  const selectValue = (value: string | number) => value === "" ? "" : value.toString();

  return (
    <div class="input-group device-filter-group">
        <select onChange={(event)=>{handleFilter("Iface", event)}} class="form-select form-select-sm" title="Filter by Iface" value={filterState().Iface}>
          <option value="">Iface</option>
          <For each={ifaces()}>{(iface) =>
            <option value={iface}>{iface}</option>
          }</For>
        </select>
        <select onChange={(event)=>{handleFilter("Known", event)}} class="form-select form-select-sm" title="Filter by Known" value={selectValue(filterState().Known)}>
          <option value="">Known</option>
          <option value={1}>Known</option>
          <option value={0}>Unknown</option>
        </select>
        <select onChange={(event)=>{handleFilter("Now", event)}} class="form-select form-select-sm" title="Filter by Online" value={selectValue(filterState().Now)}>
          <option value="">Online</option>
          <option value={1}>On</option>
          <option value={0}>Off</option>
        </select>
        <button onClick={handleReset} class="btn btn-outline-primary btn-sm" title="Reset filter">Reset filter</button>
    </div>
  )
}

export default Filter
