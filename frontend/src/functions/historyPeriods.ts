import { localTimestampParts } from "./timestamps";

export const HISTORY_DAY_START_HOUR = 7;
export const HISTORY_NIGHT_START_HOUR = 20;

export type HistoryPeriod = "day" | "night";

export type HistoryTimestamp = {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
};

export function parseHistoryTimestamp(date: string): HistoryTimestamp | null {
  return localTimestampParts(date);
}

export function getHistoryPeriod(date: string): HistoryPeriod | "" {
  const timestamp = parseHistoryTimestamp(date);

  if (!timestamp) {
    return "";
  }

  return timestamp.hour >= HISTORY_DAY_START_HOUR && timestamp.hour < HISTORY_NIGHT_START_HOUR
    ? "day"
    : "night";
}

export function historyPeriodLabel(period: HistoryPeriod | "") {
  if (period === "day") return "Day";
  if (period === "night") return "Night";
  return "";
}
