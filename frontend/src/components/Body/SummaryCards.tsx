import { createMemo } from "solid-js";
import { bkpHosts, filterState, setHistUpdOnFilter } from "../../functions/exports";
import { resetFilters, toggleHostFilter } from "../../functions/filter";
import { filterHosts, hasActiveHostFilters } from "../../functions/hostView";

type SplitSummaryItem = {
  label: string;
  value: number;
  percent: number;
  detail: string;
  icon: string;
  tone: string;
  filterField: "Known" | "Now";
  filterValue: number;
};

type CombinedSummary = {
  label: string;
  shortLabel: string;
  detail: string;
  icon: string;
  tone: string;
  primary: SplitSummaryItem;
  secondary: SplitSummaryItem;
};

function SummaryCards() {
  const summary = createMemo(() => {
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
      ? Math.round((value / base) * 100)
      : 0;
    const facetDetail = (value: number, base: number) => {
      const context = filtersActive ? " of matching" : " of devices";
      return percentage(value, base) + "%" + context;
    };

    const makeSplit = (
      label: string,
      value: number,
      base: number,
      icon: string,
      tone: string,
      filterField: "Known" | "Now",
      filterValue: number,
    ): SplitSummaryItem => ({
      label,
      value,
      percent: percentage(value, base),
      detail: facetDetail(value, base),
      icon,
      tone,
      filterField,
      filterValue,
    });

    return {
      total: {
        label: "Total Devices",
        shortLabel: "ALL",
        value: filtersActive ? visible : total,
        detail: filtersActive
          ? visible + " visible / " + total + " total"
          : total === 1 ? "1 loaded host" : total + " loaded hosts",
        icon: "bi-hdd-network",
      },
      combined: [
        {
          label: "Connectivity",
          shortLabel: "NET",
          detail: statusFacetHosts.length + " scoped devices",
          icon: "bi-broadcast-pin",
          tone: "connectivity",
          primary: makeSplit("Online", online, statusFacetHosts.length, "bi-check-circle-fill", "online", "Now", 1),
          secondary: makeSplit("Offline", offline, statusFacetHosts.length, "bi-slash-circle-fill", "offline", "Now", 0),
        },
        {
          label: "Recognition",
          shortLabel: "ID",
          detail: knownFacetHosts.length + " scoped devices",
          icon: "bi-bookmarks-fill",
          tone: "recognition",
          primary: makeSplit("Known", known, knownFacetHosts.length, "bi-bookmark-check-fill", "known", "Known", 1),
          secondary: makeSplit("Unknown", unknown, knownFacetHosts.length, "bi-question-circle-fill", "unknown", "Known", 0),
        },
      ] satisfies CombinedSummary[],
    };
  });

  const isSplitActive = (item: SplitSummaryItem) => filterState()[item.filterField] === item.filterValue;

  const handleTotal = () => {
    resetFilters();
    setHistUpdOnFilter(true);
  };

  const handleSplitFilter = (item: SplitSummaryItem) => {
    toggleHostFilter(item.filterField, item.filterValue);
    setHistUpdOnFilter(true);
  };

  const totalActive = () => !hasActiveHostFilters(filterState());

  return (
    <section class="overview-grid home-overview-grid" aria-label="Device overview">
      <button
        type="button"
        class={"overview-card overview-card-button overview-card-total" + (totalActive() ? " is-active" : "")}
        title={summary().total.label + ": " + summary().total.value + ". " + summary().total.detail}
        aria-label={summary().total.label + ": " + summary().total.value + ". " + summary().total.detail}
        aria-pressed={totalActive()}
        onClick={handleTotal}
      >
        <div class="overview-card-icon" aria-hidden="true">
          <i class={"bi " + summary().total.icon}></i>
        </div>
        <div>
          <div class="overview-card-label">
            <span class="overview-card-label-full">{summary().total.label}</span>
            <span class="overview-card-label-short" aria-hidden="true">{summary().total.shortLabel}</span>
          </div>
          <div class="overview-card-value">{summary().total.value}</div>
          <div class="overview-card-detail">{summary().total.detail}</div>
        </div>
      </button>
      {summary().combined.map((card) =>
        <article class={"overview-card overview-card-split overview-card-" + card.tone}>
          <div class="overview-card-icon" aria-hidden="true">
            <i class={"bi " + card.icon}></i>
          </div>
          <div class="overview-split-content">
            <div class="overview-card-label">
              <span class="overview-card-label-full">{card.label}</span>
              <span class="overview-card-label-short" aria-hidden="true">{card.shortLabel}</span>
            </div>
            <div class="overview-split-actions">
              <SplitButton item={card.primary} active={isSplitActive(card.primary)} onClick={handleSplitFilter}></SplitButton>
              <SplitButton item={card.secondary} active={isSplitActive(card.secondary)} onClick={handleSplitFilter}></SplitButton>
            </div>
            <div class="overview-split-bar" aria-hidden="true">
              <span class={"overview-split-bar-segment overview-split-bar-" + card.primary.tone} style={{ width: card.primary.percent + "%" }}></span>
              <span class={"overview-split-bar-segment overview-split-bar-" + card.secondary.tone} style={{ width: card.secondary.percent + "%" }}></span>
            </div>
            <div class="overview-card-detail">{card.detail}</div>
          </div>
        </article>
      )}
    </section>
  );
}

function SplitButton(props: { item: SplitSummaryItem; active: boolean; onClick: (item: SplitSummaryItem) => void }) {
  const title = () => props.item.label + ": " + props.item.value + ". " + props.item.detail;

  return (
    <button
      type="button"
      class={"overview-split-action overview-split-" + props.item.tone + (props.active ? " is-active" : "")}
      title={title()}
      aria-label={title()}
      aria-pressed={props.active}
      onClick={[props.onClick, props.item]}
    >
      <span class="overview-split-action-main">
        <i class={"bi " + props.item.icon} aria-hidden="true"></i>
        <span>{props.item.label}</span>
      </span>
      <span class="overview-split-action-value">{props.item.value}</span>
      <span class="overview-split-action-percent">{props.item.percent}%</span>
    </button>
  );
}

export default SummaryCards;
