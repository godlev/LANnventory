import { createMemo, createSignal, For, onMount, Show } from "solid-js";

import { allHosts, bkpHosts, filterState, hasMultipleIfaces, hostsLoadError } from "../functions/exports";

import TableRow from "../components/Body/TableRow";
import TableHead from "../components/Body/TableHead";
import CardHead from "../components/Body/CardHead";
import SummaryCards from "../components/Body/SummaryCards";
import RecentActivityPanel from "../components/Body/RecentActivityPanel";
import { getHosts } from "../functions/atstart";
import { deviceTypeFilterLabel } from "../functions/deviceTypes";
import { apiGetInfrastructureWorkloadMemberships, type InfrastructureWorkloadMembership } from "../functions/api";

type HomeTableView = "comfortable" | "compact";

const homeTableViewStorageKey = "lannventory.home.tableView";

function Body() {
  const [expandedDeviceRows, setExpandedDeviceRows] = createSignal<Record<string, boolean>>({});
  const [workloadMemberships, setWorkloadMemberships] = createSignal<InfrastructureWorkloadMembership[]>([]);
  const [tableView, setTableView] = createSignal<HomeTableView>("comfortable");

  onMount(() => {
    try {
      const savedView = window.localStorage.getItem(homeTableViewStorageKey);
      if (savedView === "comfortable" || savedView === "compact") {
        setTableView(savedView);
      }
    } catch {
      // Browser storage is optional; Comfortable remains the safe default.
    }

    getHosts();
    void apiGetInfrastructureWorkloadMemberships()
      .then((items) => setWorkloadMemberships(items ?? []))
      .catch(() => setWorkloadMemberships([]));
  });

  const membershipsByHost = createMemo(() => {
    const result = new Map<number, InfrastructureWorkloadMembership[]>();
    for (const item of workloadMemberships()) {
      if (item.retiredAt) continue;
      const current = result.get(item.hostId) ?? [];
      current.push(item);
      result.set(item.hostId, current);
    }
    return result;
  });

  const setHomeTableView = (next: HomeTableView) => {
    setTableView(next);
    try {
      window.localStorage.setItem(homeTableViewStorageKey, next);
    } catch {
      // Keep the in-session view even when browser storage is unavailable.
    }
  };

  const deviceLabel = (count: number) => count === 1 ? "device" : "devices";

  const currentSubtitle = () => {
    const filters = filterState();
    const count = allHosts.length;
    const total = bkpHosts().length;
    const context: string[] = [];

    if (filters.Known === 1) context.push("Known");
    if (filters.Known === 0) context.push("Unknown");
    if (filters.Now === 1) context.push("Online");
    if (filters.Now === 0) context.push("Offline");
    if (filters.Search !== "") context.push("matching search");
    if (filters.Iface !== "") context.push("Iface: " + filters.Iface);
    if (filters.DeviceType !== "") context.push("Type: " + deviceTypeFilterLabel(filters.DeviceType));

    const base = count.toLocaleString()
      + " " + deviceLabel(count)
      + " out of "
      + total.toLocaleString()
      + " total "
      + deviceLabel(total);

    return context.length > 0 ? base + " · " + context.join(" · ") : base;
  };

  const deviceExpansionKey = (host: { ID: number; Mac: string }) => host.Mac || "id:" + host.ID;
  const isDeviceExpanded = (host: { ID: number; Mac: string }) => expandedDeviceRows()[deviceExpansionKey(host)] === true;
  const toggleDeviceExpanded = (host: { ID: number; Mac: string }) => {
    const key = deviceExpansionKey(host);
    setExpandedDeviceRows((current) => ({
      ...current,
      [key]: current[key] !== true,
    }));
  };
  const hasPinnedAndOtherDevices = () =>
    allHosts.some((host) => host.Pinned) && allHosts.some((host) => !host.Pinned);
  const shouldShowPinnedHeading = (index: number, host: { Pinned: boolean }) =>
    hasPinnedAndOtherDevices()
    && index === 0
    && host.Pinned === true;
  const shouldShowOtherSeparator = (index: number, host: { Pinned: boolean }) =>
    hasPinnedAndOtherDevices()
    && index > 0
    && !host.Pinned
    && allHosts[index - 1]?.Pinned === true;
  const deviceTableColumnCount = () => hasMultipleIfaces() ? 10 : 9;

  return (
    <>
    <Show when={hostsLoadError()}>
      <div class="data-load-warning" role="status">
        <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
        <span>{hostsLoadError()}</span>
      </div>
    </Show>
    <SummaryCards></SummaryCards>
    <RecentActivityPanel></RecentActivityPanel>
    <div class="card device-panel">
      <div class="card-header device-panel-header">
        <div class="device-panel-title-group">
          <div class="device-panel-title">Devices</div>
          <div class="device-panel-subtitle">{currentSubtitle()}</div>
        </div>
        <CardHead viewMode={tableView()} onViewModeChange={setHomeTableView}></CardHead>
      </div>
      <div class="card-body table-responsive device-table-wrap">
        <table class={"table table-hover device-table device-table-" + tableView()}>
          <TableHead></TableHead>
          <tbody>
            <For each={allHosts}>{(host, index) =>
            <>
              <Show when={shouldShowPinnedHeading(index(), host)}>
                <tr class="device-pinned-separator-row">
                  <td colSpan={deviceTableColumnCount()}>
                    <span class="device-pinned-separator">Pinned devices</span>
                  </td>
                </tr>
              </Show>
              <Show when={shouldShowOtherSeparator(index(), host)}>
                <tr class="device-pinned-separator-row">
                  <td colSpan={deviceTableColumnCount()}>
                    <span class="device-pinned-separator">Other devices</span>
                  </td>
                </tr>
              </Show>
              <TableRow
                host={host}
                index={index() + 1}
                mobileExpanded={isDeviceExpanded(host)}
                onToggleMobileExpanded={() => toggleDeviceExpanded(host)}
                workloadMemberships={membershipsByHost().get(host.ID) ?? []}
                viewMode={tableView()}
              ></TableRow>
            </>
            }</For>
          </tbody> 
        </table>
      </div>
    </div>
    </>
  )
}

export default Body
