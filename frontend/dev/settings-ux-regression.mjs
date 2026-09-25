import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const read = (path) => readFileSync(resolve(root, path), "utf8");

const configPage = read("src/pages/Config.tsx");
const basic = read("src/components/Config/Basic.tsx");
const scan = read("src/components/Config/Scan.tsx");
const updates = read("src/components/Config/Updates.tsx");
const influx = read("src/components/Config/Influx.tsx");
const prometheus = read("src/components/Config/Prometheus.tsx");
const retention = read("src/components/Config/Retention.tsx");
const appStyles = read("src/App.css");

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

requireText(configPage, 'class="settings-shell"', "Settings must use a section-navigation shell.");
requireText(configPage, 'class="settings-nav"', "Desktop Settings navigation must remain explicit.");
requireText(configPage, 'class="settings-mobile-nav"', "Mobile Settings navigation must remain explicit.");
requireText(configPage, 'activeSection() === "general"', "General settings must remain independently navigable.");
requireText(configPage, 'activeSection() === "network"', "Network discovery must remain independently navigable.");
requireText(configPage, 'activeSection() === "diagnostics"', "Diagnostics must remain independently navigable.");
requireText(configPage, 'activeSection() === "data"', "Data and retention must remain independently navigable.");
requireText(configPage, 'activeSection() === "updates"', "Updates must remain independently navigable.");
requireText(configPage, 'activeSection() === "integrations"', "Integrations must remain independently navigable.");
requireText(configPage, 'activeSection() === "about"', "About must remain independently navigable.");
requireText(configPage, 'scanning: "network"', "Legacy #scanning links must continue to resolve.");
requireText(configPage, '"data-retention": "data"', "Legacy data-retention links must continue to resolve.");
requireText(configPage, '"data-export": "data"', "Legacy data-export links must continue to resolve.");
requireText(configPage, "event.preventDefault()", "Settings navigation must own hash changes predictably.");
forbidText(configPage, 'class="settings-layout"', "Legacy two-column Settings layout must not return.");

requireText(basic, "Save general settings", "General save domain must remain explicit.");
requireText(basic, 'action={apiPath + \'/api/config/\'}', "General settings must keep their existing independent save domain.");
requireText(scan, "Save scan settings", "Scan/database save domain must remain explicit.");
requireText(scan, 'action={apiPath + \'/api/config_settings/\'}', "Network scanning must keep its existing independent save domain.");
requireText(scan, "Saving these settings restarts network scanning.", "Scanner restart side effect must remain visible.");

requireText(basic, "Stored securely · value hidden", "Stored Shoutrrr secret must expose configured state without revealing the value.");
requireText(basic, "Leave blank to keep the stored URL", "Stored Shoutrrr secret replacement behavior must remain explicit.");
requireText(basic, "write-only and are never displayed after saving", "Stored Shoutrrr secret must remain write-only.");
requireText(basic, "Clear stored Shoutrrr URL", "Shoutrrr secret must retain explicit clearing.");
requireText(scan, "Stored securely · value hidden", "Stored PostgreSQL secret must expose configured state without revealing the value.");
requireText(scan, "Leave blank to keep the stored connection URL", "Stored PostgreSQL secret replacement behavior must remain explicit.");
requireText(scan, "write-only and are never displayed after saving", "Stored PostgreSQL secret must remain write-only.");
requireText(scan, "Clear stored PostgreSQL connection URL", "PostgreSQL secret must retain explicit clearing.");
requireText(influx, "Stored securely · value hidden", "Stored Influx token must expose configured state without revealing the value.");
requireText(influx, "Leave blank to keep the stored token", "Stored Influx token replacement behavior must remain explicit.");
requireText(influx, "write-only and are never displayed after saving", "Stored Influx token must remain write-only.");
requireText(influx, "Clear stored InfluxDB token", "Influx secret must retain explicit clearing.");

requireText(retention, "Save required", "Retention must identify its explicit-save behavior.");
requireText(retention, "staged until you choose", "Retention staged-save behavior must remain explicit.");
requireText(influx, "Save required", "InfluxDB must identify its explicit-save behavior.");
requireText(influx, "staged until you choose", "InfluxDB staged-save behavior must remain explicit.");
requireText(prometheus, "Save required", "Prometheus must identify its explicit-save behavior.");
requireText(prometheus, "staged until you choose", "Prometheus staged-save behavior must remain explicit.");
requireText(updates, "Update preferences save immediately.", "Update preferences must identify immediate-save behavior.");

requireText(updates, 'onChange={(event) => void saveSettings', "Update controls must retain immediate-save behavior.");
requireText(updates, "Install updates automatically", "Automatic-install setting must remain explicit.");
requireText(updates, "Check automatically", "Automatic-check-only setting must remain explicit.");

requireText(appStyles, ".settings-shell", "Settings shell must have dedicated layout styling.");
requireText(appStyles, ".settings-nav-link.is-active", "Active Settings navigation must be visually distinct.");
requireText(appStyles, ".settings-mobile-nav", "Settings mobile navigation must have responsive styling.");

console.log("Settings UX semantic contracts passed.");
