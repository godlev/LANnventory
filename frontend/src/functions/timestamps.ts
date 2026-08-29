export type LocalTimestampParts = {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
};

const explicitTimestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:\d{2})$/;
const legacyTimestampPattern = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?$/;

export function parseApiTimestamp(value: string | null | undefined): Date | null {
  const raw = (value || "").trim();
  if (!raw) {
    return null;
  }

  if (explicitTimestampPattern.test(raw)) {
    const date = new Date(raw);
    return Number.isNaN(date.getTime()) ? null : date;
  }

  const match = legacyTimestampPattern.exec(raw);
  if (!match) {
    return null;
  }

  const [, year, month, day, hour, minute, second = "0"] = match;
  const timestamp = {
    year: Number(year),
    month: Number(month),
    day: Number(day),
    hour: Number(hour),
    minute: Number(minute),
    second: Number(second),
  };

  const date = new Date(
    timestamp.year,
    timestamp.month - 1,
    timestamp.day,
    timestamp.hour,
    timestamp.minute,
    timestamp.second,
  );

  if (
    date.getFullYear() !== timestamp.year
    || date.getMonth() !== timestamp.month - 1
    || date.getDate() !== timestamp.day
    || date.getHours() !== timestamp.hour
    || date.getMinutes() !== timestamp.minute
    || date.getSeconds() !== timestamp.second
  ) {
    return null;
  }

  return date;
}

export function localTimestampParts(value: string | null | undefined): LocalTimestampParts | null {
  const date = parseApiTimestamp(value);
  if (!date) {
    return null;
  }

  return {
    year: date.getFullYear(),
    month: date.getMonth() + 1,
    day: date.getDate(),
    hour: date.getHours(),
    minute: date.getMinutes(),
    second: date.getSeconds(),
  };
}

export function localDayKey(value: string | null | undefined): string | null {
  const parts = localTimestampParts(value);
  if (!parts) {
    return null;
  }

  return [
    String(parts.year).padStart(4, "0"),
    String(parts.month).padStart(2, "0"),
    String(parts.day).padStart(2, "0"),
  ].join("-");
}

export function localDayLabel(value: string | null | undefined): string {
  const date = parseApiTimestamp(value);
  if (!date) {
    return "Unknown day";
  }

  return date.toLocaleDateString("en", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

export function formatTimestampTitle(value: string | null | undefined): string {
  const parts = localTimestampParts(value);
  if (!parts) {
    return value || "";
  }

  return `${String(parts.day).padStart(2, "0")} ${monthName(parts.month)} ${parts.year} ${twoDigits(parts.hour)}:${twoDigits(parts.minute)}:${twoDigits(parts.second)}`;
}

export function formatTimestampCompact(value: string | null | undefined): string {
  const parts = localTimestampParts(value);
  if (!parts) {
    return value || "";
  }

  const currentYear = new Date().getFullYear();
  if (parts.year === currentYear) {
    return `${twoDigits(parts.day)} ${monthName(parts.month)} ${twoDigits(parts.hour)}:${twoDigits(parts.minute)}`;
  }

  return `${twoDigits(parts.day)} ${monthName(parts.month)} ${parts.year} ${twoDigits(parts.hour)}:${twoDigits(parts.minute)}`;
}

export function compareApiTimestampsDesc(left: string, right: string): number {
  const leftDate = parseApiTimestamp(left);
  const rightDate = parseApiTimestamp(right);

  if (leftDate && rightDate) {
    return rightDate.getTime() - leftDate.getTime();
  }
  if (rightDate) {
    return 1;
  }
  if (leftDate) {
    return -1;
  }

  return right.localeCompare(left);
}

function monthName(month: number): string {
  return ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"][month - 1] || "";
}

function twoDigits(value: number): string {
  return String(value).padStart(2, "0");
}
