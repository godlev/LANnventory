import { createMemo, For } from "solid-js";
import { bkpHosts } from "../../functions/exports";

function SummaryCards() {
  const summary = createMemo(() => {
    const hosts = bkpHosts();
    const total = hosts.length;
    const online = hosts.filter((host) => host.Now === 1).length;
    const offline = hosts.filter((host) => host.Now === 0).length;
    const unknown = hosts.filter((host) => host.Known === 0).length;

    const percentage = (value: number) => total > 0
      ? Math.round((value / total) * 100) + "%"
      : "0%";

    return [
      {
        label: "Total Devices",
        value: total,
        detail: total === 1 ? "1 loaded host" : total + " loaded hosts",
        icon: "bi-hdd-network",
        tone: "total",
      },
      {
        label: "Online",
        value: online,
        detail: percentage(online) + " of devices",
        icon: "bi-check-circle-fill",
        tone: "online",
      },
      {
        label: "Offline",
        value: offline,
        detail: percentage(offline) + " of devices",
        icon: "bi-slash-circle-fill",
        tone: "offline",
      },
      {
        label: "Unknown",
        value: unknown,
        detail: percentage(unknown) + " of devices",
        icon: "bi-question-circle-fill",
        tone: "unknown",
      },
    ];
  });

  return (
    <section class="overview-grid" aria-label="Device overview">
      <For each={summary()}>{(item) =>
        <article class={"overview-card overview-card-" + item.tone}>
          <div class="overview-card-icon" aria-hidden="true">
            <i class={"bi " + item.icon}></i>
          </div>
          <div>
            <div class="overview-card-label">{item.label}</div>
            <div class="overview-card-value">{item.value}</div>
            <div class="overview-card-detail">{item.detail}</div>
          </div>
        </article>
      }</For>
    </section>
  );
}

export default SummaryCards;
