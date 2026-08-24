import { apiPath } from "../../functions/api"
import { appConfig } from "../../functions/exports"

function Prometheus() {

  return (
    <div class="card wyl-panel config-panel">
      <div class="card-header">Prometheus config</div>
      <div class="card-body table-responsive">
        <form action={apiPath + '/api/config_prometheus/'} method="post">
          <table class="table table-borderless"><tbody>
            <tr>
              <td class="config-field-label">Enable</td>
              <td class="config-field-value">
                <div class="form-check form-switch">
                  {appConfig().PrometheusEnable
                    ? <input class="form-check-input" type="checkbox" name="enable" checked></input>
                    : <input class="form-check-input" type="checkbox" name="enable"></input>
                  }
                </div>
              </td>
            </tr>
            <tr>
              <td class="config-action-cell"><button type="submit" class="btn btn-sm wyl-button">Save Prometheus</button></td>
              <td class="config-action-cell">
                <a href="/metrics" target="_blank">/metrics</a>
              </td>
            </tr>
          </tbody></table>
        </form>
      </div>
    </div>
  )
}

export default Prometheus
