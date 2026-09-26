import { useBeforeLeave, useLocation, useNavigate, useParams } from "@solidjs/router";
import { createEffect, createSignal, onCleanup, Show } from "solid-js";

import { apiGetHost, type DeviceProfileResponse } from "../functions/api";
import { deviceDisplayName } from "../functions/deviceIdentity";

import HostCard from "../components/HostPage/HostCard";
import HostedWorkloadCard from "../components/HostPage/HostedWorkloadCard";
import DeviceProfileCard from "../components/HostPage/DeviceProfileCard";
import ProxmoxInventoryCard from "../components/HostPage/ProxmoxInventoryCard";
import IdentityCard from "../components/HostPage/IdentityCard";
import ServicesCard from "../components/HostPage/ServicesCard";
import HostActivityCard from "../components/HostPage/HostActivityCard";
import HistCard from "../components/HostPage/HistCard";
import { emptyHost, emptyPageContext, Host, setPageContext } from "../functions/exports";

type HostSection = "inventory" | "network" | "activity";

function HostPage() {

  const [currentHost, setCurrentHost] = createSignal<Host>(emptyHost);
  const [loadError, setLoadError] = createSignal("");
  const [deviceProfile, setDeviceProfile] = createSignal<DeviceProfileResponse | null>(null);
  const [hasUnsavedHostChanges, setHasUnsavedHostChanges] = createSignal(false);
  const [activeSection, setActiveSection] = createSignal<HostSection>("inventory");
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const previousTitle = document.title;
  let requestId = 0;

  const isEditMode = () => new URLSearchParams(location.search).get("edit") === "1";
  const setEditMode = (editing: boolean) => {
    if (!params.id) {
      return;
    }

    navigate("/host/" + params.id + (editing ? "?edit=1" : ""));
  };

  useBeforeLeave((event) => {
    if (!hasUnsavedHostChanges() || event.defaultPrevented) {
      return;
    }

    event.preventDefault();
    setTimeout(() => {
      if (window.confirm("Discard unsaved host changes?")) {
        setHasUnsavedHostChanges(false);
        event.retry(true);
      }
    }, 0);
  });

  const handleBeforeUnload = (event: BeforeUnloadEvent) => {
    if (!hasUnsavedHostChanges()) {
      return;
    }

    event.preventDefault();
    event.returnValue = "";
  };
  window.addEventListener("beforeunload", handleBeforeUnload);

  createEffect(() => {
    const id = params.id;

    if (!id) {
      return;
    }

    const activeRequest = ++requestId;
    setLoadError("");
    setActiveSection("inventory");
    setDeviceProfile(null);
    setCurrentHost(emptyHost);
    setPageContext({ kind: "host", hostName: "" });
    document.title = "Host · LANnventory";

    apiGetHost(id)
      .then((host) => {
        if (activeRequest !== requestId) {
          return;
        }

        setCurrentHost(host);
      })
      .catch(() => {
        if (activeRequest !== requestId) {
          return;
        }

        setPageContext({ kind: "host", hostName: "" });
        document.title = "Host · LANnventory";
        setLoadError("Host details could not be loaded. The device may have been deleted or the backend may be unavailable.");
      });
  });

  onCleanup(() => {
    requestId++;
    window.removeEventListener("beforeunload", handleBeforeUnload);
    setHasUnsavedHostChanges(false);
    setPageContext(emptyPageContext);
    document.title = previousTitle;
  });

  createEffect(() => {
    const host = currentHost();

    if (host.ID === 0) {
      return;
    }

    const hostName = deviceDisplayName(host);
    setPageContext({ kind: "host", hostName });
    document.title = hostName + " · LANnventory";
  });

  return (
    <div class="host-page">
    <Show
      when={!loadError()}
      fallback={
        <div class="data-load-warning" role="alert">
          <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
          <span>{loadError()}</span>
        </div>
      }
    >
      <section class="host-workspace" aria-label="Host workspace">
        <HostCard
          host={currentHost()}
          editMode={isEditMode()}
          onEditModeChange={setEditMode}
          onHostChange={setCurrentHost}
          onDirtyChange={setHasUnsavedHostChanges}
        ></HostCard>

        <div class="host-section-nav-shell">
          <nav class="host-section-nav" role="tablist" aria-label="Host sections">
            <button
              id="host-section-inventory-tab"
              type="button"
              role="tab"
              aria-selected={activeSection() === "inventory"}
              aria-controls="host-section-inventory"
              class={"host-section-tab" + (activeSection() === "inventory" ? " is-active" : "")}
              onClick={() => setActiveSection("inventory")}
            >
              <i class="bi bi-box-seam" aria-hidden="true"></i>
              <span>Inventory</span>
            </button>
            <button
              id="host-section-network-tab"
              type="button"
              role="tab"
              aria-selected={activeSection() === "network"}
              aria-controls="host-section-network"
              class={"host-section-tab" + (activeSection() === "network" ? " is-active" : "")}
              onClick={() => setActiveSection("network")}
            >
              <i class="bi bi-diagram-3" aria-hidden="true"></i>
              <span>Network</span>
            </button>
            <button
              id="host-section-activity-tab"
              type="button"
              role="tab"
              aria-selected={activeSection() === "activity"}
              aria-controls="host-section-activity"
              class={"host-section-tab" + (activeSection() === "activity" ? " is-active" : "")}
              onClick={() => setActiveSection("activity")}
            >
              <i class="bi bi-clock-history" aria-hidden="true"></i>
              <span>Activity</span>
            </button>
          </nav>
        </div>

        <div class="host-section-content">
          <section
            id="host-section-inventory"
            class="host-section-pane"
            role="tabpanel"
            aria-labelledby="host-section-inventory-tab"
            hidden={activeSection() !== "inventory"}
          >
            <div class="host-section-stack">
              <HostedWorkloadCard host={currentHost()}></HostedWorkloadCard>
              <DeviceProfileCard host={currentHost()} onProfileChange={setDeviceProfile}></DeviceProfileCard>
              <Show when={deviceProfile()?.hypervisor?.platform === "proxmox-ve"}>
                <ProxmoxInventoryCard host={currentHost()}></ProxmoxInventoryCard>
              </Show>
            </div>
          </section>

          <section
            id="host-section-network"
            class="host-section-pane"
            role="tabpanel"
            aria-labelledby="host-section-network-tab"
            hidden={activeSection() !== "network"}
          >
            <div class="host-section-stack">
              <ServicesCard host={currentHost()}></ServicesCard>
              <IdentityCard host={currentHost()}></IdentityCard>
            </div>
          </section>

          <section
            id="host-section-activity"
            class="host-section-pane"
            role="tabpanel"
            aria-labelledby="host-section-activity-tab"
            hidden={activeSection() !== "activity"}
          >
            <div class="host-section-stack">
              <HostActivityCard host={currentHost()}></HostActivityCard>
              <HistCard mac={currentHost().Mac}></HistCard>
            </div>
          </section>
        </div>
      </section>
    </Show>
    </div>
  )
}

export default HostPage
