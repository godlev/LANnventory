import { createSignal, For, onMount, Show } from "solid-js";

import { allHosts, filterState, hasMultipleIfaces, hostsLoadError } from "../functions/exports";

import TableRow from "../components/Body/TableRow";
import TableHead from "../components/Body/TableHead";
import CardHead from "../components/Body/CardHead";
import SummaryCards from "../components/Body/SummaryCards";
import RecentActivityPanel from "../components/Body/RecentActivityPanel";
import { getHosts } from "../functions/atstart";
import { deviceTypeFilterLabel } from "../functions/deviceTypes";

function Body() {
  const [expandedDeviceRows, setExpandedDeviceRows] = createSignal<Record<string, boolean>>({});

  onMount(() => {
    getHosts();
  });

  const hostLabel = (count: number) => count === 1 ? "host" : "hosts";

  const currentSubtitle = () => {
    const filters = filterState();
    const count = allHosts.length;
    const states = [];

    if (filters.Known === 1) states.push("Known");
    if (filters.Known === 0) states.push("Unknown");
    if (filters.Now === 1) states.push("Online");
    if (filters.Now === 0) states.push("Offline");

    let text = states.length > 0
      ? `${count} ${states.join(" ")} ${hostLabel(count)}`
      : `${count} ${hostLabel(count)}`;

    if (filters.Search !== "") {
      text = states.length > 0
        ? `${text} matching search`
        : `${count} matching ${hostLabel(count)}`;
    }

    if (filters.Iface !== "") {
      text = `${text} on ${filters.Iface}`;
    }

    if (filters.DeviceType !== "") {
      text = `${text} - Type: ${deviceTypeFilterLabel(filters.DeviceType)}`;
    }

    return text;
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
  const shouldShowPinnedHeading = (index: number, host: { Pinned: boolean }) =>
    index === 0 && host.Pinned === true;
  const shouldShowOtherSeparator = (index: number, host: { Pinned: boolean }) =>
    index > 0
    && !host.Pinned
    && allHosts[index - 1]?.Pinned === true;
  const deviceTableColumnCount = () => hasMultipleIfaces() ? 11 : 10;

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
        <CardHead></CardHead>
      </div>
      <div class="card-body table-responsive device-table-wrap">
        <table class="table table-hover device-table">
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
