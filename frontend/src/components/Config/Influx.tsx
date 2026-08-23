import { apiPath } from "../../functions/api"
import { appConfig } from "../../functions/exports"

function Influx() {

  return (
    <div class="card wyl-panel config-panel">
          <div class="card-header">InfluxDB2 config</div>
          <div class="card-body table-responsive">
            <form action={apiPath + '/api/config_influx/'} method="post">
              <table class="table table-borderless"><tbody>
                <tr>
                  <td class="config-field-label">Enable</td>
                  <td class="config-field-value">
                    <div class="form-check form-switch">
                      {appConfig().InfluxEnable
                        ? <input class="form-check-input" type="checkbox" name="enable" checked></input>
                        : <input class="form-check-input" type="checkbox" name="enable"></input>
                      }
                    </div>
                  </td>
                </tr>
                <tr>
                  <td class="config-field-label">Address</td>
                  <td class="config-field-value"><input name="addr" type="text" class="form-control" value={appConfig().InfluxAddr}></input></td>
                </tr>
                <tr>
                  <td class="config-field-label">Token</td>
                  <td class="config-field-value"><input name="token" type="text" class="form-control" value={appConfig().InfluxToken}></input></td>
                </tr>
                <tr>
                  <td class="config-field-label">Org</td>
                  <td class="config-field-value"><input name="org" type="text" class="form-control" value={appConfig().InfluxOrg}></input></td>
                </tr>
                <tr>
                  <td class="config-field-label">Bucket</td>
                  <td class="config-field-value"><input name="bucket" type="text" class="form-control" value={appConfig().InfluxBucket}></input></td>
                </tr>
                <tr>
                  <td class="config-field-label">Skip TLS verify</td>
                  <td class="config-field-value">
                    <div class="form-check form-switch">
                      {appConfig().InfluxSkipTLS
                        ? <input class="form-check-input" type="checkbox" name="skip" checked></input>
                        : <input class="form-check-input" type="checkbox" name="skip"></input>
                      }
                    </div>
                  </td>
                </tr>
                <tr>
                  <td class="config-action-cell"><button type="submit" class="btn btn-sm wyl-button">Save</button></td>
                  <td class="config-action-cell"></td>
                </tr>
              </tbody></table>
            </form>
          </div>
        </div>
  )
}

export default Influx
