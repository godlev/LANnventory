import { For, onMount } from "solid-js";

import { allHosts, bkpHosts } from "../functions/exports";

import TableRow from "../components/Body/TableRow";
import TableHead from "../components/Body/TableHead";
import CardHead from "../components/Body/CardHead";
import SummaryCards from "../components/Body/SummaryCards";
import { getHosts } from "../functions/atstart";

function Body() {

  onMount(() => {
    getHosts();
  });

  return (
    <>
    <SummaryCards></SummaryCards>
    <div class="card border-primary device-panel">
      <div class="card-header device-panel-header">
        <div class="device-panel-title-group">
          <div class="device-panel-title">Devices</div>
          <div class="device-panel-subtitle">{bkpHosts().length} loaded hosts</div>
        </div>
        <CardHead></CardHead>
      </div>
      <div class="card-body table-responsive device-table-wrap">
        <table class="table table-striped table-hover device-table">
          <TableHead></TableHead>
          <tbody>
            <For each={allHosts}>{(host, index) =>
            <TableRow host={host} index={index() + 1}></TableRow>
            }</For>
          </tbody> 
        </table>
      </div>
    </div>
    </>
  )
}

export default Body
