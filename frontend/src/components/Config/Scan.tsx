import { createSignal, For, Show } from "solid-js"
import { appConfig } from "../../functions/exports"
import { apiPath } from "../../functions/api"
import { saveErrorMessage, submitConfigForm } from "../../functions/configForms";

function Scan() {
  const [status, setStatus] = createSignal("");
  const [error, setError] = createSignal("");

  const handleSubmit = async (event: SubmitEvent) => {
    event.preventDefault();

    const form = event.currentTarget as HTMLFormElement;
    setStatus("");
    setError("");

    try {
      await submitConfigForm(form);
      setStatus("Saved");
    } catch (saveError) {
      setError(saveErrorMessage(saveError));
    }
  };

  return (
    <div class="card wyl-panel config-panel">
      <div class="card-header config-panel-heading">
        <span>Network discovery &amp; database</span>
        <span class="settings-behavior-badge">Save required</span>
      </div>
      <div class="card-body table-responsive">
        <div class="settings-behavior-note">
          <i class="bi bi-arrow-repeat" aria-hidden="true"></i>
          <span>Saving this card restarts network scanning. Current values remain active until you save.</span>
        </div>

        <form action={apiPath + '/api/config_settings/'} method="post" onSubmit={handleSubmit}>
          <table class="table table-borderless"><tbody>
            <tr class="config-subsection-row">
              <td colSpan={2}>Network discovery</td>
            </tr>
            <tr>
              <td class="config-field-label">Interfaces</td>
              <td class="config-field-value">
                <input name="ifaces" type="text" class="form-control" value={appConfig().Ifaces}></input>
                <div class="config-field-helper">Interfaces LANnventory scans for devices.</div>
              </td>
            </tr>
            <tr>
              <td class="config-field-label">Scan interval</td>
              <td class="config-field-value">
                <div class="config-value-with-unit">
                  <input name="timeout" type="number" class="form-control" value={appConfig().Timeout}></input>
                  <span class="config-field-unit">seconds</span>
                </div>
                <div class="config-field-helper">Time between network scans.</div>
              </td>
            </tr>

            <tr class="config-subsection-row">
              <td colSpan={2}>Database</td>
            </tr>
            <tr>
              <td class="config-field-label">Database backend</td>
              <td class="config-field-value"><select name="usedb" class="form-select">
                <Show
                  when={appConfig().UseDB == "sqlite"}
                  fallback={<>
                    <option value="sqlite">SQLite</option>
                    <option value="postgres" selected>PostgreSQL</option>
                  </>}
                >
                  <option value="sqlite" selected>SQLite</option>
                  <option value="postgres">PostgreSQL</option>
                </Show>
              </select></td>
            </tr>
            <tr>
              <td class="config-field-label config-field-label-top">PostgreSQL connection URL</td>
              <td class="config-field-value">
                <Show
                  when={appConfig().PGConnectConfigured}
                  fallback={<div class="config-secret-state is-empty"><i class="bi bi-dash-circle" aria-hidden="true"></i><span>Not configured</span></div>}
                >
                  <div class="config-secret-state is-stored"><i class="bi bi-shield-lock-fill" aria-hidden="true"></i><span>Stored securely · value hidden</span></div>
                </Show>
                <textarea
                  name="pgconnect"
                  class="form-control"
                  style="width: 100%;"
                  rows="3"
                  wrap="soft"
                  placeholder={appConfig().PGConnectConfigured ? "Leave blank to keep the stored connection URL" : "Enter PostgreSQL connection URL"}
                ></textarea>
                <Show when={appConfig().PGConnectConfigured}>
                  <label class="form-check config-secret-clear">
                    <input name="clear_pgconnect" class="form-check-input" type="checkbox"></input>
                    <span class="form-check-label">Clear stored PostgreSQL connection URL</span>
                  </label>
                </Show>
                <div class="config-field-helper">Stored database connection URLs are write-only and are never displayed after saving. Changing the database backend or connection URL reconnects storage during Save.</div>
              </td>
            </tr>

            <tr class="config-subsection-row">
              <td colSpan={2}>Advanced scanner</td>
            </tr>
            <tr>
              <td class="config-field-label">arp-scan arguments</td>
              <td class="config-field-value">
                <input name="arpargs" type="text" class="form-control" value={appConfig().ArpArgs}></input>
                <div class="config-field-helper">Passed directly to the existing arp-scan integration. Leave unchanged unless you need custom scanner behavior.</div>
              </td>
            </tr>
            <tr>
              <td class="config-field-label config-field-label-top">Static ARP strings</td>
              <td class="config-field-value">
                <For each={appConfig().ArpStrs}>{arpStr =>
                  <input name="arpstrs" type="text" class="form-control" value={arpStr}></input>
                }</For>
                <input name="arpstrs" type="text" class="form-control"></input>
              </td>
            </tr>
            <tr>
              <td class="config-field-label">Log level</td>
              <td class="config-field-value"><select name="log" class="form-select">
              <For each={["debug","info","warn","error"]}>{level =>
                <Show
                  when={level == appConfig().LogLevel}
                  fallback={<option value={level}>{level}</option>}
                >
                <option value={level} selected>{level}</option>
                </Show>
              }</For>
              </select></td>
            </tr>

            <tr>
              <td></td>
              <td class="config-action-cell">
                <button type="submit" class="btn btn-sm wyl-button">Save scan settings</button>
                <span class={"config-save-status" + (error() ? " config-save-error" : "")} role="status">
                  {error() || status()}
                </span>
                <div class="config-field-helper config-save-helper">Saving these settings restarts network scanning.</div>
              </td>
            </tr>
            </tbody></table>
        </form>
      </div>
    </div>
  )
}

export default Scan
