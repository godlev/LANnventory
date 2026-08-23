import { bkpHosts, filterState, Host, setAllHosts, sortState } from "./exports";

export function applyHostView() {
  const filters = filterState();
  let hosts = [...bkpHosts()];

  if (filters.Iface !== "") {
    hosts = hosts.filter((host) => host.Iface === filters.Iface);
  }
  if (filters.Known !== "") {
    hosts = hosts.filter((host) => host.Known === filters.Known);
  }
  if (filters.Now !== "") {
    hosts = hosts.filter((host) => host.Now === filters.Now);
  }
  if (filters.Search !== "") {
    const search = filters.Search.toLowerCase();
    hosts = hosts.filter((host) => searchItem(host, search));
  }

  setAllHosts(sortHosts(hosts));
}

function sortHosts(hosts: Host[]) {
  const currentSort = sortState();

  if (!currentSort.field || !currentSort.direction) {
    return hosts;
  }

  const ascending = currentSort.direction === "ascending";
  const sortedHosts = [...hosts];

  if (currentSort.field === "IP") {
    sortedHosts.sort((a, b) => sortIP(a, b, ascending));
  } else {
    sortedHosts.sort((a, b) => byField(a, b, currentSort.field as keyof Host, ascending));
  }

  return sortedHosts;
}

function searchItem(host: Host, search: string) {
  const name = host.Name.toLowerCase();
  const hardware = host.Hw.toLowerCase();
  const mac = host.Mac.toLowerCase();
  const iface = host.Iface.toLowerCase();

  return name.includes(search)
    || iface.includes(search)
    || host.IP.includes(search)
    || mac.includes(search)
    || hardware.includes(search)
    || host.Date.includes(search);
}

function byField(a: Host, b: Host, fieldName: keyof Host, ascending: boolean) {
  if (a[fieldName] === b[fieldName]) {
    return 0;
  }
  if (a[fieldName] > b[fieldName]) {
    return ascending ? 1 : -1;
  }
  return ascending ? -1 : 1;
}

function sortIP(a: Host, b: Host, ascending: boolean) {
  const num1 = numIP(a);
  const num2 = numIP(b);
  return ascending ? num1 - num2 : num2 - num1;
}

function numIP(host: Host) {
  return Number(host.IP.split(".").map((num) => (`000${num}`).slice(-3)).join(""));
}
