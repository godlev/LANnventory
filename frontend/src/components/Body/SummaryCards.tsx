import { createMemo, For } from "solid-js";
import { bkpHosts, filterState, setHistUpdOnFilter } from "../../functions/exports";
import { resetFilters, toggleHostFilter } from "../../functions/filter";
import { filterHosts, hasActiveHostFilters } from "../../functions/hostView";

type SummaryItem = {
  label: string;
  shortLabel: string;
  value: number;
  detail: string;
  icon: string;
  tone: string;
  filterField?: "Known" | "Now";
  filterValue?: number;
  clearsFilters?: boolean;
};

function SummaryCards() {
  const summary = createMemo<SummaryItem[]>(() => {
    const hosts = bkpHosts();
    const filters = filterState();
    const total = hosts.length;
    const filteredHosts = filterHosts(hosts, filters);
    const statusFacetHosts = filterHosts(hosts, filters, { ignore: ["Now"] });
    const knownFacetHosts = filterHosts(hosts, filters, { ignore: ["Known"] });
    const filtersActive = hasActiveHostFilters(filters);

    const visible = filteredHosts.length;
    const online = statusFacetHosts.filter((host) => host.Now === 1).length;
    const offline = statusFacetHosts.filter((host) => host.Now === 0).length;
    const known = knownFacetHosts.filter((host) => host.Known === 1).length;
    const unknown = knownFacetHosts.filter((host) => host.Known === 0).length;

    const percentage = (value: number, base: number) => base > 0
      ? Math.round((value / base) * 100) + "%"
      : "0%";
    const facetDetail = (value: number, base: number) => {
      const context = filtersActive ? " of matching" : " of devices";
      return percentage(value, base) + context;
    };

    return [
      {
        label: "Total Devices",
        shortLabel: "ALL",
        value: filtersActive ? visible : total,
        detail: filtersActive
          ? visible + " visible / " + total + " total"
          : total === 1 ? "1 loaded host" : total + " loaded hosts",
        icon: "bi-hdd-network",
        tone: "total",
        clearsFilters: true,
      },
      {
        label: "Online",
        shortLabel: "ON",
        value: online,
        detail: facetDetail(online, statusFacetHosts.length),
        icon: "bi-check-circle-fill",
        tone: "online",
        filterField: "Now",
        filterValue: 1,
      },
      {
        label: "Offline",
        shortLabel: "OFF",
        value: offline,
        detail: facetDetail(offline, statusFacetHosts.length),
        icon: "bi-slash-circle-fill",
        tone: "offline",
        filterField: "Now",
        filterValue: 0,
      },
      {
        label: "Known",
        shortLabel: "KNOWN",
        value: known,
        detail: facetDetail(known, knownFacetHosts.length),
        icon: "bi-bookmark-check-fill",
        tone: "known",
        filterField: "Known",
        filterValue: 1,
      },
      {
        label: "Unknown",
        shortLabel: "UNK",
        value: unknown,
        detail: facetDetail(unknown, knownFacetHosts.length),
        icon: "bi-question-circle-fill",
        tone: "unknown",
        filterField: "Known",
        filterValue: 0,
      },
    ];
  });

  const isActive = (item: SummaryItem) => {
    if (item.clearsFilters) {
      return !hasActiveHostFilters(filterState());
    }

    if (!item.filterField || item.filterValue === undefined) {
      return false;
    }
    return filterState()[item.filterField] === item.filterValue;
  };

  const handleQuickFilter = (item: SummaryItem) => {
    if (item.clearsFilters) {
      resetFilters();
      setHistUpdOnFilter(true);
      return;
    }

    if (!item.filterField || item.filterValue === undefined) {
      return;
    }
    toggleHostFilter(item.filterField, item.filterValue);
    setHistUpdOnFilter(true);
  };

  const cardContent = (item: SummaryItem) => (
    <>
      <div class="overview-card-icon" aria-hidden="true">
        <i class={"bi " + item.icon}></i>
      </div>
      <div>
        <div class="overview-card-label">
          <span class="overview-card-label-full">{item.label}</span>
          <span class="overview-card-label-short" aria-hidden="true">{item.shortLabel}</span>
        </div>
        <div class="overview-card-value">{item.value}</div>
        <div class="overview-card-detail">{item.detail}</div>
      </div>
    </>
  );

  return (
    <section class="overview-grid home-overview-grid" aria-label="Device overview">
      <For each={summary()}>{(item) =>
        item.filterField || item.clearsFilters
          ? <button
              type="button"
              class={"overview-card overview-card-button overview-card-" + item.tone + (isActive(item) ? " is-active" : "")}
              title={item.label + ": " + item.value + ". " + item.detail}
              aria-label={item.label + ": " + item.value + ". " + item.detail}
              aria-pressed={isActive(item)}
              onClick={[handleQuickFilter, item]}
            >
              {cardContent(item)}
            </button>
          : <article class={"overview-card overview-card-" + item.tone}>
              {cardContent(item)}
            </article>
      }</For>
    </section>
  );
}

export default SummaryCards;
