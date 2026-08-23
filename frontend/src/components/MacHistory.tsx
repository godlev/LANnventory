import { For, onCleanup, onMount, Show } from "solid-js";
import { getHistoryForMac } from "../functions/history";
import { Host, show } from "../functions/exports";
import { createStore } from "solid-js/store";

function MacHistory(_props: any) {

  const [hist, setHist] = createStore<Host[]>([]);
  let interval: number;

  onMount(async () => {
    const newHistory = await getHistoryForMac(_props.mac, _props.date);
    setHist(newHistory);
    interval = setInterval(async () => {
      // console.log("Upd Hist", new Date());
      const newHistory = await getHistoryForMac(_props.mac, _props.date);
      setHist(newHistory);
    }, 60000); // 60000 ms = 1 minute
  });

  onCleanup(() => {
    clearInterval(interval);
  });

  const statusLabel = (host: Host) => host.Now === 0 ? "Offline" : "Online";
  const knownLabel = (host: Host) => host.Known === 0 ? "Unknown" : "Known";

  const sampleTitle = (host: Host) => {
    return "Date: " + host.Date
      + "\nStatus: " + statusLabel(host)
      + "\nIface: " + host.Iface
      + "\nIP: " + host.IP
      + "\nKnown: " + knownLabel(host);
  };

  const boundaryClass = (host: Host, index: number) => {
    if (index === 0) {
      return "";
    }

    const previous = hist[index - 1];

    if (!previous) {
      return "";
    }

    const currentDate = parseHistoryDate(host.Date);
    const previousDate = parseHistoryDate(previous.Date);

    if (!currentDate || !previousDate) {
      return "";
    }

    if (currentDate.toDateString() !== previousDate.toDateString()) {
      return " history-sample-day-break";
    }

    if (currentDate.getHours() !== previousDate.getHours()) {
      return " history-sample-hour-break";
    }

    return "";
  };

  return (
    <For each={hist}>{(h, index) =>
      <Show
        when={index() < show()}
      >
        <i
          title={sampleTitle(h)}
          aria-label={sampleTitle(h)}
          role="img"
          class={(h.Now === 0 ? "my-box-off" : "my-box-on") + " history-sample" + boundaryClass(h, index())}
        ></i>
      </Show>
    }</For>
  )
}

function parseHistoryDate(date: string) {
  const normalized = date.replace(" ", "T");
  const parsed = new Date(normalized);

  return isNaN(parsed.getTime()) ? null : parsed;
}

export default MacHistory
