import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const read = (path) => readFileSync(resolve(root, path), "utf8");

const identity = read("src/components/HostPage/IdentityCard.tsx");
const correlation = read("src/components/HostPage/CorrelationPanel.tsx");
const hostActivity = read("src/components/HostPage/HostActivityCard.tsx");
const activityFeed = read("src/components/ActivityFeed.tsx");
const hostCard = read("src/components/HostPage/HostCard.tsx");
const deviceProfile = read("src/components/HostPage/DeviceProfileCard.tsx");
const proxmoxInventory = read("src/components/HostPage/ProxmoxInventoryCard.tsx");
const hostedWorkload = read("src/components/HostPage/HostedWorkloadCard.tsx");
const homeTableRow = read("src/components/Body/TableRow.tsx");
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

requireText(identity, "Network identity history", "Identity section title must clarify history.");
requireText(identity, "This IP was also used by", "Shared IP wording must make the IP the grammatical subject.");
requireText(
  identity,
  "An IP address can be reused by different devices. This alone does not mean the MAC addresses belong to the same physical device.",
  "IP reuse warning must remain explicit.",
);
requireText(identity, '<MACObservation address={props.address.address}', "Historical MAC rows must retain explicit address context.");
forbidText(identity, "Also observed with", "Ambiguous shared-IP wording must not return.");

requireText(correlation, "Possible same device", "Correlation section must remain separate.");
requireText(correlation, "Shared IP history alone is not a same-device conclusion.", "Correlation must remain separate from address reuse.");
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

requireText(activityFeed, '<A href={"/host/" + event.HostID}', "Global Activity Host links must remain available.");

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
requireText(hostedWorkload, "Hosted on Proxmox", "Linked guest Host pages must expose their Proxmox parent.");
requireText(hostedWorkload, "Child relationship", "Guest Host relationship semantics must be explicit.");
requireText(homeTableRow, "device-workload-parent", "Home device rows must surface linked Proxmox workload ancestry.");
requireText(homeTableRow, "Linked Proxmox workload", "Home parent indicator must explain the relationship.");
forbidText(deviceTypes, '| "hypervisor"', "Hypervisor must not become an exclusive Device Type.");

console.log("Host UX semantic regression checks passed.");
