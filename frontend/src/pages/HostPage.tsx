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

function HostPage() {

  const [currentHost, setCurrentHost] = createSignal<Host>(emptyHost);
  const [loadError, setLoadError] = createSignal("");
  const [deviceProfile, setDeviceProfile] = createSignal<DeviceProfileResponse | null>(null);
  const [hasUnsavedHostChanges, setHasUnsavedHostChanges] = createSignal(false);
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
      <div class="row g-3 mx-0 host-page-row">
        <div class="col-12 host-details-column">
          <HostCard
            host={currentHost()}
            editMode={isEditMode()}
            onEditModeChange={setEditMode}
            onHostChange={setCurrentHost}
            onDirtyChange={setHasUnsavedHostChanges}
          ></HostCard>
        </div>
      </div>
      <div class="row g-3 mx-0 mt-1 host-page-row">
        <div class="col-md">
          <HostedWorkloadCard host={currentHost()}></HostedWorkloadCard>
        </div>
      </div>
      <div class="row g-3 mx-0 mt-1 host-page-row">
        <div class="col-md">
          <DeviceProfileCard host={currentHost()} onProfileChange={setDeviceProfile}></DeviceProfileCard>
        </div>
      </div>
      <Show when={deviceProfile()?.hypervisor?.platform === "proxmox-ve"}>
        <div class="row g-3 mx-0 mt-1 host-page-row">
          <div class="col-md">
            <ProxmoxInventoryCard host={currentHost()}></ProxmoxInventoryCard>
          </div>
        </div>
      </Show>
      <div class="row g-3 mx-0 mt-1 host-page-row">
        <div class="col-md">
          <ServicesCard host={currentHost()}></ServicesCard>
        </div>
      </div>
      <div class="row g-3 mx-0 mt-1 host-page-row">
        <div class="col-md">
          <IdentityCard host={currentHost()}></IdentityCard>
        </div>
      </div>
      <div class="row g-3 mx-0 mt-1 host-page-row">
        <div class="col-md">
          <HostActivityCard host={currentHost()}></HostActivityCard>
        </div>
      </div>
      <div class="row g-3 mx-0 mt-1 host-page-row">
        <div class="col-md">
          <HistCard mac={currentHost().Mac}></HistCard>
        </div>
      </div>
    </Show>
    </div>
  )
}

export default HostPage
