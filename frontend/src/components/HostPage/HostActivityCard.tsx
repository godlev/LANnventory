import { createEffect, createSignal, For, onCleanup, onMount, Show } from "solid-js";

import { apiGetHostActivity } from "../../functions/api";
import {
  activityDescription,
  activityIcon,
  activityTimeDateTime,
  activityTimeTitle,
  activityTone,
  relativeActivityTime,
} from "../../functions/activity";
import type { Host, HostEvent } from "../../functions/exports";

type HostActivityCardProps = {
  host: Host;
};

const hostActivityLimit = 10;

function HostActivityCard(props: HostActivityCardProps) {
  const [events, setEvents] = createSignal<HostEvent[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [loadError, setLoadError] = createSignal("");
  let requestId = 0;
  let refreshTimer = 0;

  const loadActivity = async (hostID: number) => {
    if (hostID < 1) {
      requestId++;
      setEvents([]);
      setLoadError("");
      setLoading(false);
      return;
    }

    const activeRequest = ++requestId;
    setLoading(true);
    setLoadError("");
    try {
      const nextEvents = await apiGetHostActivity(hostID, hostActivityLimit);
      if (activeRequest === requestId) {
        setEvents(nextEvents);
      }
    } catch {
      if (activeRequest === requestId) {
        setLoadError("Recent events could not be loaded.");
      }
    } finally {
      if (activeRequest === requestId) {
        setLoading(false);
      }
    }
  };

  createEffect(() => {
    void loadActivity(props.host.ID);
  });

  onMount(() => {
    refreshTimer = window.setInterval(() => {
      void loadActivity(props.host.ID);
    }, 60000);
  });

  onCleanup(() => {
    requestId++;
    window.clearInterval(refreshTimer);
  });

  return (
    <section class="card wyl-panel activity-panel host-activity-panel" aria-labelledby="host-activity-title">
      <div class="card-header activity-panel-header">
        <div>
          <div id="host-activity-title" class="activity-panel-title">Recent events</div>
          <div class="activity-panel-subtitle">Historical event snapshots for this Host</div>
        </div>
      </div>

      <div class="card-body activity-panel-body">
        <Show when={loadError()}>
          <div class="host-section-error" role="alert">
            <span>{loadError()}</span>
            <button type="button" class="btn btn-sm wyl-button" onClick={() => void loadActivity(props.host.ID)}>
              Retry
            </button>
          </div>
        </Show>

        <Show
          when={events().length > 0}
          fallback={
            <div class="activity-empty">
              {loading() ? "Loading recent events…" : loadError() ? "No event data is available right now." : "No recorded events yet"}
            </div>
          }
        >
          <div class="table-responsive host-event-table-wrap">
            <table class="table table-sm align-middle mb-0 host-event-table">
              <thead>
                <tr>
                  <th scope="col">Event</th>
                  <th scope="col">IP</th>
                  <th scope="col">MAC</th>
                  <th scope="col">When</th>
                </tr>
              </thead>
              <tbody>
                <For each={events()}>{(event) =>
                  <tr class={"host-event-row activity-row-" + activityTone(event.EventType)}>
                    <td data-label="Event">
                      <span class="host-event-mobile-label">Event</span>
                      <span class="host-event-description">
                        <span class="host-event-icon" aria-hidden="true">
                          <i class={"bi " + activityIcon(event.EventType)}></i>
                        </span>
                        <span>{activityDescription(event)}</span>
                      </span>
                    </td>
                    <td data-label="IP">
                      <span class="host-event-mobile-label">IP</span>
                      <span class="font-monospace host-event-network-value">{event.IP || "—"}</span>
                    </td>
                    <td data-label="MAC">
                      <span class="host-event-mobile-label">MAC</span>
                      <span class="font-monospace host-event-network-value">{event.Mac || "—"}</span>
                    </td>
                    <td data-label="When">
                      <span class="host-event-mobile-label">When</span>
                      <time
                        class="activity-time"
                        dateTime={activityTimeDateTime(event)}
                        title={activityTimeTitle(event)}
                      >
                        {relativeActivityTime(event)}
                      </time>
                    </td>
                  </tr>
                }</For>
              </tbody>
            </table>
          </div>
        </Show>
      </div>
    </section>
  );
}

export default HostActivityCard;
