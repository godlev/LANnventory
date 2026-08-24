import { A } from "@solidjs/router";
import { createEffect, createMemo, createSignal, For, onMount, Show } from "solid-js";

import {
  apiGetActivity,
  apiGetActivityDevices,
  apiGetActivityStats,
  type ActivityCategory,
  type ActivityEventType,
} from "../functions/api";
import {
  activityCategoryLabel,
  activityDayLabel,
  activityDescription,
  activityDetails,
  activityDeviceIcon,
  activityDeviceTypeLabel,
  activityEventLabel,
  activityHostName,
  activityIcon,
  activityTone,
  relativeActivityTime,
} from "../functions/activity";
import { appConfig, bkpHosts, type ActivityDeviceOption, type ActivityStats, type HostEvent } from "../functions/exports";

type EventFilterKey =
  | "all"
  | "connectivity"
  | "online"
  | "offline"
  | "changes"
  | "discovered"
  | "recognition"
  | "known"
  | "unknown"
  | "device-type-changed";

type GroupByKey = "none" | "device" | "event" | "category" | "device-type" | "ip" | "iface" | "day";

type EventFilterOption = {
  key: EventFilterKey;
  label: string;
  category?: ActivityCategory;
  eventTypes?: ActivityEventType[];
};

type EventSummaryCard = {
  key: EventFilterKey;
  label: string;
  value: number;
  detail: string;
  icon: string;
  tone: string;
};

type EventGroup = {
  key: string;
  label: string;
  events: HostEvent[];
};

const eventsPageSize = 100;

const emptyStats: ActivityStats = {
  Total: 0,
  Online: 0,
  Offline: 0,
  Discovered: 0,
  Known: 0,
  Unknown: 0,
  DeviceTypeChanged: 0,
};

const eventFilterOptions: EventFilterOption[] = [
  { key: "all", label: "All events" },
  { key: "connectivity", label: "Connectivity", category: "connectivity" },
  { key: "online", label: "Online", eventTypes: ["online"] },
  { key: "offline", label: "Offline", eventTypes: ["offline"] },
  { key: "changes", label: "Device changes", category: "changes" },
  { key: "discovered", label: "New device detected", eventTypes: ["discovered"] },
  { key: "recognition", label: "Recognition changes", eventTypes: ["known", "unknown"] },
  { key: "known", label: "Marked known", eventTypes: ["known"] },
  { key: "unknown", label: "Marked unknown", eventTypes: ["unknown"] },
  { key: "device-type-changed", label: "Device type changed", eventTypes: ["device-type-changed"] },
];

const groupByOptions: { key: GroupByKey; label: string }[] = [
  { key: "none", label: "None" },
  { key: "device", label: "Device" },
  { key: "event", label: "Event" },
  { key: "category", label: "Category" },
  { key: "device-type", label: "Device type" },
  { key: "ip", label: "IP" },
  { key: "iface", label: "Interface" },
  { key: "day", label: "Day" },
];

function Activity() {
  const [selectedMac, setSelectedMac] = createSignal("");
  const [eventFilter, setEventFilter] = createSignal<EventFilterKey>("all");
  const [groupBy, setGroupBy] = createSignal<GroupByKey>("none");
  const [events, setEvents] = createSignal<HostEvent[]>([]);
  const [stats, setStats] = createSignal<ActivityStats>(emptyStats);
  const [devices, setDevices] = createSignal<ActivityDeviceOption[]>([]);
  const [collapsedGroups, setCollapsedGroups] = createSignal<Record<string, boolean>>({});
  const [hasMore, setHasMore] = createSignal(false);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal("");
  const [statsError, setStatsError] = createSignal(false);
  let eventsRequest = 0;
  let statsRequest = 0;

  const currentEventOption = () => eventFilterOptions.find((option) => option.key === eventFilter()) ?? eventFilterOptions[0];
  const hostExists = (event: HostEvent) => bkpHosts().some((host) => host.ID === event.HostID && host.Mac === event.Mac);
  const filtersActive = () => selectedMac() !== "" || eventFilter() !== "all";

  const retentionText = () => {
    const hours = appConfig().ConnectivityRetention || appConfig().TrimHist;
    const retention = Number.isFinite(hours) && hours > 0
      ? hours + " " + (hours === 1 ? "hour" : "hours")
      : "the configured retention window";

    return "Connectivity events retained for " + retention + ". Device changes retained while the device exists.";
  };

  const loadEvents = async (reset: boolean) => {
    const activeRequest = ++eventsRequest;
    const option = currentEventOption();
    const mac = selectedMac();
    const offset = reset ? 0 : events().length;

    setLoading(true);
    setError("");
    if (reset) {
      setEvents([]);
      setHasMore(false);
      setCollapsedGroups({});
    }

    try {
      const nextEvents = await apiGetActivity(eventsPageSize, {
        category: option.category,
        eventTypes: option.eventTypes,
        macs: mac === "" ? undefined : [mac],
        offset,
      });
      if (activeRequest !== eventsRequest) {
        return;
      }

      setEvents((currentEvents) => reset ? nextEvents : [...currentEvents, ...nextEvents]);
      setHasMore(nextEvents.length === eventsPageSize);
    } catch {
      if (activeRequest !== eventsRequest) {
        return;
      }

      setError("Events could not be loaded");
      setHasMore(false);
    } finally {
      if (activeRequest === eventsRequest) {
        setLoading(false);
      }
    }
  };

  const loadStats = async () => {
    const activeRequest = ++statsRequest;
    const mac = selectedMac();
    setStatsError(false);

    try {
      const nextStats = await apiGetActivityStats({
        macs: mac === "" ? undefined : [mac],
      });
      if (activeRequest === statsRequest) {
        setStats(nextStats);
      }
    } catch {
      if (activeRequest === statsRequest) {
        setStats(emptyStats);
        setStatsError(true);
      }
    }
  };

  const loadDevices = async () => {
    try {
      setDevices(await apiGetActivityDevices());
    } catch {
      setDevices([]);
    }
  };

  createEffect(() => {
    selectedMac();
    eventFilter();
    loadEvents(true);
  });

  createEffect(() => {
    selectedMac();
    loadStats();
  });

  onMount(() => {
    loadDevices();
  });

  const summaryCards = createMemo<EventSummaryCard[]>(() => {
    const currentStats = stats();
    return [
      {
        key: "all",
        label: "Total events",
        value: currentStats.Total,
        detail: statsError() ? "Counts unavailable" : "All retained events",
        icon: "bi-collection-fill",
        tone: "total",
      },
      {
        key: "online",
        label: "Online",
        value: currentStats.Online,
        detail: "Connectivity",
        icon: "bi-check-circle-fill",
        tone: "online",
      },
      {
        key: "offline",
        label: "Offline",
        value: currentStats.Offline,
        detail: "Connectivity",
        icon: "bi-x-circle-fill",
        tone: "offline",
      },
      {
        key: "discovered",
        label: "New devices",
        value: currentStats.Discovered,
        detail: "Discovery",
        icon: "bi-plus-circle-fill",
        tone: "unknown",
      },
      {
        key: "recognition",
        label: "Recognition",
        value: currentStats.Known + currentStats.Unknown,
        detail: "Known and unknown",
        icon: "bi-bookmark-check-fill",
        tone: "known",
      },
      {
        key: "device-type-changed",
        label: "Type changes",
        value: currentStats.DeviceTypeChanged,
        detail: "Classification",
        icon: "bi-tag-fill",
        tone: "type",
      },
    ];
  });

  const groupedEvents = createMemo<EventGroup[]>(() => {
    if (groupBy() === "none") {
      return [];
    }

    const groups = new Map<string, EventGroup>();
    for (const event of events()) {
      const key = groupKey(event, groupBy());
      const label = groupLabel(event, groupBy());
      const existing = groups.get(key);
      if (existing) {
        existing.events.push(event);
      } else {
        groups.set(key, { key, label, events: [event] });
      }
    }

    return [...groups.values()];
  });

  const handleReset = () => {
    setSelectedMac("");
    setEventFilter("all");
  };

  const handleSummaryClick = (card: EventSummaryCard) => {
    setEventFilter(card.key);
  };

  const handleGroupToggle = (key: string) => {
    setCollapsedGroups((current) => ({
      ...current,
      [key]: !current[key],
    }));
  };

  return (
    <div class="activity-page">
      <header class="activity-page-header">
        <div>
          <h1 class="activity-page-title">Events</h1>
          <p class="activity-page-subtitle">Device changes and connectivity transitions</p>
        </div>
        <div class="activity-retention-note">{retentionText()}</div>
      </header>

      <section class="activity-summary-grid overview-grid" aria-label="Event overview">
        <For each={summaryCards()}>{(card) =>
          <button
            type="button"
            class={"overview-card overview-card-button overview-card-" + card.tone + (eventFilter() === card.key ? " is-active" : "")}
            aria-pressed={eventFilter() === card.key}
            onClick={[handleSummaryClick, card]}
          >
            <div class="overview-card-icon" aria-hidden="true">
              <i class={"bi " + card.icon}></i>
            </div>
            <div>
              <div class="overview-card-label">{card.label}</div>
              <div class="overview-card-value">{card.value}</div>
              <div class="overview-card-detail">{card.detail}</div>
            </div>
          </button>
        }</For>
      </section>

      <section class="card wyl-panel activity-filter-panel" aria-label="Event filters">
        <div class="card-body activity-filter-grid">
          <label class="activity-filter-field">
            <span class="activity-filter-label">Device</span>
            <select
              class={"form-select form-select-sm activity-filter-select" + (selectedMac() !== "" ? " is-active" : "")}
              value={selectedMac()}
              onChange={(event) => setSelectedMac(event.currentTarget.value)}
            >
              <option value="">All devices</option>
              <For each={devices()}>{(device) =>
                <option value={device.Mac}>{deviceOptionLabel(device)}</option>
              }</For>
            </select>
          </label>
          <label class="activity-filter-field">
            <span class="activity-filter-label">Event type</span>
            <select
              class={"form-select form-select-sm activity-filter-select" + (eventFilter() !== "all" ? " is-active" : "")}
              value={eventFilter()}
              onChange={(event) => setEventFilter(event.currentTarget.value as EventFilterKey)}
            >
              <For each={eventFilterOptions}>{(option) =>
                <option value={option.key}>{option.label}</option>
              }</For>
            </select>
          </label>
          <label class="activity-filter-field">
            <span class="activity-filter-label">Group by</span>
            <select
              class="form-select form-select-sm activity-filter-select"
              value={groupBy()}
              onChange={(event) => setGroupBy(event.currentTarget.value as GroupByKey)}
            >
              <For each={groupByOptions}>{(option) =>
                <option value={option.key}>{option.label}</option>
              }</For>
            </select>
          </label>
          <Show when={filtersActive()}>
            <button type="button" class="btn btn-sm device-reset-filter activity-filter-reset" onClick={handleReset} title="Reset event filters">
              <i class="bi bi-arrow-counterclockwise" aria-hidden="true"></i>
              <span>Reset filter</span>
            </button>
          </Show>
        </div>
      </section>

      <section class="card wyl-panel activity-table-panel" aria-labelledby="events-table-title">
        <div class="card-header activity-table-header">
          <div>
            <div id="events-table-title" class="activity-table-title">Events</div>
            <div class="activity-table-subtitle">{tableSubtitle(events().length, groupBy())}</div>
          </div>
        </div>
        <div class="card-body activity-table-body">
          <div class="table-responsive">
            <table class="table table-hover activity-table">
              <thead>
                <tr>
                  <th scope="col">Device</th>
                  <th scope="col">Event</th>
                  <th scope="col">Details</th>
                  <th scope="col">IP</th>
                  <th scope="col">Iface</th>
                  <th scope="col">Time</th>
                </tr>
              </thead>
              <tbody>
                <Show
                  when={groupBy() === "none"}
                  fallback={
                    <For each={groupedEvents()}>{(group) =>
                      <>
                        <tr class="activity-group-row">
                          <td colSpan={6} class="activity-group-cell" data-label="Group">
                            <button
                              type="button"
                              class="activity-group-toggle"
                              aria-expanded={!collapsedGroups()[group.key]}
                              onClick={[handleGroupToggle, group.key]}
                            >
                              <span class="activity-group-title">
                                <i class={"bi " + (collapsedGroups()[group.key] ? "bi-chevron-right" : "bi-chevron-down")} aria-hidden="true"></i>
                                <span>{group.label}</span>
                              </span>
                              <span class="activity-group-count">{group.events.length} loaded {group.events.length === 1 ? "event" : "events"}</span>
                            </button>
                          </td>
                        </tr>
                        <Show when={!collapsedGroups()[group.key]}>
                          <For each={group.events}>{(event) => eventRow(event, hostExists)}</For>
                        </Show>
                      </>
                    }</For>
                  }
                >
                  <For each={events()}>{(event) => eventRow(event, hostExists)}</For>
                </Show>
              </tbody>
            </table>
          </div>
          <Show when={!loading() && events().length === 0 && error() === ""}>
            <div class="activity-empty">No events match the current filters</div>
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
                onClick={() => loadEvents(false)}
              >
                <i class="bi bi-chevron-down" aria-hidden="true"></i>
                <span>{loading() ? "Loading" : "Load more"}</span>
              </button>
            </div>
          </Show>
        </div>
      </section>
    </div>
  );
}

function eventRow(event: HostEvent, hostExists: (event: HostEvent) => boolean) {
  const canLinkHost = () => event.HostID > 0 && hostExists(event);

  return (
    <tr class={"activity-table-row activity-row-" + activityTone(event.EventType)}>
      <td data-label="Device">
        <span class="activity-table-device">
          <span class="activity-host-icon" aria-hidden="true">
            <i class={"bi " + activityDeviceIcon(event)}></i>
          </span>
          <Show
            when={canLinkHost()}
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
      <td data-label="Details" class="activity-table-muted">{activityDetails(event) || " "}</td>
      <td data-label="IP" class="activity-table-muted">{event.IP || " "}</td>
      <td data-label="Iface" class="activity-table-muted">{event.Iface || " "}</td>
      <td data-label="Time">
        <time class="activity-time" dateTime={event.Date} title={event.Date}>
          {relativeActivityTime(event.Date)}
        </time>
      </td>
    </tr>
  );
}

function tableSubtitle(count: number, groupBy: GroupByKey) {
  const base = count + " loaded " + (count === 1 ? "event" : "events");
  return groupBy === "none" ? base : base + " grouped by " + groupByOptions.find((option) => option.key === groupBy)?.label;
}

function groupKey(event: HostEvent, groupBy: GroupByKey) {
  switch (groupBy) {
    case "device":
      return "device:" + (event.Mac || event.HostID || activityHostName(event));
    case "event":
      return "event:" + event.EventType;
    case "category":
      return "category:" + activityCategoryLabel(event.EventType);
    case "device-type":
      return "device-type:" + (event.DeviceType || "not-set");
    case "ip":
      return "ip:" + (event.IP || "none");
    case "iface":
      return "iface:" + (event.Iface || "none");
    case "day":
      return "day:" + event.Date.slice(0, 10);
    case "none":
    default:
      return "all";
  }
}

function groupLabel(event: HostEvent, groupBy: GroupByKey) {
  switch (groupBy) {
    case "device":
      return activityHostName(event);
    case "event":
      return activityEventLabel(event.EventType);
    case "category":
      return activityCategoryLabel(event.EventType);
    case "device-type":
      return activityDeviceTypeLabel(event.DeviceType);
    case "ip":
      return event.IP || "No IP";
    case "iface":
      return event.Iface || "No interface";
    case "day":
      return activityDayLabel(event.Date);
    case "none":
    default:
      return "Events";
  }
}

function deviceOptionLabel(device: ActivityDeviceOption) {
  const name = device.Name.trim() || device.Mac || "Unknown device";
  return name + (device.Exists ? "" : " (deleted)");
}

export default Activity;
