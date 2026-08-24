import { createSignal, onCleanup, onMount, Show } from "solid-js";

import { apiGetActivity } from "../../functions/api";
import { bkpHosts, type HostEvent } from "../../functions/exports";
import ActivityFeed from "../ActivityFeed";

const collapsedLimit = 6;
const activityLimit = 20;

function RecentActivityPanel() {
  const [events, setEvents] = createSignal<HostEvent[]>([]);
  const [expanded, setExpanded] = createSignal(false);
  let refreshTimer = 0;

  const loadActivity = async () => {
    try {
      setEvents(await apiGetActivity(activityLimit));
    } catch {
      setEvents([]);
    }
  };

  onMount(() => {
    loadActivity();
    refreshTimer = window.setInterval(loadActivity, 60000);
  });

  onCleanup(() => {
    window.clearInterval(refreshTimer);
  });

  const visibleEvents = () => expanded() ? events() : events().slice(0, collapsedLimit);
  const hostExists = (event: HostEvent) => bkpHosts().some((host) => host.ID === event.HostID && host.Mac === event.Mac);

  return (
    <section class="card wyl-panel activity-panel" aria-labelledby="recent-activity-title">
      <div class="card-header activity-panel-header">
        <div>
          <div id="recent-activity-title" class="activity-panel-title">Recent activity</div>
          <div class="activity-panel-subtitle">{events().length === 1 ? "1 event" : events().length + " events"}</div>
        </div>
        <Show when={events().length > collapsedLimit}>
          <button
            type="button"
            class="btn btn-sm wyl-button activity-show-more"
            aria-expanded={expanded()}
            onClick={() => setExpanded(!expanded())}
          >
            <i class={expanded() ? "bi bi-chevron-up" : "bi bi-chevron-down"} aria-hidden="true"></i>
            <span>{expanded() ? "Show less" : "Show more"}</span>
          </button>
        </Show>
      </div>
      <div class="card-body activity-panel-body">
        <ActivityFeed
          events={visibleEvents()}
          emptyText="No recent activity"
          hostExists={hostExists}
        ></ActivityFeed>
      </div>
    </section>
  );
}

export default RecentActivityPanel;
