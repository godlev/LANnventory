import { apiGetHistory, apiGetHistoryByDate } from "./api";
import { Host } from "./exports";
import { browserTimeZone, localDayUTCRange } from "./historyDate";

export async function getHistoryForMac(mac: string, date: string) {
  let h: Host[] = [];
  const timeZone = browserTimeZone();

  if (date === "") {
    h = await apiGetHistory(mac, timeZone);
  } else {
    const range = localDayUTCRange(date);
    h = await apiGetHistoryByDate(
      mac,
      date,
      range ? { ...range, timeZone } : undefined,
    );
  }

  if (h != null) {
    h.sort((a: Host, b: Host) => (a.Date < b.Date ? 1 : -1));
    return h;
  }
  return [];
}
