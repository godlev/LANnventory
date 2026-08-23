import { useParams } from "@solidjs/router";
import { createEffect, createSignal, onCleanup } from "solid-js";

import { apiGetHost } from "../functions/api";

import HostCard from "../components/HostPage/HostCard";
import Ping from "../components/HostPage/Ping";
import HistCard from "../components/HostPage/HistCard";
import { emptyHost, emptyPageContext, Host, setPageContext } from "../functions/exports";

function HostPage() {

  const [currentHost, setCurrentHost] = createSignal<Host>(emptyHost);
  const params = useParams();
  const previousTitle = document.title;
  let requestId = 0;

  createEffect(() => {
    const id = params.id;

    if (!id) {
      return;
    }

    const activeRequest = ++requestId;
    setCurrentHost(emptyHost);
    setPageContext({ kind: "host", hostName: "" });
    document.title = "Host · WatchYourLAN2";

    apiGetHost(id)
      .then((host) => {
        if (activeRequest !== requestId) {
          return;
        }

        const hostName = host.Name.trim();
        setCurrentHost(host);
        setPageContext({ kind: "host", hostName });
        document.title = (hostName || "Host") + " · WatchYourLAN2";
      })
      .catch(() => {
        if (activeRequest !== requestId) {
          return;
        }

        setPageContext({ kind: "host", hostName: "" });
        document.title = "Host · WatchYourLAN2";
      });
  });

  onCleanup(() => {
    requestId++;
    setPageContext(emptyPageContext);
    document.title = previousTitle;
  });

  return (
    <>
    <div class="row">
      <div class="col-md">
        <HostCard host={currentHost()}></HostCard>
      </div>
      <div class="col-md">
        <Ping IP={currentHost().IP}></Ping>
      </div>
    </div>
    <div class="row mt-4">
      <div class="col-md">
        <HistCard mac={currentHost().Mac}></HistCard>
      </div>
    </div>
    </>
  )
}

export default HostPage
