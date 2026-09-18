import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const read = (path) => readFileSync(resolve(root, path), "utf8");

const identity = read("src/components/HostPage/IdentityCard.tsx");
const correlation = read("src/components/HostPage/CorrelationPanel.tsx");
const hostActivity = read("src/components/HostPage/HostActivityCard.tsx");
const activityFeed = read("src/components/ActivityFeed.tsx");
const hostCard = read("src/components/HostPage/HostCard.tsx");

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

console.log("Host UX semantic regression checks passed.");
