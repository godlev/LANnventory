import ActivityTable from "../components/ActivityTable";
import { appConfig, bkpHosts, type HostEvent } from "../functions/exports";

function Activity() {
  const hostExists = (event: HostEvent) => bkpHosts().some((host) => host.ID === event.HostID && host.Mac === event.Mac);

  const retentionText = () => {
    const hours = appConfig().TrimHist;
    if (!Number.isFinite(hours) || hours <= 0) {
      return "Showing currently retained activity";
    }

    return "Showing activity retained for the last " + hours + " " + (hours === 1 ? "hour" : "hours");
  };

  return (
    <div class="activity-page">
      <header class="activity-page-header">
        <div>
          <h1 class="activity-page-title">Activity</h1>
          <p class="activity-page-subtitle">Meaningful device and connectivity events</p>
        </div>
        <div class="activity-retention-note">{retentionText()}</div>
      </header>

      <ActivityTable
        category="connectivity"
        title="Connectivity"
        subtitle="Online and offline transitions"
        emptyText="No connectivity events recorded yet"
        variant="connectivity"
        hostExists={hostExists}
      ></ActivityTable>

      <ActivityTable
        category="changes"
        title="Device changes"
        subtitle="Discovery and classification changes"
        emptyText="No device changes recorded yet"
        variant="changes"
        hostExists={hostExists}
      ></ActivityTable>
    </div>
  );
}

export default Activity;
