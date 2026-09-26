import { createEffect, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { getHistoryForMac } from "../functions/history";
import { Host, show } from "../functions/exports";
import { createStore } from "solid-js/store";
import { getHistoryPeriod, historyPeriodLabel, parseHistoryTimestamp } from "../functions/historyPeriods";

function MacHistory(_props: any) {

  const [hist, setHist] = createStore<Host[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal("");
  let interval: number;
  let requestID = 0;

  const loadHistory = async (mac: string, date: string, foreground: boolean) => {
    const activeRequest = ++requestID;
    if (foreground) {
      setLoading(true);
    }
    setLoadError("");

    try {
      const newHistory = await getHistoryForMac(mac, date);
      if (activeRequest !== requestID) {
        return;
      }
      setHist(newHistory);
    } catch {
      if (activeRequest !== requestID) {
        return;
      }
      setHist([]);
      setLoadError("Presence history could not be loaded.");
    } finally {
      if (activeRequest === requestID && foreground) {
        setLoading(false);
      }
    }
  };

  createEffect(() => {
    const mac = _props.mac;
    const date = _props.date;
    void loadHistory(mac, date, true);
  });

  onMount(() => {
    interval = window.setInterval(() => {
      void loadHistory(_props.mac, _props.date, false);
    }, 60000);
  });

  onCleanup(() => {
    requestID++;
    window.clearInterval(interval);
  });

  const statusLabel = (host: Host) => host.Now === 0 ? "Offline" : "Online";
  const knownLabel = (host: Host) => host.Known === 0 ? "Unknown" : "Known";

  const sampleTitle = (host: Host) => {
    const period = host.Now === 1 ? historyPeriodLabel(getHistoryPeriod(host.Date)) : "";

    return "Date: " + host.Date
      + "\nStatus: " + statusLabel(host)
      + (period ? "\nPeriod: " + period : "")
      + "\nIface: " + host.Iface
      + "\nIP: " + host.IP
      + "\nKnown: " + knownLabel(host);
  };

  const sampleClass = (host: Host) => {
    if (host.Now === 0) {
      return "my-box-off";
    }

    const period = getHistoryPeriod(host.Date);

    if (period === "night") {
      return "my-box-on my-box-on-night";
    }

    return period === "day" ? "my-box-on my-box-on-day" : "my-box-on";
  };

  const boundaryClass = (host: Host, index: number) => {
    if (index === 0) {
      return "";
    }

    const previous = hist[index - 1];

    if (!previous) {
      return "";
    }

    const currentDate = parseHistoryTimestamp(host.Date);
    const previousDate = parseHistoryTimestamp(previous.Date);

    if (!currentDate || !previousDate) {
      return "";
    }

    if (currentDate.year !== previousDate.year
      || currentDate.month !== previousDate.month
      || currentDate.day !== previousDate.day) {
      return " history-sample-day-break";
    }

    if (currentDate.hour !== previousDate.hour) {
      return " history-sample-hour-break";
    }

    return "";
  };

  return (
    <Show
      when={!loading()}
      fallback={<div class="device-cell-muted" role="status">Loading presence history…</div>}
    >
      <Show
        when={!loadError()}
        fallback={
          <div class="host-section-error" role="alert">
            <span>{loadError()}</span>
            <button
              type="button"
              class="btn btn-sm wyl-button"
              onClick={() => void loadHistory(_props.mac, _props.date, true)}
            >
              Retry
            </button>
          </div>
        }
      >
        <Show
          when={hist.length > 0}
          fallback={
            <div class="device-cell-muted">
              {_props.date ? "No presence data recorded for this date." : "No presence data recorded yet."}
            </div>
          }
        >
          <div class="history-sample-strip" aria-label="Presence samples">
            <For each={hist}>{(h, index) =>
              <Show
                when={index() < show()}
              >
                <i
                  title={sampleTitle(h)}
                  aria-label={sampleTitle(h)}
                  role="img"
                  class={sampleClass(h) + " history-sample" + boundaryClass(h, index())}
                ></i>
              </Show>
            }</For>
          </div>
        </Show>
      </Show>
    </Show>
  )
}

export default MacHistory
