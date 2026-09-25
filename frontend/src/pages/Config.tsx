import { createSignal, onCleanup, onMount, Show } from "solid-js"
import About from "../components/Config/About"
import Basic from "../components/Config/Basic"
import DataExport from "../components/Config/DataExport"
import Diagnostics from "../components/Config/Diagnostics"
import Influx from "../components/Config/Influx"
import Prometheus from "../components/Config/Prometheus"
import Retention from "../components/Config/Retention"
import Scan from "../components/Config/Scan"
import Updates from "../components/Config/Updates"
import "../components/Config/Updates.css"
import { refreshAppConfig } from "../functions/theme"

type SettingsSection = "general" | "network" | "diagnostics" | "data" | "updates" | "integrations" | "about";

const settingsSections: Array<{ id: SettingsSection; label: string; icon: string }> = [
  { id: "general", label: "General", icon: "bi-sliders" },
  { id: "network", label: "Network discovery", icon: "bi-router" },
  { id: "diagnostics", label: "Diagnostics", icon: "bi-heart-pulse" },
  { id: "data", label: "Data & retention", icon: "bi-database" },
  { id: "updates", label: "Updates", icon: "bi-arrow-up-circle" },
  { id: "integrations", label: "Integrations", icon: "bi-plug" },
  { id: "about", label: "About", icon: "bi-info-circle" },
];

function sectionFromHash(): SettingsSection {
  const value = window.location.hash.replace(/^#/, "");
  return settingsSections.some((section) => section.id === value)
    ? value as SettingsSection
    : "general";
}

function Config() {
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal("");
  const [activeSection, setActiveSection] = createSignal<SettingsSection>(sectionFromHash());

  const selectSection = (section: SettingsSection) => {
    setActiveSection(section);
    const nextHash = "#" + section;
    if (window.location.hash !== nextHash) {
      window.history.replaceState(null, "", window.location.pathname + window.location.search + nextHash);
    }
  };

  onMount(() => {
    const syncSectionFromHash = () => setActiveSection(sectionFromHash());
    window.addEventListener("hashchange", syncSectionFromHash);

    refreshAppConfig()
      .then(() => {
        setError("");
      })
      .catch(() => {
        setError("Settings could not be loaded. Check that the LANnventory backend is reachable, then reload this page.");
      })
      .finally(() => {
        setLoading(false);
      });

    onCleanup(() => window.removeEventListener("hashchange", syncSectionFromHash));
  });

  return (
    <div class="settings-page">
      <header class="settings-page-header">
        <h1>Settings</h1>
        <p>Configure LANnventory by area without scanning one long page.</p>
      </header>

      <Show when={!loading()} fallback={<div class="settings-status-panel" role="status">Loading settings</div>}>
        <Show when={!error()} fallback={<div class="settings-status-panel settings-status-error" role="alert">{error()}</div>}>
          <div class="settings-shell">
            <nav class="settings-nav" aria-label="Settings sections">
              {settingsSections.map((section) => (
                <a
                  href={"#" + section.id}
                  class={"settings-nav-link" + (activeSection() === section.id ? " is-active" : "")}
                  aria-current={activeSection() === section.id ? "page" : undefined}
                  onClick={() => selectSection(section.id)}
                >
                  <i class={"bi " + section.icon} aria-hidden="true"></i>
                  <span>{section.label}</span>
                </a>
              ))}
            </nav>

            <label class="settings-mobile-nav">
              <span>Settings section</span>
              <select
                class="form-select form-select-sm wyl-control"
                value={activeSection()}
                onChange={(event) => selectSection(event.currentTarget.value as SettingsSection)}
              >
                {settingsSections.map((section) => (
                  <option value={section.id}>{section.label}</option>
                ))}
              </select>
            </label>

            <main class="settings-content">
              <Show when={activeSection() === "general"}>
                <section id="general" class="settings-section" aria-label="General">
                  <div class="settings-section-heading">
                    <h2>General</h2>
                    <p>Application behavior, appearance and existing notification compatibility settings.</p>
                  </div>
                  <Basic />
                </section>
              </Show>

              <Show when={activeSection() === "network"}>
                <section id="network" class="settings-section" aria-label="Network discovery">
                  <div class="settings-section-heading">
                    <h2>Network discovery</h2>
                    <p>Interfaces, scan cadence, ARP scanner behavior and the current database-backed scanner configuration.</p>
                  </div>
                  <Scan />
                </section>
              </Show>

              <Show when={activeSection() === "diagnostics"}>
                <section id="diagnostics" class="settings-section" aria-label="Scanner health and diagnostics">
                  <div class="settings-section-heading">
                    <h2>Diagnostics</h2>
                    <p>Scanner, database and runtime health information.</p>
                  </div>
                  <Diagnostics />
                </section>
              </Show>

              <Show when={activeSection() === "data"}>
                <section id="data" class="settings-section" aria-label="Data and retention">
                  <div class="settings-section-heading">
                    <h2>Data &amp; retention</h2>
                    <p>Retention periods plus safe backup and inventory export actions.</p>
                  </div>
                  <Retention />
                  <DataExport />
                </section>
              </Show>

              <Show when={activeSection() === "updates"}>
                <section id="updates" class="settings-section" aria-label="Updates">
                  <div class="settings-section-heading">
                    <h2>Updates</h2>
                    <p>Release channel, automatic checks and installation status.</p>
                  </div>
                  <Updates />
                </section>
              </Show>

              <Show when={activeSection() === "integrations"}>
                <section id="integrations" class="settings-section" aria-label="Integrations">
                  <div class="settings-section-heading">
                    <h2>Integrations</h2>
                    <p>Send LANnventory metrics to external monitoring systems.</p>
                  </div>
                  <Influx />
                  <Prometheus />
                </section>
              </Show>

              <Show when={activeSection() === "about"}>
                <section id="about" class="settings-section" aria-label="About">
                  <div class="settings-section-heading">
                    <h2>About</h2>
                    <p>Version, project information, documentation and upstream credits.</p>
                  </div>
                  <About />
                </section>
              </Show>
            </main>
          </div>
        </Show>
      </Show>
    </div>
  )
}

export default Config
