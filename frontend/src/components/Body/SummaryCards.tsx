import { createMemo } from "solid-js";
import { bkpHosts, filterState, setHistUpdOnFilter } from "../../functions/exports";
import { toggleHostFilter } from "../../functions/filter";
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
  scopeCount: number;
  icon: string;
  tone: string;
  primary: SplitSummaryItem;
  secondary: SplitSummaryItem;
};

function SummaryCards() {
  const summary = createMemo(() => {
    const hosts = bkpHosts();
    const filters = filterState();
    const statusFacetHosts = filterHosts(hosts, filters, { ignore: ["Now"] });
    const knownFacetHosts = filterHosts(hosts, filters, { ignore: ["Known"] });
    const filtersActive = hasActiveHostFilters(filters);

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

    return [
      {
        label: "Connectivity",
        scopeCount: statusFacetHosts.length,
        icon: "bi-broadcast-pin",
        tone: "connectivity",
        primary: makeSplit("On", online, statusFacetHosts.length, "bi-check-circle-fill", "online", "Now", 1),
        secondary: makeSplit("Off", offline, statusFacetHosts.length, "bi-slash-circle-fill", "offline", "Now", 0),
      },
      {
        label: "Recognition",
        scopeCount: knownFacetHosts.length,
        icon: "bi-bookmarks-fill",
        tone: "recognition",
        primary: makeSplit("Known", known, knownFacetHosts.length, "bi-bookmark-check-fill", "known", "Known", 1),
        secondary: makeSplit("Unknown", unknown, knownFacetHosts.length, "bi-question-circle-fill", "unknown", "Known", 0),
      },
    ] satisfies CombinedSummary[];
  });

  const isSplitActive = (item: SplitSummaryItem) => filterState()[item.filterField] === item.filterValue;

  const handleSplitFilter = (item: SplitSummaryItem) => {
    toggleHostFilter(item.filterField, item.filterValue);
    setHistUpdOnFilter(true);
  };

  return (
    <section class="overview-grid home-overview-grid" aria-label="Device overview">
      {summary().map((card) =>
        <article
          class={"overview-card overview-card-split overview-card-" + card.tone}
          aria-label={card.label + ": " + card.scopeCount + " scoped devices"}
        >
          <div class="overview-summary-rail">
            <div class="overview-summary-rail-icon" aria-hidden="true">
              <i class={"bi " + card.icon}></i>
            </div>
            <div class="overview-summary-title">{card.label}</div>
          </div>
          <div class="overview-split-content">
            <div class="overview-summary-scope">
              <strong>{card.scopeCount}</strong>
              <span>scoped devices</span>
            </div>
            <div class="overview-split-actions">
              <SplitButton item={card.primary} active={isSplitActive(card.primary)} onClick={handleSplitFilter}></SplitButton>
              <SplitButton item={card.secondary} active={isSplitActive(card.secondary)} onClick={handleSplitFilter}></SplitButton>
            </div>
            <div
              class="overview-split-bar"
              aria-hidden="true"
              title={card.primary.percent + "% " + card.primary.label + ", " + card.secondary.percent + "% " + card.secondary.label}
            >
              <SplitBarSegment item={card.primary}></SplitBarSegment>
              <SplitBarSegment item={card.secondary}></SplitBarSegment>
            </div>
          </div>
        </article>
      )}
    </section>
  );
}

function SplitBarSegment(props: { item: SplitSummaryItem }) {
  const hasShare = () => props.item.percent > 0;

  return (
    <span
      class={"overview-split-bar-segment overview-split-bar-" + props.item.tone}
      style={{
        display: "flex",
        "align-items": "center",
        "justify-content": "center",
        "flex-grow": String(props.item.percent),
        "flex-basis": "0",
        "min-width": hasShare() ? "2.5rem" : "0",
        overflow: "hidden",
      }}
    >
      {hasShare() && (
        <span
          style={{
            color: "rgba(255, 255, 255, 0.96)",
            "font-size": "0.63rem",
            "font-weight": "900",
            "line-height": "1",
            "white-space": "nowrap",
          }}
        >
          {props.item.percent}%
        </span>
      )}
    </span>
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
      <span class="overview-split-action-value">{props.item.value}</span>
      <span class="overview-split-action-main">
        <i class={"bi " + props.item.icon} aria-hidden="true"></i>
        <span>{props.item.label}</span>
      </span>
    </button>
  );
}

export default SummaryCards;
