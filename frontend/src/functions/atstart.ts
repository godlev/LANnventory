import { apiGetAllHosts } from "./api";
import { Host, setBkpHosts, setIfaces } from "./exports";
import { applyHostView } from "./hostView";
import { filterAtStart } from "./filter";
import { sortAtStart } from "./sort";

export function runAtStart() {
  getHosts();

  setInterval(() => {
    getHosts();
  }, 60000); // 60000 ms = 1 minute
}

export async function getHosts() {
  const hosts = await apiGetAllHosts();

  if (hosts !== null && hosts.length > 0) {
    setBkpHosts(hosts);

    listIfaces(hosts);
    sortAtStart();
    filterAtStart();
    applyHostView();
  }
}

function listIfaces(hosts: Host[]) {

  let ifaces:string[] = [];

  for (let host of hosts) {
    if (!ifaces.includes(host.Iface)) {
      ifaces.push(host.Iface);
    }
  }

  setIfaces(ifaces);
}
