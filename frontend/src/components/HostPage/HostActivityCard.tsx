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
  let requestId = 0;
  let refreshTimer = 0;

  const loadActivity = async (hostID: number) => {
    if (hostID < 1) {
      requestId++;
      setEvents([]);
      return;
    }

    const activeRequest = ++requestId;
    try {
      const nextEvents = await apiGetHostActivity(hostID, hostActivityLimit);
      if (activeRequest === requestId) {
        setEvents(nextEvents);
      }
    } catch {
      if (activeRequest === requestId) {
        setEvents([]);
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
          <div class="activity-panel-subtitle">Historical snapshots · IP and MAC are shown as recorded at event time.</div>
        </div>
      </div>
      <div class="card-body activity-panel-body">
        <Show
          when={events().length > 0}
          fallback={<div class="activity-empty">No recorded events yet</div>}
        >
          <div class="host-event-grid" role="table" aria-label="Recent host events">
            <div class="host-event-row host-event-row-header" role="row">
              <div class="host-event-cell" role="columnheader">Event</div>
              <div class="host-event-cell" role="columnheader">IP</div>
              <div class="host-event-cell" role="columnheader">MAC</div>
              <div class="host-event-cell" role="columnheader">Time</div>
            </div>
            <For each={events()}>{(event) =>
              <div class={"host-event-row host-event-row-" + activityTone(event.EventType)} role="row">
                <div class="host-event-cell host-event-event" data-label="Event" role="cell">
                  <span class="host-event-event-content">
                    <span class="host-event-icon" aria-hidden="true">
                      <i class={"bi " + activityIcon(event.EventType)}></i>
                    </span>
                    <span>{activityDescription(event)}</span>
                  </span>
                </div>
                <div class="host-event-cell font-monospace" data-label="IP" role="cell">
                  {event.IP || "—"}
                </div>
                <div class="host-event-cell font-monospace" data-label="MAC" role="cell">
                  {event.Mac || "—"}
                </div>
                <div class="host-event-cell" data-label="Time" role="cell">
                  <time class="host-event-time" dateTime={activityTimeDateTime(event)} title={activityTimeTitle(event)}>
                    {relativeActivityTime(event) || "—"}
                  </time>
                </div>
              </div>
            }</For>
          </div>
        </Show>
      </div>
    </section>
  );
}

export default HostActivityCard;
