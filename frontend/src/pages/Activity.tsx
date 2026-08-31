import { A } from "@solidjs/router";
import { createEffect, createMemo, createSignal, For, onCleanup, onMount, Show } from "solid-js";

import {
  apiGetActivity,
  apiGetActivityDevices,
  apiGetActivityStats,
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

type GroupByKey = "device" | "event" | "category" | "device-type" | "ip" | "iface" | "day";
type DeviceDisplayMode = "name-icon" | "name" | "icon";
type DeviceSelectionMode = "all" | "custom" | "none";

type EventFilterOption = {
  key: EventFilterKey;
  label: string;
  eventTypes: ActivityEventType[];
};

type EventSummaryCard = {
  key: EventFilterKey;
  label: string;
  value: number;
  detail: string;
  icon: string;
  tone: string;
  eventTypes: ActivityEventType[];
};

type EventGroup = {
  pathKey: string;
  dimension: GroupByKey;
  dimensionLabel: string;
  label: string;
  eventsCount: number;
  children: EventGroup[];
  events: HostEvent[];
};

type GroupValue = {
  identity: string;
  label: string;
};

const eventsPageSize = 100;
const eventsDeviceDisplayStorageKey = "eventsDeviceDisplay";
const defaultDeviceDisplayMode: DeviceDisplayMode = "icon";
const eventTypeOrder: ActivityEventType[] = ["online", "offline", "discovered", "known", "unknown", "device-type-changed"];
const deviceDropdownId = "activity-device-filter";
const eventTypeDropdownId = "activity-event-type-filter";
const groupByDropdownId = "activity-group-by-filter";
const maxGroupHierarchySummaryLevels = 3;
const maxGroupIndentLevel = 5;

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
  { key: "all", label: "All events", eventTypes: eventTypeOrder },
  { key: "connectivity", label: "Connectivity", eventTypes: ["online", "offline"] },
  { key: "online", label: "Online", eventTypes: ["online"] },
  { key: "offline", label: "Offline", eventTypes: ["offline"] },
  { key: "changes", label: "Device changes", eventTypes: ["discovered", "known", "unknown", "device-type-changed"] },
  { key: "discovered", label: "New device detected", eventTypes: ["discovered"] },
  { key: "recognition", label: "Recognition changes", eventTypes: ["known", "unknown"] },
  { key: "known", label: "Marked known", eventTypes: ["known"] },
  { key: "unknown", label: "Marked unknown", eventTypes: ["unknown"] },
  { key: "device-type-changed", label: "Device type changed", eventTypes: ["device-type-changed"] },
];

const groupByOptions: { key: GroupByKey; label: string }[] = [
  { key: "device", label: "Device" },
  { key: "event", label: "Event" },
  { key: "category", label: "Category" },
  { key: "device-type", label: "Device type" },
  { key: "ip", label: "IP" },
  { key: "iface", label: "Interface" },
  { key: "day", label: "Day" },
];

const deviceDisplayOptions: { key: DeviceDisplayMode; label: string }[] = [
  { key: "name-icon", label: "Name + icon" },
  { key: "name", label: "Name only" },
  { key: "icon", label: "Icon only" },
];

function Activity() {
  const [deviceSelectionMode, setDeviceSelectionMode] = createSignal<DeviceSelectionMode>("all");
  const [selectedMacs, setSelectedMacs] = createSignal<string[]>([]);
  const [deviceSearch, setDeviceSearch] = createSignal("");
  const [selectedEventTypes, setSelectedEventTypes] = createSignal<ActivityEventType[]>(normalizeSelectedEventTypes(eventTypeOrder));
  const [groupByKeys, setGroupByKeys] = createSignal<GroupByKey[]>([]);
  const [deviceDisplayMode, setDeviceDisplayMode] = createSignal<DeviceDisplayMode>(defaultDeviceDisplayMode);
  const [events, setEvents] = createSignal<HostEvent[]>([]);
  const [stats, setStats] = createSignal<ActivityStats>(emptyStats);
  const [devices, setDevices] = createSignal<ActivityDeviceOption[]>([]);
  const [collapsedGroups, setCollapsedGroups] = createSignal<Record<string, boolean>>({});
  const [groupsCollapsedByDefault, setGroupsCollapsedByDefault] = createSignal(false);
  const [hasMore, setHasMore] = createSignal(false);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal("");
  const [statsError, setStatsError] = createSignal(false);
  const [deviceDropdownOpen, setDeviceDropdownOpen] = createSignal(false);
  const [eventTypeDropdownOpen, setEventTypeDropdownOpen] = createSignal(false);
  const [groupByDropdownOpen, setGroupByDropdownOpen] = createSignal(false);
  let eventsRequest = 0;
  let statsRequest = 0;
  let deviceTriggerRef: HTMLButtonElement | undefined;
  let deviceDropdownRef: HTMLDivElement | undefined;
  let eventTypeTriggerRef: HTMLButtonElement | undefined;
  let eventTypeDropdownRef: HTMLDivElement | undefined;
  let groupByTriggerRef: HTMLButtonElement | undefined;
  let groupByDropdownRef: HTMLDivElement | undefined;

  const selectedEventTypeSet = createMemo(() => new Set(selectedEventTypes()));
  const normalizedGroupByKeys = createMemo(() => normalizeGroupByKeys(groupByKeys()));
  const hostExists = (event: HostEvent) => bkpHosts().some((host) => host.ID === event.HostID && host.Mac === event.Mac);
  const allEventTypesSelected = () => selectedEventTypes().length === eventTypeOrder.length;
  const noEventTypesSelected = () => selectedEventTypes().length === 0;
  const eventTypeFilterActive = () => !allEventTypesSelected();
  const groupingActive = () => normalizedGroupByKeys().length > 0;
  const deviceFilterActive = () => deviceSelectionMode() !== "all";
  const noDevicesSelected = () => deviceSelectionMode() === "none";
  const normalizedSelectedMacs = createMemo(() => normalizeSelectedMacs(selectedMacs()));
  const selectedMacSet = createMemo(() => new Set(normalizedSelectedMacs()));
  const availableDeviceMacs = createMemo(() => normalizeSelectedMacs(devices().map((device) => device.Mac)));
  const deviceOptionsByMac = createMemo(() => {
    const options = new Map<string, ActivityDeviceOption>();
    for (const device of devices()) {
      if (device.Mac && !options.has(device.Mac)) {
        options.set(device.Mac, device);
      }
    }
    return options;
  });
  const filtersActive = () => deviceFilterActive() || eventTypeFilterActive();
  const deviceRequestMacs = () => deviceSelectionMode() === "custom" ? normalizedSelectedMacs() : undefined;
  const selectedDeviceLabel = () => {
    const macs = normalizedSelectedMacs();
    if (deviceSelectionMode() === "all") {
      return "All devices";
    }
    if (deviceSelectionMode() === "none" || macs.length === 0) {
      return "No devices";
    }
    if (macs.length === 1) {
      const device = deviceOptionsByMac().get(macs[0]);
      return device ? deviceOptionLabel(device) : macs[0];
    }
    return macs.length + " devices";
  };
  const deviceFilterTooltip = () => {
    const mode = deviceSelectionMode();
    const macs = normalizedSelectedMacs();
    if (mode === "all") {
      return "All devices included";
    }
    if (mode === "none" || macs.length === 0) {
      return "No devices selected";
    }

    const labels = macs.map((mac) => {
      const device = deviceOptionsByMac().get(mac);
      return device ? deviceOptionLabel(device) + " (" + mac + ")" : mac;
    });
    const visibleLabels = labels.slice(0, 8);
    const remainder = labels.length - visibleLabels.length;
    const suffix = remainder > 0 ? ", +" + remainder + " more" : "";

    return "Selected devices: " + visibleLabels.join(", ") + suffix;
  };
  const filteredDeviceOptions = createMemo(() => {
    const needle = deviceSearch().trim().toLowerCase();
    if (needle === "") {
      return devices();
    }

    return devices().filter((device) => deviceSearchText(device).includes(needle));
  });
  const eventTypeSummary = () => eventTypeClosedSummary(selectedEventTypes());
  const eventTypeTooltipSummary = () => eventTypeTooltip(selectedEventTypes());
  const groupBySummary = () => groupByClosedSummary(normalizedGroupByKeys());
  const groupByTooltipSummary = () => groupByTooltip(normalizedGroupByKeys());
  const tableStateSummary = createMemo(() => {
    const active: string[] = [];

    if (filtersActive()) {
      active.push("Filtered");
    }
    if (deviceFilterActive()) {
      active.push(selectedDeviceLabel());
    }
    if (eventTypeFilterActive()) {
      active.push(eventTypeSummary());
    }
    if (groupingActive()) {
      active.push("Grouped by " + groupByTableSummary(normalizedGroupByKeys()));
    }

    return active.join(" · ");
  });
  const tableStateTooltip = createMemo(() => {
    const activeFilters: string[] = [];

    if (deviceFilterActive()) {
      activeFilters.push("Device: " + deviceFilterTooltip());
    }
    if (eventTypeFilterActive()) {
      activeFilters.push(eventTypeTooltipSummary());
    }

    const groupedBy = groupingActive() ? "Grouped by " + groupByFullSummary(normalizedGroupByKeys()) : "";
    if (activeFilters.length > 0 && groupedBy) {
      return "Active filters - " + activeFilters.join(", ") + ". " + groupedBy + ".";
    }
    if (activeFilters.length > 0) {
      return "Active filters - " + activeFilters.join(", ");
    }

    return groupedBy;
  });
  const tableStateIcon = () => filtersActive() ? "bi-funnel-fill" : "bi-layers-fill";

  const retentionText = () => {
    const hours = appConfig().ConnectivityRetention || appConfig().TrimHist;
    const retention = Number.isFinite(hours) && hours > 0
      ? hours + " " + (hours === 1 ? "hour" : "hours")
      : "the configured retention window";

    return "Connectivity events retained for " + retention + ". Device changes retained while the device exists.";
  };

  const loadEvents = async (reset: boolean) => {
    const activeRequest = ++eventsRequest;
    const eventTypes = selectedEventTypes();
    const deviceMode = deviceSelectionMode();
    const macs = deviceRequestMacs();
    const offset = reset ? 0 : events().length;

    setError("");
    if (reset) {
      setEvents([]);
      setHasMore(false);
      setCollapsedGroups({});
    }

    if (eventTypes.length === 0) {
      setLoading(false);
      setHasMore(false);
      return;
    }
    if (deviceMode === "none") {
      setLoading(false);
      setHasMore(false);
      return;
    }

    setLoading(true);

    try {
      const nextEvents = await apiGetActivity(eventsPageSize, {
        eventTypes: eventTypes.length === eventTypeOrder.length ? undefined : eventTypes,
        macs,
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
    const deviceMode = deviceSelectionMode();
    const macs = deviceRequestMacs();
    setStatsError(false);

    if (deviceMode === "none") {
      setStats(emptyStats);
      return;
    }

    try {
      const nextStats = await apiGetActivityStats({
        macs,
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
    deviceSelectionMode();
    normalizedSelectedMacs();
    selectedEventTypes();
    loadEvents(true);
  });

  createEffect(() => {
    deviceSelectionMode();
    normalizedSelectedMacs();
    loadStats();
  });

  onMount(() => {
    setDeviceDisplayMode(readStoredEventsDeviceDisplay());
    loadDevices();

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (deviceDropdownOpen() && !deviceTriggerRef?.contains(target) && !deviceDropdownRef?.contains(target)) {
        setDeviceDropdownOpen(false);
      }
      if (eventTypeDropdownOpen() && !eventTypeTriggerRef?.contains(target) && !eventTypeDropdownRef?.contains(target)) {
        setEventTypeDropdownOpen(false);
      }
      if (groupByDropdownOpen() && !groupByTriggerRef?.contains(target) && !groupByDropdownRef?.contains(target)) {
        setGroupByDropdownOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") {
        return;
      }

      if (deviceDropdownOpen()) {
        event.preventDefault();
        setDeviceDropdownOpen(false);
        deviceTriggerRef?.focus();
      } else if (eventTypeDropdownOpen()) {
        event.preventDefault();
        setEventTypeDropdownOpen(false);
        eventTypeTriggerRef?.focus();
      } else if (groupByDropdownOpen()) {
        event.preventDefault();
        setGroupByDropdownOpen(false);
        groupByTriggerRef?.focus();
      }
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    onCleanup(() => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    });
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
        eventTypes: [],
      },
      {
        key: "online",
        label: "Online",
        value: currentStats.Online,
        detail: "Connectivity",
        icon: "bi-check-circle-fill",
        tone: "online",
        eventTypes: ["online"],
      },
      {
        key: "offline",
        label: "Offline",
        value: currentStats.Offline,
        detail: "Connectivity",
        icon: "bi-x-circle-fill",
        tone: "offline",
        eventTypes: ["offline"],
      },
      {
        key: "discovered",
        label: "New devices",
        value: currentStats.Discovered,
        detail: "Discovery",
        icon: "bi-plus-circle-fill",
        tone: "unknown",
        eventTypes: ["discovered"],
      },
      {
        key: "recognition",
        label: "Recognition",
        value: currentStats.Known + currentStats.Unknown,
        detail: "Known and unknown",
        icon: "bi-bookmark-check-fill",
        tone: "known",
        eventTypes: ["known", "unknown"],
      },
      {
        key: "device-type-changed",
        label: "Type changes",
        value: currentStats.DeviceTypeChanged,
        detail: "Classification",
        icon: "bi-tag-fill",
        tone: "type",
        eventTypes: ["device-type-changed"],
      },
    ];
  });

  const groupedEvents = createMemo<EventGroup[]>(() => {
    const keys = normalizedGroupByKeys();
    if (keys.length === 0) {
      return [];
    }

    return buildEventGroupTree(events(), keys);
  });
  const allGroupNodes = createMemo(() => flattenEventGroups(groupedEvents()));
  const isGroupCollapsed = (pathKey: string) => collapsedGroups()[pathKey] ?? groupsCollapsedByDefault();
  const hasExpandedGroups = () => groupingActive() && allGroupNodes().some((group) => !isGroupCollapsed(group.pathKey));
  const groupAllControlTitle = () => hasExpandedGroups() ? "Collapse all groups" : "Expand all groups";
  const groupAllControlIcon = () => hasExpandedGroups() ? "bi-arrows-collapse" : "bi-arrows-expand";
  const groupAllControlLabel = () => hasExpandedGroups() ? "Collapse all" : "Expand all";
  const isSummaryCardActive = (card: EventSummaryCard) => {
    if (card.key === "all") {
      return allEventTypesSelected();
    }
    if (allEventTypesSelected()) {
      return false;
    }

    const selected = selectedEventTypeSet();
    return card.eventTypes.length > 0 && card.eventTypes.every((eventType) => selected.has(eventType));
  };

  const handleReset = () => {
    setDeviceSelectionMode("all");
    setSelectedMacs([]);
    setDeviceSearch("");
    setSelectedEventTypes(normalizeSelectedEventTypes(eventTypeOrder));
  };

  const openDeviceDropdown = () => {
    const nextOpen = !deviceDropdownOpen();
    setDeviceDropdownOpen(nextOpen);
    if (nextOpen) {
      setEventTypeDropdownOpen(false);
      setGroupByDropdownOpen(false);
    }
  };

  const openEventTypeDropdown = () => {
    const nextOpen = !eventTypeDropdownOpen();
    setEventTypeDropdownOpen(nextOpen);
    if (nextOpen) {
      setDeviceDropdownOpen(false);
      setGroupByDropdownOpen(false);
    }
  };

  const openGroupByDropdown = () => {
    const nextOpen = !groupByDropdownOpen();
    setGroupByDropdownOpen(nextOpen);
    if (nextOpen) {
      setDeviceDropdownOpen(false);
      setEventTypeDropdownOpen(false);
    }
  };

  const handleDeviceToggle = (mac: string) => {
    if (deviceSelectionMode() === "all") {
      const nextMacs = availableDeviceMacs().filter((deviceMac) => deviceMac !== mac);
      setSelectedMacs(nextMacs);
      setDeviceSelectionMode(nextMacs.length > 0 ? "custom" : "none");
      return;
    }

    const selected = new Set(deviceSelectionMode() === "custom" ? normalizedSelectedMacs() : []);
    if (selected.has(mac)) {
      selected.delete(mac);
    } else {
      selected.add(mac);
    }

    const nextMacs = normalizeSelectedMacs([...selected]);
    setSelectedMacs(nextMacs);
    setDeviceSelectionMode(nextMacs.length > 0 ? "custom" : "none");
  };

  const handleDeviceSelectAll = () => {
    setDeviceSelectionMode("all");
    setSelectedMacs([]);
  };

  const handleDeviceClearAll = () => {
    setDeviceSelectionMode("none");
    setSelectedMacs([]);
  };

  const applyGroupByKeys = (keys: GroupByKey[]) => {
    const normalized = normalizeGroupByKeys(keys);
    if (sameGroupByKeys(normalizedGroupByKeys(), normalized)) {
      return;
    }

    setGroupByKeys(normalized);
    setGroupsCollapsedByDefault(false);
    setCollapsedGroups({});
  };

  const handleGroupByToggle = (key: GroupByKey) => {
    const keys = normalizedGroupByKeys();
    if (keys.includes(key)) {
      applyGroupByKeys(keys.filter((groupKey) => groupKey !== key));
      return;
    }

    applyGroupByKeys([...keys, key]);
  };

  const handleGroupByMove = (key: GroupByKey, direction: -1 | 1) => {
    const keys = [...normalizedGroupByKeys()];
    const index = keys.indexOf(key);
    const nextIndex = index + direction;
    if (index < 0 || nextIndex < 0 || nextIndex >= keys.length) {
      return;
    }

    [keys[index], keys[nextIndex]] = [keys[nextIndex], keys[index]];
    applyGroupByKeys(keys);
  };

  const handleGroupByClear = () => {
    applyGroupByKeys([]);
  };

  const handleSummaryClick = (event: MouseEvent, card: EventSummaryCard) => {
    if (card.key === "all") {
      setSelectedEventTypes(normalizeSelectedEventTypes(eventTypeOrder));
      return;
    }

    if (event.ctrlKey || event.metaKey) {
      const selected = new Set(selectedEventTypes());
      const allCardTypesSelected = card.eventTypes.every((eventType) => selected.has(eventType));

      for (const eventType of card.eventTypes) {
        if (allCardTypesSelected) {
          selected.delete(eventType);
        } else {
          selected.add(eventType);
        }
      }

      setSelectedEventTypes(normalizeSelectedEventTypes([...selected]));
      return;
    }

    setSelectedEventTypes(
      sameEventTypes(selectedEventTypes(), card.eventTypes)
        ? normalizeSelectedEventTypes(eventTypeOrder)
        : normalizeSelectedEventTypes(card.eventTypes),
    );
  };

  const handleGroupToggle = (key: string) => {
    setCollapsedGroups((current) => ({
      ...current,
      [key]: !isGroupCollapsed(key),
    }));
  };

  const handleGroupAllToggle = () => {
    const nextCollapsed = hasExpandedGroups();
    const nextGroups: Record<string, boolean> = {};

    for (const group of allGroupNodes()) {
      nextGroups[group.pathKey] = nextCollapsed;
    }

    setGroupsCollapsedByDefault(nextCollapsed);
    setCollapsedGroups(nextGroups);
  };

  const handleEventTypeToggle = (eventType: ActivityEventType) => {
    const selected = new Set(selectedEventTypes());
    if (selected.has(eventType)) {
      selected.delete(eventType);
    } else {
      selected.add(eventType);
    }

    setSelectedEventTypes(normalizeSelectedEventTypes([...selected]));
  };

  const handleEventTypeGroupToggle = (eventTypes: ActivityEventType[]) => {
    const selected = new Set(selectedEventTypes());
    const allSelected = eventTypes.every((eventType) => selected.has(eventType));

    for (const eventType of eventTypes) {
      if (allSelected) {
        selected.delete(eventType);
      } else {
        selected.add(eventType);
      }
    }

    setSelectedEventTypes(normalizeSelectedEventTypes([...selected]));
  };

  const handleEventTypeSelectAll = () => {
    setSelectedEventTypes(normalizeSelectedEventTypes(eventTypeOrder));
  };

  const handleEventTypeDeselectAll = () => {
    setSelectedEventTypes([]);
  };

  const eventTypeGroupState = (eventTypes: ActivityEventType[]) => {
    const selected = selectedEventTypeSet();
    const selectedCount = eventTypes.filter((eventType) => selected.has(eventType)).length;

    return {
      checked: selectedCount === eventTypes.length,
      indeterminate: selectedCount > 0 && selectedCount < eventTypes.length,
    };
  };

  const handleDeviceDisplayChange = (event: Event & { currentTarget: HTMLSelectElement }) => {
    const nextMode = normalizeDeviceDisplayMode(event.currentTarget.value);

    setDeviceDisplayMode(nextMode);
    persistEventsDeviceDisplay(nextMode);
  };

  return (
    <div class="activity-page">
      <header class="activity-page-header">
        <div>
          <h1 class="activity-page-title">Events</h1>
          <p class="activity-page-subtitle">Device changes and connectivity transitions</p>
        </div>
        <div class="activity-retention-note">
          <span>{retentionText()}</span>
          <A class="activity-retention-link" href="/config#data-retention">Retention settings</A>
        </div>
      </header>

      <section class="activity-summary-grid overview-grid" aria-label="Event overview">
        <For each={summaryCards()}>{(card) =>
          <button
            type="button"
            class={"overview-card overview-card-button overview-card-" + card.tone + (isSummaryCardActive(card) ? " is-active" : "")}
            aria-pressed={isSummaryCardActive(card)}
            onClick={(event) => handleSummaryClick(event, card)}
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
          <div class="activity-filter-field activity-multiselect-field activity-device-filter-field">
            <span class="activity-filter-label">Device</span>
            <button
              ref={deviceTriggerRef}
              type="button"
              class={"form-select form-select-sm activity-filter-select activity-multiselect-trigger" + (deviceFilterActive() ? " is-active" : "")}
              aria-expanded={deviceDropdownOpen() ? "true" : "false"}
              aria-controls={deviceDropdownId}
              aria-label={"Device filter: " + deviceFilterTooltip()}
              title={deviceFilterTooltip()}
              onClick={openDeviceDropdown}
            >
              <span>{selectedDeviceLabel()}</span>
            </button>
            <Show when={deviceDropdownOpen()}>
              <div
                ref={deviceDropdownRef}
                id={deviceDropdownId}
                class="activity-multiselect-panel activity-device-multiselect-panel"
                role="group"
                aria-label="Device filter options"
              >
                <label class="activity-multiselect-search-field">
                  <span class="visually-hidden">Search devices</span>
                  <input
                    type="search"
                    class="form-control form-control-sm activity-multiselect-search"
                    value={deviceSearch()}
                    placeholder="Search devices"
                    onInput={(event) => setDeviceSearch(event.currentTarget.value)}
                  />
                </label>
                <div class="activity-multiselect-actions">
                  <button type="button" class="btn btn-sm activity-multiselect-action" onClick={handleDeviceSelectAll}>
                    Select all
                  </button>
                  <button type="button" class="btn btn-sm activity-multiselect-action" onClick={handleDeviceClearAll}>
                    Clear all
                  </button>
                </div>
                <div class="activity-multiselect-options">
                  <Show
                    when={filteredDeviceOptions().length > 0}
                    fallback={<div class="activity-multiselect-empty">No matching devices</div>}
                  >
                    <For each={filteredDeviceOptions()}>{(device) =>
                      <DeviceCheckbox
                        device={device}
                        checked={deviceSelectionMode() === "all" || (deviceSelectionMode() === "custom" && selectedMacSet().has(device.Mac))}
                        onChange={() => handleDeviceToggle(device.Mac)}
                      />
                    }</For>
                  </Show>
                </div>
                <div class="activity-multiselect-footer">
                  <button type="button" class="btn btn-sm device-reset-filter" onClick={() => setDeviceDropdownOpen(false)}>
                    Done
                  </button>
                </div>
              </div>
            </Show>
          </div>
          <div class="activity-filter-field activity-multiselect-field">
            <span class="activity-filter-label">Event type</span>
            <button
              ref={eventTypeTriggerRef}
              type="button"
              class={"form-select form-select-sm activity-filter-select activity-multiselect-trigger" + (eventTypeFilterActive() ? " is-active" : "")}
              aria-expanded={eventTypeDropdownOpen() ? "true" : "false"}
              aria-controls={eventTypeDropdownId}
              aria-label={"Event type filter: " + eventTypeTooltipSummary()}
              title={eventTypeTooltipSummary()}
              onClick={openEventTypeDropdown}
            >
              <span>{eventTypeSummary()}</span>
            </button>
            <Show when={eventTypeDropdownOpen()}>
              <div
                ref={eventTypeDropdownRef}
                id={eventTypeDropdownId}
                class="activity-multiselect-panel"
                role="group"
                aria-label="Event type options"
              >
                <div class="activity-multiselect-actions">
                  <button type="button" class="btn btn-sm activity-multiselect-action" onClick={handleEventTypeSelectAll}>
                    Select all
                  </button>
                  <button type="button" class="btn btn-sm activity-multiselect-action" onClick={handleEventTypeDeselectAll}>
                    Deselect all
                  </button>
                </div>
                <div class="activity-multiselect-options">
                  <EventTypeCheckbox
                    label="Connectivity"
                    className="activity-multiselect-parent"
                    checked={eventTypeGroupState(["online", "offline"]).checked}
                    indeterminate={eventTypeGroupState(["online", "offline"]).indeterminate}
                    onChange={() => handleEventTypeGroupToggle(["online", "offline"])}
                  />
                  <EventTypeCheckbox
                    label="Online"
                    className="activity-multiselect-child"
                    checked={selectedEventTypeSet().has("online")}
                    onChange={() => handleEventTypeToggle("online")}
                  />
                  <EventTypeCheckbox
                    label="Offline"
                    className="activity-multiselect-child"
                    checked={selectedEventTypeSet().has("offline")}
                    onChange={() => handleEventTypeToggle("offline")}
                  />
                  <EventTypeCheckbox
                    label="Device changes"
                    className="activity-multiselect-parent"
                    checked={eventTypeGroupState(["discovered", "known", "unknown", "device-type-changed"]).checked}
                    indeterminate={eventTypeGroupState(["discovered", "known", "unknown", "device-type-changed"]).indeterminate}
                    onChange={() => handleEventTypeGroupToggle(["discovered", "known", "unknown", "device-type-changed"])}
                  />
                  <EventTypeCheckbox
                    label="New device detected"
                    className="activity-multiselect-child"
                    checked={selectedEventTypeSet().has("discovered")}
                    onChange={() => handleEventTypeToggle("discovered")}
                  />
                  <EventTypeCheckbox
                    label="Recognition changes"
                    className="activity-multiselect-child activity-multiselect-parent"
                    checked={eventTypeGroupState(["known", "unknown"]).checked}
                    indeterminate={eventTypeGroupState(["known", "unknown"]).indeterminate}
                    onChange={() => handleEventTypeGroupToggle(["known", "unknown"])}
                  />
                  <EventTypeCheckbox
                    label="Marked known"
                    className="activity-multiselect-grandchild"
                    checked={selectedEventTypeSet().has("known")}
                    onChange={() => handleEventTypeToggle("known")}
                  />
                  <EventTypeCheckbox
                    label="Marked unknown"
                    className="activity-multiselect-grandchild"
                    checked={selectedEventTypeSet().has("unknown")}
                    onChange={() => handleEventTypeToggle("unknown")}
                  />
                  <EventTypeCheckbox
                    label="Device type changed"
                    className="activity-multiselect-child"
                    checked={selectedEventTypeSet().has("device-type-changed")}
                    onChange={() => handleEventTypeToggle("device-type-changed")}
                  />
                </div>
                <div class="activity-multiselect-footer">
                  <button type="button" class="btn btn-sm device-reset-filter" onClick={() => setEventTypeDropdownOpen(false)}>
                    Done
                  </button>
                </div>
              </div>
            </Show>
          </div>
          <div class="activity-filter-field activity-multiselect-field activity-groupby-filter-field">
            <span class="activity-filter-label">Group by</span>
            <button
              ref={groupByTriggerRef}
              type="button"
              class={"form-select form-select-sm activity-filter-select activity-multiselect-trigger" + (groupingActive() ? " is-active" : "")}
              aria-expanded={groupByDropdownOpen() ? "true" : "false"}
              aria-controls={groupByDropdownId}
              aria-label={"Group by: " + groupByTooltipSummary()}
              title={groupByTooltipSummary()}
              onClick={openGroupByDropdown}
            >
              <span>{groupBySummary()}</span>
            </button>
            <Show when={groupByDropdownOpen()}>
              <div
                ref={groupByDropdownRef}
                id={groupByDropdownId}
                class="activity-multiselect-panel activity-groupby-panel"
                role="group"
                aria-label="Group by options"
              >
                <div class="activity-multiselect-options activity-groupby-options">
                  <For each={groupByOptions}>{(option) =>
                    <GroupByOptionControl
                      option={option}
                      selectedIndex={normalizedGroupByKeys().indexOf(option.key)}
                      selectedCount={normalizedGroupByKeys().length}
                      onToggle={() => handleGroupByToggle(option.key)}
                      onMoveUp={() => handleGroupByMove(option.key, -1)}
                      onMoveDown={() => handleGroupByMove(option.key, 1)}
                    />
                  }</For>
                </div>
                <div class="activity-multiselect-footer">
                  <button
                    type="button"
                    class="btn btn-sm activity-multiselect-action"
                    disabled={!groupingActive()}
                    onClick={handleGroupByClear}
                  >
                    Clear grouping
                  </button>
                  <button type="button" class="btn btn-sm device-reset-filter" onClick={() => setGroupByDropdownOpen(false)}>
                    Done
                  </button>
                </div>
              </div>
            </Show>
          </div>
          <label class="activity-filter-field activity-display-field">
            <span class="activity-filter-label">Device display</span>
            <select
              class="form-select form-select-sm activity-filter-select"
              value={deviceDisplayMode()}
              onChange={handleDeviceDisplayChange}
              title="Choose how the Events Device column is displayed"
            >
              <For each={deviceDisplayOptions}>{(option) =>
                <option value={option.key}>{option.label}</option>
              }</For>
            </select>
          </label>
          <Show when={groupingActive()}>
            <button
              type="button"
              class="btn btn-sm device-reset-filter activity-group-all-toggle"
              onClick={handleGroupAllToggle}
              title={groupAllControlTitle()}
              aria-label={groupAllControlTitle()}
            >
              <i class={"bi " + groupAllControlIcon()} aria-hidden="true"></i>
              <span>{groupAllControlLabel()}</span>
            </button>
          </Show>
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
          <div class="activity-table-title-group">
            <div class="activity-table-title-row">
              <div id="events-table-title" class="activity-table-title">Events</div>
              <Show when={tableStateSummary()}>
                <span
                  class="activity-state-indicator"
                  title={tableStateTooltip()}
                  aria-label={tableStateTooltip()}
                >
                  <i class={"bi " + tableStateIcon()} aria-hidden="true"></i>
                  <span>{tableStateSummary()}</span>
                </span>
              </Show>
            </div>
            <div class="activity-table-subtitle">{tableSubtitle(events().length, normalizedGroupByKeys())}</div>
          </div>
        </div>
        <div class="card-body activity-table-body">
          <div class="table-responsive">
            <table class={"table table-hover activity-table activity-table-device-display-" + deviceDisplayMode()}>
              <thead>
                <tr>
                  <th scope="col" class="activity-table-time-heading">
                    <span class="activity-heading-with-state">
                      <Show when={tableStateSummary()}>
                        <span
                          class="activity-sticky-state-indicator"
                          title={tableStateTooltip()}
                          aria-label={tableStateTooltip()}
                          role="img"
                        >
                          <i class={"bi " + tableStateIcon()} aria-hidden="true"></i>
                        </span>
                      </Show>
                      <span>Time</span>
                    </span>
                  </th>
                  <th scope="col" class="activity-table-device-heading">Device</th>
                  <th scope="col" class="activity-table-ip-heading">IP</th>
                  <th scope="col" class="activity-table-event-heading">Event</th>
                  <th scope="col" class="activity-table-iface-heading">Iface</th>
                  <th scope="col" class="activity-table-details-heading">Details</th>
                </tr>
              </thead>
              <tbody>
                <Show
                  when={!groupingActive()}
                  fallback={
                    <For each={groupedEvents()}>{(group) =>
                      <EventGroupRows
                        group={group}
                        level={0}
                        isGroupCollapsed={isGroupCollapsed}
                        onGroupToggle={handleGroupToggle}
                        hostExists={hostExists}
                        deviceDisplayMode={deviceDisplayMode}
                      />
                    }</For>
                  }
                >
                  <For each={events()}>{(event) => eventRow(event, hostExists, deviceDisplayMode)}</For>
                </Show>
              </tbody>
            </table>
          </div>
          <Show when={!loading() && events().length === 0 && error() === ""}>
            <div class="activity-empty">{noEventTypesSelected() ? "No event types selected" : noDevicesSelected() ? "No devices selected" : "No events match the current filters"}</div>
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

function EventGroupRows(props: {
  group: EventGroup;
  level: number;
  isGroupCollapsed: (pathKey: string) => boolean;
  onGroupToggle: (pathKey: string) => void;
  hostExists: (event: HostEvent) => boolean;
  deviceDisplayMode: () => DeviceDisplayMode;
}) {
  const collapsed = () => props.isGroupCollapsed(props.group.pathKey);
  const visualLevel = () => Math.min(props.level, maxGroupIndentLevel);
  const groupTitle = () => props.group.dimensionLabel + " - " + props.group.label;

  return (
    <>
      <tr class={"activity-group-row activity-group-row-level-" + visualLevel()} data-group-level={props.level}>
        <td colSpan={6} class="activity-group-cell" data-label="Group">
          <button
            type="button"
            class={"activity-group-toggle activity-group-toggle-level-" + visualLevel()}
            aria-expanded={!collapsed()}
            title={groupTitle()}
            onClick={() => props.onGroupToggle(props.group.pathKey)}
          >
            <span class="activity-group-title">
              <i class={"bi " + (collapsed() ? "bi-chevron-right" : "bi-chevron-down")} aria-hidden="true"></i>
              <span class="activity-group-dimension">{props.group.dimensionLabel}</span>
              <span class="activity-group-separator">·</span>
              <span class="activity-group-label">{props.group.label}</span>
            </span>
            <span class="activity-group-count">{props.group.eventsCount} loaded {props.group.eventsCount === 1 ? "event" : "events"}</span>
          </button>
        </td>
      </tr>
      <Show when={!collapsed()}>
        <Show
          when={props.group.children.length > 0}
          fallback={<For each={props.group.events}>{(event) => eventRow(event, props.hostExists, props.deviceDisplayMode)}</For>}
        >
          <For each={props.group.children}>{(child) =>
            <EventGroupRows
              group={child}
              level={props.level + 1}
              isGroupCollapsed={props.isGroupCollapsed}
              onGroupToggle={props.onGroupToggle}
              hostExists={props.hostExists}
              deviceDisplayMode={props.deviceDisplayMode}
            />
          }</For>
        </Show>
      </Show>
    </>
  );
}

function eventRow(event: HostEvent, hostExists: (event: HostEvent) => boolean, deviceDisplayMode: () => DeviceDisplayMode) {
  const [detailsExpanded, setDetailsExpanded] = createSignal(false);
  const canLinkHost = () => event.HostID > 0 && hostExists(event);
  const deviceName = () => activityHostName(event);
  const deviceTypeLabel = () => activityDeviceTypeLabel(event.DeviceType);
  const iconOnlyTitle = () => deviceName() + " \u00b7 " + deviceTypeLabel() + (canLinkHost() ? " \u00b7 open device details" : " \u00b7 Device no longer exists");
  const iconOnlyAria = () => canLinkHost()
    ? "Open " + deviceName() + " device details"
    : deviceName() + " \u00b7 " + deviceTypeLabel() + " \u00b7 Device no longer exists";
  const mobileToggleLabel = () => (detailsExpanded() ? "Hide" : "Show") + " event details for " + deviceName();
  const mobileDetailsText = () => eventTechnicalDetails(event).join(" \u00b7 ") || "No technical details recorded";
  const nameContent = () => (
    <Show
      when={canLinkHost()}
      fallback={<span class="activity-host-name">{deviceName()}</span>}
    >
      <A href={"/host/" + event.HostID} class="activity-host-link">{deviceName()}</A>
    </Show>
  );
  const deviceTypeIcon = () => (
    <span
      class="activity-host-icon"
      title={"Device type: " + deviceTypeLabel()}
      aria-label={"Device type: " + deviceTypeLabel()}
      role="img"
    >
      <i class={"bi " + activityDeviceIcon(event)} aria-hidden="true"></i>
    </span>
  );
  const iconOnlyContent = () => (
    <Show
      when={canLinkHost()}
      fallback={
        <span
          class="activity-host-icon"
          title={iconOnlyTitle()}
          aria-label={iconOnlyAria()}
          role="img"
        >
          <i class={"bi " + activityDeviceIcon(event)} aria-hidden="true"></i>
        </span>
      }
    >
      <A
        href={"/host/" + event.HostID}
        class="activity-host-icon activity-host-icon-link"
        title={iconOnlyTitle()}
        aria-label={iconOnlyAria()}
      >
        <i class={"bi " + activityDeviceIcon(event)} aria-hidden="true"></i>
      </A>
    </Show>
  );
  const mobileDeviceContent = () => (
    <Show
      when={canLinkHost()}
      fallback={<span class="activity-mobile-device-name">{deviceName()}</span>}
    >
      <A href={"/host/" + event.HostID} class="activity-mobile-device-link">{deviceName()}</A>
    </Show>
  );

  return (
    <tr class={"activity-table-row activity-row-" + activityTone(event.EventType)}>
      <td class="activity-table-mobile-cell">
        <div class="activity-mobile-event-row">
          <span class="activity-event-icon activity-mobile-event-icon" aria-hidden="true">
            <i class={"bi " + activityIcon(event.EventType)}></i>
          </span>
          <span class="activity-mobile-device">
            {mobileDeviceContent()}
          </span>
          <span class="activity-mobile-description">{activityDescription(event)}</span>
          <time class="activity-mobile-time activity-time" dateTime={event.Date} title={event.Date}>
            {relativeActivityTime(event.Date)}
          </time>
          <button
            type="button"
            class="activity-mobile-expand"
            aria-expanded={detailsExpanded() ? "true" : "false"}
            aria-label={mobileToggleLabel()}
            title={mobileToggleLabel()}
            onClick={() => setDetailsExpanded(!detailsExpanded())}
          >
            <i class={"bi " + (detailsExpanded() ? "bi-chevron-up" : "bi-chevron-down")} aria-hidden="true"></i>
          </button>
        </div>
        <Show when={detailsExpanded()}>
          <div class="activity-mobile-details">{mobileDetailsText()}</div>
        </Show>
      </td>
      <td data-label="Time" class="activity-table-time-cell">
        <time class="activity-time" dateTime={event.Date} title={event.Date}>
          {relativeActivityTime(event.Date)}
        </time>
      </td>
      <td data-label="Device" class="activity-table-device-cell">
        <span class={"activity-table-device activity-device-display-" + deviceDisplayMode()}>
          <Show
            when={deviceDisplayMode() === "icon"}
            fallback={
              <>
                {nameContent()}
                <Show when={deviceDisplayMode() === "name-icon"}>
                  {deviceTypeIcon()}
                </Show>
              </>
            }
          >
            {iconOnlyContent()}
          </Show>
        </span>
      </td>
      <td data-label="IP" class="activity-table-muted activity-table-ip-cell">{event.IP || " "}</td>
      <td data-label="Event" class="activity-table-event-cell">
        <span class="activity-table-event">
          <span class="activity-event-icon" aria-hidden="true">
            <i class={"bi " + activityIcon(event.EventType)}></i>
          </span>
          <span>{activityDescription(event)}</span>
        </span>
      </td>
      <td data-label="Iface" class="activity-table-muted activity-table-iface-cell">{event.Iface || " "}</td>
      <td data-label="Details" class="activity-table-muted activity-table-details-cell">{activityDetails(event) || " "}</td>
    </tr>
  );
}

function eventTechnicalDetails(event: HostEvent) {
  const parts: string[] = [];
  const ip = cleanEventValue(event.IP);
  const mac = cleanEventValue(event.Mac);
  const iface = cleanEventValue(event.Iface);
  const details = cleanEventValue(activityDetails(event));

  if (ip) {
    parts.push("IP " + ip);
  }
  if (mac) {
    parts.push("MAC " + mac);
  }
  if (iface) {
    parts.push(iface);
  }
  if (details && !isDuplicateNetworkDetail(details, ip, iface)) {
    parts.push(details);
  }

  return parts;
}

function isDuplicateNetworkDetail(details: string, ip: string, iface: string) {
  return details === ip
    || details === iface
    || (ip !== "" && iface !== "" && details === ip + " / " + iface);
}

function cleanEventValue(value: string | null | undefined) {
  return (value ?? "").trim();
}

function tableSubtitle(count: number, groupByKeys: GroupByKey[]) {
  const base = count + " loaded " + (count === 1 ? "event" : "events");
  return groupByKeys.length === 0 ? base : base + " grouped by " + groupByFullSummary(groupByKeys);
}

function buildEventGroupTree(
  sourceEvents: HostEvent[],
  groupByKeys: GroupByKey[],
  level = 0,
  parentPath: [GroupByKey, string][] = [],
): EventGroup[] {
  const dimension = groupByKeys[level];
  if (dimension === undefined) {
    return [];
  }

  const groups = new Map<string, { value: GroupValue; events: HostEvent[] }>();
  for (const event of sourceEvents) {
    const value = groupValue(event, dimension);
    const existing = groups.get(value.identity);
    if (existing) {
      existing.events.push(event);
    } else {
      groups.set(value.identity, { value, events: [event] });
    }
  }

  return [...groups.values()].map((group) => {
    const path = [...parentPath, [dimension, group.value.identity] as [GroupByKey, string]];
    const isLeaf = level === groupByKeys.length - 1;

    return {
      pathKey: JSON.stringify(path),
      dimension,
      dimensionLabel: groupByLabel(dimension),
      label: group.value.label,
      eventsCount: group.events.length,
      children: isLeaf ? [] : buildEventGroupTree(group.events, groupByKeys, level + 1, path),
      events: isLeaf ? group.events : [],
    };
  });
}

function flattenEventGroups(groups: EventGroup[]) {
  const flattened: EventGroup[] = [];
  const visit = (nodes: EventGroup[]) => {
    for (const node of nodes) {
      flattened.push(node);
      visit(node.children);
    }
  };

  visit(groups);
  return flattened;
}

function groupValue(event: HostEvent, groupBy: GroupByKey): GroupValue {
  switch (groupBy) {
    case "device":
      return {
        identity: event.Mac
          ? "mac:" + event.Mac
          : event.HostID > 0
            ? "host:" + event.HostID
            : "name:" + activityHostName(event),
        label: activityHostName(event),
      };
    case "event":
      return {
        identity: "event:" + event.EventType,
        label: activityEventLabel(event.EventType),
      };
    case "category":
      return {
        identity: "category:" + activityCategoryKey(event.EventType),
        label: activityCategoryLabel(event.EventType),
      };
    case "device-type":
      return {
        identity: "device-type:" + (cleanEventValue(event.DeviceType) || "not-set"),
        label: activityDeviceTypeLabel(event.DeviceType),
      };
    case "ip":
      return {
        identity: cleanEventValue(event.IP) ? "ip:" + cleanEventValue(event.IP) : "missing-ip",
        label: cleanEventValue(event.IP) || "No IP",
      };
    case "iface":
      return {
        identity: cleanEventValue(event.Iface) ? "iface:" + cleanEventValue(event.Iface) : "missing-iface",
        label: cleanEventValue(event.Iface) || "No interface",
      };
    case "day":
      return {
        identity: activityDayKey(event.Date),
        label: activityDayLabel(event.Date),
      };
    default:
      return {
        identity: "unknown",
        label: "Unknown",
      };
  }
}

function activityCategoryKey(eventType: string) {
  return eventType === "online" || eventType === "offline" ? "connectivity" : "changes";
}

function activityDayKey(value: string) {
  return value.match(/^(\d{4}-\d{2}-\d{2})/)?.[1] ?? "missing-day";
}

function groupByLabel(key: GroupByKey) {
  return groupByOptions.find((option) => option.key === key)?.label ?? key;
}

function groupByFullSummary(keys: GroupByKey[]) {
  return keys.map(groupByLabel).join(" → ");
}

function groupByClosedSummary(keys: GroupByKey[]) {
  if (keys.length === 0) {
    return "None";
  }
  if (keys.length > maxGroupHierarchySummaryLevels) {
    return keys.length + " levels";
  }

  return groupByFullSummary(keys);
}

function groupByTableSummary(keys: GroupByKey[]) {
  return groupByClosedSummary(keys);
}

function groupByTooltip(keys: GroupByKey[]) {
  return keys.length === 0 ? "No grouping" : "Grouped by " + groupByFullSummary(keys);
}

function normalizeGroupByKeys(keys: GroupByKey[]) {
  const seen = new Set<GroupByKey>();
  const normalized: GroupByKey[] = [];

  for (const key of keys) {
    if (!groupByOptions.some((option) => option.key === key) || seen.has(key)) {
      continue;
    }
    seen.add(key);
    normalized.push(key);
  }

  return normalized;
}

function sameGroupByKeys(left: GroupByKey[], right: GroupByKey[]) {
  if (left.length !== right.length) {
    return false;
  }

  return left.every((key, index) => key === right[index]);
}

function deviceOptionLabel(device: ActivityDeviceOption) {
  const name = device.Name.trim() || device.Mac || "Unknown device";
  return name + (device.Exists ? "" : " (deleted)");
}

function deviceOptionMeta(device: ActivityDeviceOption) {
  return [device.IP, device.Mac].filter(Boolean).join(" · ");
}

function deviceSearchText(device: ActivityDeviceOption) {
  return [
    deviceOptionLabel(device),
    device.Mac,
    device.IP,
  ].join(" ").toLowerCase();
}

function normalizeSelectedMacs(macs: string[]) {
  const seen = new Set<string>();
  const normalized: string[] = [];

  for (const mac of macs) {
    const value = mac.trim();
    if (value === "" || seen.has(value)) {
      continue;
    }
    seen.add(value);
    normalized.push(value);
  }

  return normalized;
}

function GroupByOptionControl(props: {
  option: { key: GroupByKey; label: string };
  selectedIndex: number;
  selectedCount: number;
  onToggle: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
}) {
  const selected = () => props.selectedIndex >= 0;
  const order = () => props.selectedIndex + 1;
  const checkboxLabel = () => selected()
    ? "Remove " + props.option.label + " from grouping level " + order()
    : "Add " + props.option.label + " to grouping";

  return (
    <div class={"activity-groupby-option-row" + (selected() ? " is-selected" : "")}>
      <label class="activity-multiselect-option activity-groupby-option" title={checkboxLabel()}>
        <input
          type="checkbox"
          checked={selected()}
          aria-label={checkboxLabel()}
          onChange={props.onToggle}
        />
        <span class="activity-groupby-option-main">
          <Show when={selected()}>
            <span class="activity-groupby-order" aria-hidden="true">{order()}</span>
          </Show>
          <span class="activity-groupby-label">{props.option.label}</span>
        </span>
      </label>
      <Show when={selected()}>
        <span class="activity-groupby-move-controls" aria-label={props.option.label + " grouping order controls"}>
          <button
            type="button"
            class="btn btn-sm activity-groupby-move"
            disabled={props.selectedIndex === 0}
            title={"Move " + props.option.label + " up"}
            aria-label={"Move " + props.option.label + " up"}
            onClick={props.onMoveUp}
          >
            <i class="bi bi-chevron-up" aria-hidden="true"></i>
          </button>
          <button
            type="button"
            class="btn btn-sm activity-groupby-move"
            disabled={props.selectedIndex === props.selectedCount - 1}
            title={"Move " + props.option.label + " down"}
            aria-label={"Move " + props.option.label + " down"}
            onClick={props.onMoveDown}
          >
            <i class="bi bi-chevron-down" aria-hidden="true"></i>
          </button>
        </span>
      </Show>
    </div>
  );
}

function DeviceCheckbox(props: {
  device: ActivityDeviceOption;
  checked: boolean;
  onChange: () => void;
}) {
  const label = () => deviceOptionLabel(props.device);
  const meta = () => deviceOptionMeta(props.device);
  const title = () => meta() ? label() + " - " + meta() : label();

  return (
    <label class={"activity-multiselect-option activity-device-option" + (props.device.Exists ? "" : " activity-device-option-deleted")} title={title()}>
      <input
        type="checkbox"
        checked={props.checked}
        aria-label={title()}
        onChange={props.onChange}
      />
      <span class="activity-device-option-text">
        <span class="activity-device-option-name">{label()}</span>
        <Show when={meta()}>
          <span class="activity-device-option-meta">{meta()}</span>
        </Show>
      </span>
    </label>
  );
}

function EventTypeCheckbox(props: {
  label: string;
  checked: boolean;
  indeterminate?: boolean;
  className?: string;
  onChange: () => void;
}) {
  let checkboxRef: HTMLInputElement | undefined;

  createEffect(() => {
    if (checkboxRef) {
      checkboxRef.indeterminate = props.indeterminate ?? false;
    }
  });

  return (
    <label class={"activity-multiselect-option " + (props.className ?? "")}>
      <input
        ref={checkboxRef}
        type="checkbox"
        checked={props.checked}
        aria-checked={props.indeterminate ? "mixed" : props.checked ? "true" : "false"}
        aria-label={props.label}
        onChange={props.onChange}
      />
      <span>{props.label}</span>
    </label>
  );
}

function eventTypeClosedSummary(eventTypes: ActivityEventType[]) {
  if (eventTypes.length === 0) {
    return "No event types";
  }

  const option = eventFilterOptionForTypes(eventTypes);
  if (option) {
    return option.label;
  }

  return eventTypes.length + " of " + eventTypeOrder.length + " event types";
}

function eventTypeTooltip(eventTypes: ActivityEventType[]) {
  if (eventTypes.length === 0) {
    return "No event types selected";
  }
  if (eventTypes.length === eventTypeOrder.length) {
    return "All event types selected";
  }

  return "Selected event types: " + eventTypes.map(activityEventLabel).join(", ");
}

function normalizeSelectedEventTypes(eventTypes: ActivityEventType[]): ActivityEventType[] {
  const selected = new Set(eventTypes);
  return eventTypeOrder.filter((eventType) => selected.has(eventType));
}

function sameEventTypes(left: ActivityEventType[], right: ActivityEventType[]) {
  const normalizedLeft = normalizeSelectedEventTypes(left);
  const normalizedRight = normalizeSelectedEventTypes(right);

  if (normalizedLeft.length !== normalizedRight.length) {
    return false;
  }

  return normalizedLeft.every((eventType, index) => eventType === normalizedRight[index]);
}

function eventFilterOptionForTypes(eventTypes: ActivityEventType[]) {
  return eventFilterOptions.find((option) => sameEventTypes(eventTypes, option.eventTypes));
}

function normalizeDeviceDisplayMode(value: unknown): DeviceDisplayMode {
  return deviceDisplayOptions.some((option) => option.key === value)
    ? value as DeviceDisplayMode
    : defaultDeviceDisplayMode;
}

function readStoredEventsDeviceDisplay(): DeviceDisplayMode {
  try {
    return normalizeDeviceDisplayMode(localStorage.getItem(eventsDeviceDisplayStorageKey));
  } catch {
    return defaultDeviceDisplayMode;
  }
}

function persistEventsDeviceDisplay(value: DeviceDisplayMode) {
  try {
    localStorage.setItem(eventsDeviceDisplayStorageKey, value);
  } catch {
    // Display preference persistence is optional; rendering should continue if storage is unavailable.
  }
}

export default Activity;
