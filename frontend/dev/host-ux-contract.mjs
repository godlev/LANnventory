import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");

const identity = read("../src/components/HostPage/IdentityCard.tsx");
const correlation = read("../src/components/HostPage/CorrelationPanel.tsx");
const hostActivity = read("../src/components/HostPage/HostActivityCard.tsx");
const activityFeed = read("../src/components/ActivityFeed.tsx");
const globalActivity = read("../src/pages/Activity.tsx");
const hostCard = read("../src/components/HostPage/HostCard.tsx");
const css = read("../src/App.css");
const correlationTests = read("../../backend/internal/correlation/correlation_test.go");

assert.match(identity, /Network identity history/);
assert.match(identity, /This IP was also used by/);
assert.doesNotMatch(identity, /Also observed with/);
assert.match(identity, /An IP address can be reused by different devices\. This alone does not mean the MAC addresses belong to the same physical device\./);
assert.match(identity, /IP \{props\.address\}/);
assert.match(identity, /MAC \{props\.observation\.mac\}/);

assert.match(correlation, /Same-device correlation/);
assert.match(correlation, /Separate from IP address history/);
assert.match(correlation, /No evidence currently suggests that another MAC belongs to this same physical device\./);
assert.match(correlationTests, /TestCandidatesRejectSharedVendorOrAddressAlone/);
assert.match(correlationTests, /expected no candidate for shared vendor\/IP alone/);

assert.match(hostCard, />Current IP</);
assert.match(hostCard, />Current MAC</);
assert.match(hostCard, />Network vendor</);
assert.match(hostCard, />This MAC first seen</);
assert.match(hostCard, />This MAC last seen</);
assert.doesNotMatch(hostCard, />Hardware</);

assert.match(hostActivity, /\{event\.IP \|\| "—"\}/);
assert.match(hostActivity, /\{event\.Mac \|\| "—"\}/);
assert.doesNotMatch(hostActivity, /ActivityFeed/);
assert.doesNotMatch(hostActivity, /href=\{\"\/host\//);
assert.match(hostActivity, /Historical snapshots · IP and MAC are shown as recorded at event time\./);

assert.match(activityFeed, /<A href=\{\"\/host\/\" \+ event\.HostID\}/);
assert.match(globalActivity, /<A href=\{\"\/host\/\" \+ event\.HostID\}/);

assert.match(css, /\.host-event-row/);
assert.match(css, /@media \(max-width: 767\.98px\)/);
assert.match(css, /\.host-event-cell::before/);
assert.match(css, /content: attr\(data-label\)/);

console.log("Host UX semantic contract checks passed.");
