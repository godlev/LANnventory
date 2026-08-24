import { A } from "@solidjs/router";
import { createSignal, For, onMount, Show } from "solid-js";

import { apiGetActivity, type ActivityCategory } from "../functions/api";
import {
  activityDescription,
  activityDetails,
  activityDeviceIcon,
  activityHostName,
  activityIcon,
  activityTone,
  relativeActivityTime,
} from "../functions/activity";
import type { HostEvent } from "../functions/exports";

type ActivityStreamCategory = Exclude<ActivityCategory, "all">;

type ActivityTableProps = {
  category: ActivityStreamCategory;
  title: string;
  subtitle: string;
  emptyText: string;
  variant: ActivityStreamCategory;
  hostExists: (event: HostEvent) => boolean;
};

const activityPageSize = 50;

function ActivityTable(props: ActivityTableProps) {
  const [events, setEvents] = createSignal<HostEvent[]>([]);
  const [hasMore, setHasMore] = createSignal(false);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal("");
  let requestId = 0;

  const canLinkHost = (event: HostEvent) => event.HostID > 0 && props.hostExists(event);

  const loadPage = async () => {
    const activeRequest = ++requestId;
    const offset = events().length;

    setLoading(true);
    setError("");

    try {
      const nextEvents = await apiGetActivity(activityPageSize, {
        category: props.category,
        offset,
      });
      if (activeRequest !== requestId) {
        return;
      }

      setEvents((currentEvents) => [...currentEvents, ...nextEvents]);
      setHasMore(nextEvents.length === activityPageSize);
    } catch {
      if (activeRequest !== requestId) {
        return;
      }

      setError("Activity could not be loaded");
      setHasMore(false);
    } finally {
      if (activeRequest === requestId) {
        setLoading(false);
      }
    }
  };

  onMount(() => {
    loadPage();
  });

  return (
    <section class="card wyl-panel activity-table-panel" aria-labelledby={"activity-" + props.category + "-title"}>
      <div class="card-header activity-table-header">
        <div>
          <div id={"activity-" + props.category + "-title"} class="activity-table-title">{props.title}</div>
          <div class="activity-table-subtitle">{props.subtitle}</div>
        </div>
      </div>
      <div class="card-body activity-table-body">
        <div class="table-responsive">
          <table class={"table table-hover activity-table activity-table-" + props.variant}>
            <thead>
              <tr>
                <th scope="col">Device</th>
                <th scope="col">Event</th>
                <Show
                  when={props.variant === "connectivity"}
                  fallback={<th scope="col">Details</th>}
                >
                  <th scope="col">IP</th>
                  <th scope="col">Iface</th>
                </Show>
                <th scope="col">Time</th>
              </tr>
            </thead>
            <tbody>
              <For each={events()}>{(event) =>
                <tr class={"activity-table-row activity-row-" + activityTone(event.EventType)}>
                  <td data-label="Device">
                    <span class="activity-table-device">
                      <span class="activity-host-icon" aria-hidden="true">
                        <i class={"bi " + activityDeviceIcon(event)}></i>
                      </span>
                      <Show
                        when={canLinkHost(event)}
                        fallback={<span class="activity-host-name">{activityHostName(event)}</span>}
                      >
                        <A href={"/host/" + event.HostID} class="activity-host-link">{activityHostName(event)}</A>
                      </Show>
                    </span>
                  </td>
                  <td data-label="Event">
                    <span class="activity-table-event">
                      <span class="activity-event-icon" aria-hidden="true">
                        <i class={"bi " + activityIcon(event.EventType)}></i>
                      </span>
                      <span>{activityDescription(event)}</span>
                    </span>
                  </td>
                  <Show
                    when={props.variant === "connectivity"}
                    fallback={
                      <td data-label="Details" class="activity-table-muted">
                        {activityDetails(event) || " "}
                      </td>
                    }
                  >
                    <td data-label="IP" class="activity-table-muted">{event.IP || " "}</td>
                    <td data-label="Iface" class="activity-table-muted">{event.Iface || " "}</td>
                  </Show>
                  <td data-label="Time">
                    <time class="activity-time" dateTime={event.Date} title={event.Date}>
                      {relativeActivityTime(event.Date)}
                    </time>
                  </td>
                </tr>
              }</For>
            </tbody>
          </table>
        </div>
        <Show when={!loading() && events().length === 0 && error() === ""}>
          <div class="activity-empty">{props.emptyText}</div>
        </Show>
        <Show when={error()}>
          <div class="activity-empty" role="status">{error()}</div>
        </Show>
        <Show when={hasMore()}>
          <div class="activity-table-actions">
            <button
              type="button"
              class="btn btn-sm wyl-button activity-load-more"
              disabled={loading()}
              onClick={loadPage}
            >
              <i class="bi bi-chevron-down" aria-hidden="true"></i>
              <span>{loading() ? "Loading" : "Load more"}</span>
            </button>
          </div>
        </Show>
      </div>
    </section>
  );
}

export default ActivityTable;
