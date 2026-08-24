import { createSignal, onMount } from "solid-js";
import { apiGetVersion } from "../../functions/api"

function About() {

  const [version, setVersion] = createSignal('');
  const [link, setLink] = createSignal('');

  onMount(async () => {
    const v = await apiGetVersion();
    setVersion(v);
    setLink("https://github.com/aceberg/WatchYourLAN/releases/tag/"+v);
  });

  return (
    <div class="card wyl-panel config-panel">
      <div class="card-header">
        About
      </div>
      <div class="card-body table-responsive">
        <table class="table config-info-table"><tbody>
          <tr>
            <td class="config-field-label"><b>Version</b></td>
            <td class="config-field-value">
              <a href={link()} target="_blank" rel="noreferrer">{version()}</a>
            </td>
          </tr>
          <tr>
            <td class="config-field-label"><b>Swagger API docs</b></td>
            <td class="config-field-value"><a href="/swagger/index.html" target="_blank" rel="noreferrer">/swagger/index.html</a></td>
          </tr>
          <tr>
            <td class="config-field-label"><b>Project</b></td>
            <td class="config-field-value"><a href="https://github.com/aceberg/WatchYourLAN" target="_blank" rel="noreferrer">WatchYourLAN on GitHub</a></td>
          </tr>
          <tr>
            <td class="config-field-label"><b>Network docs</b></td>
            <td class="config-field-value"><a href="https://github.com/aceberg/WatchYourLAN/blob/main/docs/VLAN_ARP_SCAN.md" target="_blank" rel="noreferrer">VLAN and ARP scan guide</a></td>
          </tr>
          <tr>
            <td class="config-field-label config-field-label-top"><b>Notifications</b></td>
            <td class="config-field-value">Shoutrrr supports Discord, Email, Gotify, Telegram and other services. <a href="https://shoutrrr.nickfedor.com/services/overview/" target="_blank" rel="noreferrer">Service documentation</a></td>
          </tr>
          <tr>
            <td class="config-field-label config-field-label-top"><b>PostgreSQL URL</b></td>
            <td class="config-field-value">Connection string parameters are documented by <a href="https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters" target="_blank" rel="noreferrer">lib/pq</a>.</td>
          </tr>
        </tbody></table>
      </div>
    </div>
  )
}

export default About
