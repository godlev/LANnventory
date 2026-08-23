import { Show } from "solid-js";
import { editNames, selectedIDs, setEditNames, setSelectedIDs } from "../../functions/exports";
import Filter from "../Filter";
import Search from "../Search";
import { getHosts } from "../../functions/atstart";
import { apiDelHost } from "../../functions/api";

function CardHead() {

  const handleEditNames = (toggle: boolean) => {
    if (!toggle) {
      getHosts();
      setSelectedIDs([]);
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
          fallback={
            <button class="btn btn-sm wyl-button device-edit-button" title="Edit device names" onClick={[handleEditNames, true]}>
              <i class="bi bi-pencil-fill" aria-hidden="true"></i>
              <span>Edit</span>
            </button>
          }
        >
          <Show when={selectedIDs().length > 0}>
            <button type="button" onClick={handleDel} title="Delete selected hosts" class="btn btn-sm wyl-button device-delete-button">
              <i class="bi bi-trash3-fill" aria-hidden="true"></i>
              <span>Delete ({selectedIDs().length})</span>
            </button>
          </Show>
          <button class="btn btn-sm wyl-button device-edit-button" title="Finish editing" onClick={[handleEditNames, false]}>
            <i class="bi bi-check-lg" aria-hidden="true"></i>
            <span>Done</span>
          </button>
        </Show>
      </div>
    </div>
  )
}

export default CardHead
