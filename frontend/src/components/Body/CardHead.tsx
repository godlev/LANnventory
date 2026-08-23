import { Show } from "solid-js";
import { editNames, selectedIDs, setEditNames } from "../../functions/exports";
import Filter from "../Filter";
import Search from "../Search";
import { getHosts } from "../../functions/atstart";
import { apiDelHost } from "../../functions/api";

function CardHead() {

  const handleEditNames = (toggle: boolean) => {
    if (!toggle) {
      getHosts();
    }
    setEditNames(toggle);
  };

  const handleDel = async () => {
    const ids = selectedIDs();
    
    for (let id of ids) {
      await apiDelHost(id);
    }
    
    window.location.href = '/';
  };

  return (
    <div class="device-toolbar">
      <div class="device-toolbar-filters">
        <Filter></Filter>
      </div>
      <div class="device-toolbar-actions">
        <Search></Search>
        <Show
          when={editNames()}
          fallback={<button class="btn btn-outline-primary btn-sm" title="Toggle edit" onClick={[handleEditNames, true]}>Edit</button>}
        >
          <button type="button" onClick={handleDel} title="Delete selected hosts" class="btn btn-outline-danger btn-sm">Delete selected</button>
          <button class="btn btn-primary btn-sm" title="Toggle edit" onClick={[handleEditNames, false]}>Edit</button>
        </Show>
      </div>
    </div>
  )
}

export default CardHead
