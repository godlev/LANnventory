import { createEffect, For, Show } from "solid-js"
import Filter from "../components/Filter"
import { allHosts, histUpdOnFilter, Host, setHistUpdOnFilter, setShow, show } from "../functions/exports"
import MacHistory from "../components/MacHistory"
import HistShow from "../components/HistShow"

function History() {

  let hosts: Host[] = [];
  hosts.push(...allHosts);

  const showStr = localStorage.getItem("histShow") as string;
  setShow(+showStr);
  (show() === 0 || isNaN(show())) ? setShow(200) : '';
  
  createEffect(() => {
    if (histUpdOnFilter()) {
      hosts = [];
      hosts.push(...allHosts);
      console.log("Upd on Filter");
      setHistUpdOnFilter(false);
    }
  });

  return (
    <div class="card border-primary wyl-panel history-panel">
      <div class="card-header history-panel-header">
        <Filter></Filter>
        <HistShow name="histShow"></HistShow>
      </div>
      <div class="card-body history-panel-body table-responsive">
        <table class="table table-hover history-table">
          <tbody>
          <Show
            when={!histUpdOnFilter()}
          >
            <For each={hosts}>{(host, index) =>
            <tr>
              <td class="history-table-index opacity-50">{index()+1}.</td>
              <td class="history-host-cell">
                <a href={"/host/"+host.ID}>{host.Name}</a><br></br>
                <a href={"http://"+host.IP}>{host.IP}</a>
              </td>
              <td class="history-mac-cell">
                <MacHistory mac={host.Mac} date=""></MacHistory>
              </td>
            </tr>
            }</For>
          </Show>
          </tbody> 
        </table>
      </div>
    </div>
  )
}

export default History
