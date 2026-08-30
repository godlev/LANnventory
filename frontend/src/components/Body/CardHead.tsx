import { Show } from "solid-js";
import { editNames, selectedIDs, setEditNames, setSelectedIDs } from "../../functions/exports";
import { getHosts } from "../../functions/atstart";
import Filter from "../Filter";
import Search from "../Search";
import { apiDelHost } from "../../functions/api";
import DeviceDisplaySelect from "./DeviceDisplaySelect";

function CardHead() {

  const handleDel = async () => {
    const ids = selectedIDs();
    
    for (let id of ids) {
      await apiDelHost(id);
    }
    
    window.location.href = '/';
  };

  const handleEditMode = async () => {
    const next = !editNames();

    if (!next) {
      await getHosts();
      setSelectedIDs([]);
    }

    setEditNames(next);
  };

  const editTitle = () => editNames() ? "Finish editing" : "Edit devices";

  return (
    <div class="device-toolbar">
      <div class="device-toolbar-filters">
        <Filter></Filter>
      </div>
      <div class="device-toolbar-actions">
        <DeviceDisplaySelect></DeviceDisplaySelect>
        <button
          type="button"
          class="device-header-action device-mobile-edit-toggle"
          title={editTitle()}
          aria-label={editTitle()}
          aria-pressed={editNames()}
          onClick={handleEditMode}
        >
          <i class={editNames() ? "bi bi-check-lg" : "bi bi-pencil-fill"} aria-hidden="true"></i>
        </button>
        <Search></Search>
        <Show when={editNames() && selectedIDs().length > 0}>
          <button type="button" onClick={handleDel} title="Delete selected hosts" class="btn btn-sm wyl-button device-delete-button">
            <i class="bi bi-trash3-fill" aria-hidden="true"></i>
            <span>DELETE ({selectedIDs().length})</span>
          </button>
        </Show>
      </div>
    </div>
  )
}

export default CardHead
