import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { formatLastSeen } from "../src/functions/dateFormat.ts";

const root = resolve(import.meta.dirname, "..");
const read = (path) => readFileSync(resolve(root, path), "utf8");

const identity = read("src/components/HostPage/IdentityCard.tsx");
const correlation = read("src/components/HostPage/CorrelationPanel.tsx");
const hostActivity = read("src/components/HostPage/HostActivityCard.tsx");
const hostHistory = read("src/components/HostPage/HistCard.tsx");
const servicesCard = read("src/components/HostPage/ServicesCard.tsx");
const activityFeed = read("src/components/ActivityFeed.tsx");
const hostPage = read("src/pages/HostPage.tsx");
const hostCard = read("src/components/HostPage/HostCard.tsx");
const portScan = read("src/components/HostPage/Ping.tsx");
const deviceProfile = read("src/components/HostPage/DeviceProfileCard.tsx");
const proxmoxInventory = read("src/components/HostPage/ProxmoxInventoryCard.tsx");
const hostedWorkload = read("src/components/HostPage/HostedWorkloadCard.tsx");
const homeBody = read("src/pages/Body.tsx");
const homeCardHead = read("src/components/Body/CardHead.tsx");
const homeTableRow = read("src/components/Body/TableRow.tsx");
const appStyles = read("src/App.css");
const deviceTypes = read("src/functions/deviceTypes.ts");

function requireText(source, value, message) {
  if (!source.includes(value)) {
    throw new Error(message + " Missing: " + value);
  }
}

function forbidText(source, value, message) {
  if (source.includes(value)) {
    throw new Error(message + " Unexpected: " + value);
  }
}

requireText(identity, "Network identity", "Identity section title must describe the current device identity.");
requireText(identity, "This IP was also used by", "Shared IP wording must make the IP the grammatical subject.");
requireText(
  identity,
  "An IP address can be reused by different devices. This alone does not mean the MAC addresses belong to the same physical device.",
  "IP reuse warning must remain explicit.",
);
requireText(identity, '<MACObservation address={props.address.address}', "Historical MAC rows must retain explicit address context.");
forbidText(identity, "Also observed with", "Ambiguous shared-IP wording must not return.");

requireText(identity, "Current network identity", "Current identity observations must have visual priority.");
requireText(identity, '<details class="host-identity-history">', "Historical identity observations must use progressive disclosure.");
requireText(identity, "Previous addresses and names are historical observations.", "Identity history must explicitly distinguish history from current state.");
requireText(appStyles, ".host-identity-history", "Identity history must have dedicated secondary styling.");

requireText(correlation, "Possible identity matches", "Possible identity matches must remain separate from IP reuse history.");
requireText(correlation, "Shared IP history alone is not a same-device conclusion.", "Correlation must remain separate from address reuse.");
requireText(correlation, "Confirmed same-device identities", "Confirmed user decisions must use user-facing identity terminology.");
forbidText(correlation, "Read only projection", "Implementation-oriented correlation terminology must not be exposed to users.");
requireText(
  correlation,
  "No evidence currently suggests that another MAC belongs to this same physical device.",
  "Empty correlation state must not imply hidden same-device evidence.",
);

requireText(hostActivity, '{event.IP || "—"}', "Host Recent Events must use historical event.IP.");
requireText(hostActivity, '{event.Mac || "—"}', "Host Recent Events must use historical event.Mac.");
requireText(hostActivity, '<th scope="col">Event</th>', "Host Recent Events must expose Event column.");
requireText(hostActivity, '<th scope="col">IP</th>', "Host Recent Events must expose IP column.");
requireText(hostActivity, '<th scope="col">MAC</th>', "Host Recent Events must expose MAC column.");
requireText(hostActivity, '<th scope="col">When</th>', "Host Recent Events must expose When column.");
forbidText(hostActivity, 'href={"/host/"', "Host Recent Events must not link back to the same Host page.");
forbidText(hostActivity, "activityHostName", "Host Recent Events must not repeat the Host name.");

requireText(hostActivity, "Recent events could not be loaded.", "Host Recent Events must expose section-level API failures.");
requireText(hostActivity, "Retry", "Host Recent Events must offer a local retry action.");
requireText(hostActivity, 'class="host-section-error"', "Secondary Host section failure must not replace the whole Host page.");
requireText(appStyles, ".host-section-error", "Partial Host section errors must have dedicated styling.");

requireText(activityFeed, '<A href={"/host/" + event.HostID}', "Global Activity Host links must remain available.");

requireText(hostHistory, 'class="card wyl-panel host-history-panel host-history-disclosure"', "Presence history must be secondary progressive disclosure.");
requireText(hostHistory, '<Show when={expanded()}>', "Presence history content must load only after the user opens it.");
requireText(hostHistory, '>History</span>', "Presence history must be labelled as historical context.");
requireText(appStyles, ".host-history-disclosure", "Presence history disclosure must have dedicated styling.");

requireText(servicesCard, "Current open services", "Services must prioritize current open state.");
requireText(servicesCard, "Manual service scan", "Manual port scanning must live with service inventory.");
requireText(servicesCard, '<Ping', "Services must own the manual scan workflow.");
requireText(portScan, "props.embedded", "Port scan must support embedded Services presentation.");
requireText(hostPage, 'class="col-12 host-details-column"', "Device Overview must use the full Host page width.");
forbidText(hostPage, "host-port-column", "Port scan must not remain a top-level peer of Device Overview.");
requireText(servicesCard, "Previously observed services", "Closed services must remain available as explicit history.");
requireText(servicesCard, "Closed services are retained as history.", "Historical service state must not imply a current open service.");
requireText(servicesCard, '<details class="host-services-secondary">', "Historical services and scan settings must use progressive disclosure.");
requireText(servicesCard, "Scheduled scanning", "Per-host scheduled scan controls must remain available.");
requireText(servicesCard, 'data-label="Service"', "Services table must expose mobile row labels.");
requireText(appStyles, ".host-services-table td::before", "Services must transform into labelled rows on narrow screens.");

requireText(hostCard, 'class="host-overview-main"', "Host must expose a first-class Device Overview hierarchy.");
requireText(hostCard, '{overviewName()}', "Device Overview must lead with the device display name.");
requireText(hostCard, '{currentDeviceType().label}', "Device Overview must expose device type.");
requireText(hostCard, '{statusText()}', "Device Overview must expose current online/offline state.");
requireText(hostCard, '{_props.host.IP || "No IP"}', "Device Overview must expose current IP.");
requireText(hostCard, '{_props.host.Mac || "No MAC"}', "Device Overview must expose current MAC.");
requireText(hostCard, '>Managed information</span>', "User-controlled Host fields must be labelled Managed information.");
requireText(hostCard, '>Current network</span>', "Observed current network fields must be grouped separately.");
requireText(hostCard, '>Discovered · Read only</span>', "Discovered network data must expose read-only provenance.");
forbidText(hostCard, '>Host details</div>', "Legacy feature-card title must not remain the primary Host hierarchy.");
requireText(appStyles, ".host-overview-header", "Device Overview must have dedicated responsive styling.");
requireText(appStyles, ".host-overview-network", "Current network identity must have dedicated summary styling.");

requireText(hostCard, 'class="host-overview-more"', "Destructive Host actions must be separated from routine actions.");
requireText(hostCard, "More device actions", "Host action overflow must have an accessible name.");
requireText(hostCard, "This action cannot be undone.", "Delete must require explicit destructive confirmation.");
requireText(hostCard, ">Delete device</span>", "Delete must remain available as an explicit device action.");
forbidText(hostCard, 'class="host-actions"', "Delete must not remain visually tied to edit-mode Save/Cancel actions.");
requireText(appStyles, ".host-overview-danger-action", "Destructive Host action must have dedicated visual treatment.");

for (const label of [
  "Current IP",
  "Current MAC",
  "MAC type",
  "Interface",
  "Vendor",
  "This MAC first seen",
  "This MAC last seen",
]) {
  requireText(hostCard, ">" + label + "</div>", "Host Details label " + label + " must remain explicit.");
}
forbidText(hostCard, ">Hardware</div>", "ARP/OUI vendor text must not be labelled Hardware.");

requireText(deviceProfile, "Managed inventory only.", "Device Profile must identify user-managed provenance.");
requireText(deviceProfile, "Imported Proxmox data is kept separate", "Managed/imported profile separation must remain explicit.");
requireText(deviceProfile, "Data source", "Device Profile must expose a data-source legend.");
requireText(deviceProfile, 'source="manual"', "Managed Device Profile fields must carry manual provenance.");
requireText(deviceProfile, 'source="discovered"', "Data-source legend must explain discovered provenance.");
requireText(deviceProfile, 'source="imported"', "Data-source legend must explain imported provenance.");
requireText(deviceProfile, "Imports will not overwrite this field.", "Manual field tooltip must state overwrite protection.");
requireText(deviceProfile, "optional reference values", "Hypervisor reference fields must be explained as optional.");
requireText(deviceProfile, "Capability profile; the Host remains Device Type Server.", "Hypervisor must remain a profile capability rather than a Device Type.");
requireText(deviceProfile, '<option value="proxmox-ve">Proxmox VE</option>', "Manual Proxmox profile option must remain available.");

requireText(deviceProfile, "No device profile configured", "Empty Device Profiles must collapse into a useful compact state.");
requireText(deviceProfile, "hasAnyProfile", "Device Profile must distinguish meaningful profile data from empty typed sections.");
requireText(deviceProfile, "props.editing || props.value", "Unset profile fields must stay hidden until editing.");
requireText(appStyles, ".profile-empty-state", "Empty Device Profile state must have dedicated compact styling.");

requireText(proxmoxInventory, "How access works", "Proxmox onboarding must explain collector permissions.");
requireText(proxmoxInventory, "uses the permissions of the shell user", "Collector access model must be explicit.");
requireText(proxmoxInventory, "Open the Proxmox shell", "Proxmox import must be presented as a guided workflow.");
requireText(proxmoxInventory, "Create a compact snapshot file", "Collector execution must be a dedicated wizard step.");
requireText(proxmoxInventory, "Download collector", "Collector must be downloadable from the LANnventory UI.");
requireText(proxmoxInventory, "/lannventory-proxmox-collector.py", "Collector download must be served by the local LANnventory instance.");
requireText(proxmoxInventory, "Preview changes", "Preview must remain mandatory and explicit.");
requireText(proxmoxInventory, "Nothing is written until you review and confirm the import.", "Preview/apply safety wording must remain explicit.");
requireText(proxmoxInventory, "Imported: observed by the read-only Proxmox collector.", "Imported source-state fields must expose provenance.");
requireText(proxmoxInventory, '--compact > "+snapshotPath', "Collector workflow must write compact output to a file instead of flooding the shell.");
requireText(proxmoxInventory, "Recommended for larger environments:", "Large-environment file transfer guidance must remain explicit.");
requireText(proxmoxInventory, "scp root@", "Wizard must provide an SCP file-transfer option.");
requireText(proxmoxInventory, "Select command", "Shell commands must be manually selected rather than copied programmatically.");
forbidText(proxmoxInventory, "navigator.clipboard.writeText", "Shell commands must not be written to the clipboard programmatically.");
requireText(proxmoxInventory, "What Import will do in LANnventory", "Preview must explain the effect of Import before confirmation.");
requireText(proxmoxInventory, "no fake Hosts are created", "Preview must clarify that workloads are not LANnventory Hosts.");
requireText(proxmoxInventory, "No VM/LXC is created, changed, started, stopped, or deleted on Proxmox.", "Preview must explicitly state that Proxmox is not modified.");
requireText(proxmoxInventory, "Managed Device Profile fields and manual workload links are not overwritten.", "Preview must state preservation guarantees.");
requireText(proxmoxInventory, "Import into LANnventory", "Final action must be named as a LANnventory import rather than ambiguous Apply.");
requireText(proxmoxInventory, "Possible IP conflict", "Address-only matches with conflicting MACs must be first-class diagnostics.");
requireText(proxmoxInventory, "IP address match only", "Weak address evidence must be described explicitly.");
requireText(proxmoxInventory, "MAC differs", "IP conflict diagnostics must expose the contradictory MAC evidence.");
requireText(proxmoxInventory, "Link anyway", "Weak matches must require explicit manual linking.");
requireText(proxmoxInventory, "Not this Host", "Weak matches must support explicit rejection.");
requireText(proxmoxInventory, "Rejected suggestions", "Rejected candidates must remain recoverable without returning as active suggestions.");
requireText(proxmoxInventory, "No MAC confirmation", "Name/address-only evidence must not imply MAC confirmation.");
requireText(proxmoxInventory, "Linked automatically", "Persisted exact-MAC relationships must be visibly identified as already linked.");
requireText(proxmoxInventory, "Linked manually", "Persisted manual relationships must be visibly identified as already linked.");
requireText(proxmoxInventory, "Remove link", "The linked-state action must describe removing an existing relationship.");
requireText(proxmoxInventory, "Historical IP reuse is kept in Host history only.", "Historical IP reuse must be explained as context rather than workload-match evidence.");
requireText(proxmoxInventory, "Manual / script", "Proxmox inventory must retain the existing script source selector.");
requireText(proxmoxInventory, "Proxmox API", "Proxmox inventory must offer the optional API source.");
requireText(proxmoxInventory, "Connection &amp; collection", "Proxmox connection and collection controls must be secondary disclosure.");
requireText(proxmoxInventory, "Source details", "Technical Proxmox collector metadata must remain accessible without dominating the summary.");
requireText(proxmoxInventory, "Current LANnventory view of the last successfully applied read-only inventory snapshot.", "Proxmox summary must describe current inventory state.");
requireText(proxmoxInventory, 'data-label="Guest"', "Proxmox workloads must expose mobile row labels.");
requireText(appStyles, ".proxmox-workload-table td::before", "Proxmox workload table must transform into labelled rows on narrow screens.");
requireText(proxmoxInventory, "API Token Secret", "Proxmox API setup must expose a dedicated token-secret field.");
requireText(proxmoxInventory, "Stored secret — leave blank to keep", "Stored Proxmox token secrets must remain write-only in the UI.");
requireText(proxmoxInventory, "Clear stored token secret", "Proxmox API setup must provide explicit secret clearing.");
requireText(proxmoxInventory, "Verify TLS certificate", "TLS verification must be explicit in Proxmox API setup.");
requireText(proxmoxInventory, "TLS certificate verification is disabled explicitly.", "Disabling TLS verification must show an explicit warning.");
requireText(proxmoxInventory, "Test connection", "Proxmox API setup must provide a non-mutating connection test.");
requireText(proxmoxInventory, "Connection verified. No LANnventory inventory was changed.", "Connection test must state its non-mutating behavior.");
requireText(proxmoxInventory, "Sync now", "Proxmox API setup must expose manual sync.");
requireText(proxmoxInventory, "Automatic Sync", "Proxmox API setup must expose per-host automatic sync.");
requireText(proxmoxInventory, "Every 15 minutes", "Automatic sync must expose the supported interval selector.");
requireText(proxmoxInventory, "Every 24 hours", "Automatic sync must expose the full supported interval range.");
requireText(proxmoxInventory, "Last attempt", "Automatic sync status must expose the last attempt.");
requireText(proxmoxInventory, "Last successful collection", "Automatic sync status must separate successful collection from apply.");
requireText(proxmoxInventory, "Last applied", "Automatic sync status must expose the last applied snapshot.");
requireText(proxmoxInventory, "Next sync", "Automatic sync status must expose the next scheduled run.");
requireText(proxmoxInventory, "Review required.", "Automatic safety blocks must be visible and actionable.");
requireText(proxmoxInventory, "does not start an immediate sync", "Enabling automatic sync must not imply an immediate run.");
requireText(proxmoxInventory, "First sync preview ready.", "First API sync must remain a preview before enablement.");
requireText(proxmoxInventory, "The first successful Apply also enables this API inventory source.", "API source enablement must follow reviewed Apply.");
requireText(proxmoxInventory, "Disable API source", "An enabled Proxmox API source must be explicitly disableable.");
requireText(proxmoxInventory, "Existing inventory was preserved.", "API sync failure wording must preserve last-good inventory semantics.");
forbidText(proxmoxInventory, "Enable API inventory source", "API source must not be enableable before the first reviewed Apply.");
requireText(hostedWorkload, "Hosted on Proxmox", "Linked guest Host pages must expose their Proxmox parent.");
requireText(hostedWorkload, "Child relationship", "Guest Host relationship semantics must be explicit.");
requireText(homeTableRow, "device-workload-parent", "Home device rows must surface linked Proxmox workload ancestry.");
requireText(homeTableRow, "Linked Proxmox workload", "Home parent indicator must explain the relationship.");
requireText(homeBody, "lannventory.home.tableView", "Home table view preference must persist in browser storage.");
requireText(homeBody, 'device-table-" + tableView()', "Home table must expose the selected Comfortable/Compact mode as a CSS class.");
requireText(homeCardHead, "Comfortable", "Home toolbar must offer Comfortable table view.");
requireText(homeCardHead, "Compact", "Home toolbar must offer Compact table view.");
requireText(homeTableRow, '_props.viewMode === "compact"', "Compact Home rows must have dedicated workload rendering.");
requireText(homeTableRow, "device-compact-workload", "Compact Home rows must expose a one-line Proxmox workload marker.");
requireText(homeTableRow, "device-hypervisor-summary", "Comfortable Home rows must expose Proxmox VM/LXC totals under the hypervisor name.");
requireText(homeTableRow, "device-compact-hypervisor-summary", "Compact Home rows must keep Proxmox VM/LXC totals on one line.");
requireText(homeTableRow, "containerCount", "Proxmox Home summary must expose LXC/container counts.");
requireText(homeTableRow, "vmCount", "Proxmox Home summary must expose VM counts.");
requireText(homeBody, "apiGetInfrastructureWorkloadSummaries", "Home must load full hypervisor workload inventory counts rather than count only linked workloads.");
requireText(read("dev/mock-api.mjs"), "/api/infrastructure/workload-summaries", "Mock backend must expose the Home workload summary endpoint.");
requireText(homeTableRow, '_props.viewMode !== "compact"', "Comfortable-only second-line workload ancestry must be suppressed in Compact view.");
requireText(appStyles, ".device-table-compact tbody td", "Compact Home table must reduce row density.");
requireText(appStyles, ".device-compact-workload", "Compact Proxmox workload marker must have dedicated styling.");
requireText(appStyles, ".device-hypervisor-summary", "Comfortable Proxmox inventory totals must have dedicated styling.");
requireText(appStyles, ".device-compact-hypervisor-summary", "Compact Proxmox inventory totals must have dedicated styling.");
forbidText(deviceTypes, '| "hypervisor"', "Hypervisor must not become an exclusive Device Type.");

const originalTZ = process.env.TZ;
process.env.TZ = "Europe/Sofia";
const currentYear = new Date().getFullYear();
if (formatLastSeen(`${currentYear}-09-24T18:16:00Z`) !== "24 Sep 21:16") {
  throw new Error("RFC3339 UTC timestamps must be rendered in the browser-local timezone.");
}
if (formatLastSeen(`${currentYear}-09-24T18:16:00+00:00`) !== "24 Sep 21:16") {
  throw new Error("RFC3339 offset timestamps must be rendered in the browser-local timezone.");
}
if (formatLastSeen(`${currentYear}-09-24 18:16:00`) !== "24 Sep 18:16") {
  throw new Error("Legacy timestamps without timezone information must preserve their displayed clock time.");
}
if (originalTZ === undefined) {
  delete process.env.TZ;
} else {
  process.env.TZ = originalTZ;
}

console.log("Host UX semantic regression checks passed.");
