import { onMount } from "solid-js"
import About from "../components/Config/About"
import Basic from "../components/Config/Basic"
import Influx from "../components/Config/Influx"
import Prometheus from "../components/Config/Prometheus"
import Retention from "../components/Config/Retention"
import Scan from "../components/Config/Scan"

function Config() {
  const sections = [
    { label: "General", href: "#general" },
    { label: "Scanning & database", href: "#scanning" },
    { label: "Data retention", href: "#data-retention" },
    { label: "Integrations", href: "#integrations" },
    { label: "About", href: "#about" },
  ];

  onMount(() => {
    const scrollToHash = () => {
      const id = window.location.hash.slice(1);
      if (!id) {
        return;
      }

      document.getElementById(id)?.scrollIntoView();
    };

    requestAnimationFrame(scrollToHash);
    window.setTimeout(scrollToHash, 100);
    window.setTimeout(scrollToHash, 500);
  });

  return (
    <div class="settings-page">
      <header class="settings-page-header">
        <h1>Settings</h1>
        <p>Configure WatchYourLAN</p>
      </header>

      <div class="settings-layout">
        <nav class="settings-section-nav" aria-label="Settings sections">
          {sections.map((section) =>
            <a href={section.href}>{section.label}</a>
          )}
        </nav>

        <div class="settings-content">
          <section id="general" class="settings-section" aria-label="General">
            <Basic></Basic>
          </section>

          <section id="scanning" class="settings-section" aria-label="Scanning and database">
            <Scan></Scan>
          </section>

          <section class="settings-section" aria-label="Data retention">
            <Retention></Retention>
          </section>

          <section id="integrations" class="settings-section" aria-label="Integrations">
            <div class="settings-section-heading">
              <h2>Integrations</h2>
              <p>Send WatchYourLAN metrics to external monitoring systems.</p>
            </div>
            <Influx></Influx>
            <Prometheus></Prometheus>
          </section>

          <section id="about" class="settings-section" aria-label="About">
            <About></About>
          </section>
        </div>
      </div>
    </div>
  )
}

export default Config
