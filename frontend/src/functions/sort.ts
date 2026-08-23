import { createSignal } from "solid-js";
import { bkpHosts, Host, setAllHosts, setBkpHosts } from "./exports";

export type SortDirection = "ascending" | "descending";

export interface SortState {
  field: keyof Host | "";
  direction: SortDirection | "";
}

const sortableFields = ["Name", "Iface", "IP", "Mac", "Hw", "Date", "Known", "Now"];

export const [sortState, setSortState] = createSignal<SortState>({
  field: "",
  direction: "",
});

export function sortAtStart() {
  const field = normalizeField(localStorage.getItem("sortField"));
  const direction = readStoredDirection();

  if (!field || !direction) {
    setSortState({ field: "", direction: "" });
    return;
  }

  applySort(field, direction);
}

export function sortByAnyField(field: keyof Host) {
  const current = sortState();
  const direction: SortDirection = current.field === field && current.direction === "ascending"
    ? "descending"
    : "ascending";

  applySort(field, direction);
}

function applySort(field: keyof Host, direction: SortDirection) {
  const ascending = direction === "ascending";
  const sortedHosts = [...bkpHosts()];

  localStorage.setItem("sortDown", ascending.toString());
  localStorage.setItem("sortField", field);

  if (field == 'IP') {
    sortedHosts.sort((a, b) => sortIP(a, b, ascending));
  } else {
    sortedHosts.sort((a, b) => byField(a, b, field, ascending));
  }

  setSortState({ field, direction });
  setBkpHosts(sortedHosts);
  setAllHosts(sortedHosts);
}

function normalizeField(field: string | null): keyof Host | "" {
  return field && sortableFields.includes(field) ? field as keyof Host : "";
}

function readStoredDirection(): SortDirection | "" {
  const stored = localStorage.getItem("sortDown");
  if (stored === "true") return "ascending";
  if (stored === "false") return "descending";
  return "";
}

function byField(a:Host, b:Host, fieldName: keyof Host, ascending:boolean){
  if (a[fieldName] === b[fieldName]) {
    return 0;
  }
  if (a[fieldName] > b[fieldName]) {
    return ascending ? 1 : -1;
  } else {
    return ascending ? -1 : 1;
  }
}

function sortIP(a:Host, b:Host, ascending: boolean) {
  const num1 = numIP(a);
  const num2 = numIP(b);
  if (ascending) {
    return num1-num2;
  } else {
    return num2-num1;
  } 
}

function numIP(a:Host) {
  return Number(a.IP.split(".").map((num) => (`000${num}`).slice(-3) ).join(""));
}
