import { For, Show } from "solid-js"
import { appConfig } from "../../functions/exports"
import { apiPath } from "../../functions/api"

function Scan() {

  return (
    <div class="card wyl-panel config-panel">
      <div class="card-header">Scan settings</div>
      <div class="card-body table-responsive">
        <form action={apiPath + '/api/config_settings/'} method="post">
          <table class="table table-borderless"><tbody>
            <tr>
              <td class="config-field-label">Interfaces</td>
              <td class="config-field-value"><input name="ifaces" type="text" class="form-control" value={appConfig().Ifaces}></input></td>
            </tr>
            <tr>
              <td class="config-field-label">Timeout (seconds)</td>
              <td class="config-field-value"><input name="timeout" type="number" class="form-control" value={appConfig().Timeout}></input></td>
            </tr>
            <tr>
              <td class="config-field-label">Args for arp-scan</td>
              <td class="config-field-value"><input name="arpargs" type="text" class="form-control" value={appConfig().ArpArgs}></input></td>
            </tr>
            <tr>
              <td class="config-field-label config-field-label-top">Arp Strings</td>
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
              <td class="config-field-label">Trim History (hours)</td>
              <td class="config-field-value"><input name="trim" type="number" class="form-control" value={appConfig().TrimHist}></input></td>
            </tr>
            <tr>
              <td class="config-field-label">Use DB</td>
              <td class="config-field-value"><select name="usedb" class="form-select">
                <Show
                  when={appConfig().UseDB == "sqlite"}
                  fallback={<>
                    <option value="sqlite">sqlite</option>
                    <option value="postgres" selected>postgres</option>
                  </>}
                >
                  <option value="sqlite" selected>sqlite</option>
                  <option value="postgres">postgres</option>
                </Show>
              </select></td>
            </tr>
            <tr>
              <td class="config-field-label config-field-label-top">PG Connect URL</td>
              <td class="config-field-value">
                <textarea name="pgconnect" class="form-control" style="width: 100%;" rows="3" wrap="soft">{appConfig().PGConnect}</textarea>
              </td>
            </tr>
            <tr>
              <td class="config-action-cell"><button type="submit" class="btn btn-sm wyl-button">Save</button></td>
              <td class="config-action-cell text-muted">*Pressing <b>Save</b> button will trigger rescan</td>
            </tr>
            </tbody></table>
        </form>
      </div>
    </div>
  )
}

export default Scan
