export type LocalDayRange = {
  from: string;
  to: string;
};

export function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "";
  } catch {
    return "";
  }
}

export function localDayUTCRange(date: string): LocalDayRange | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (!match) {
    return null;
  }

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const start = new Date(year, month - 1, day, 0, 0, 0, 0);

  if (Number.isNaN(start.getTime())
    || start.getFullYear() !== year
    || start.getMonth() !== month - 1
    || start.getDate() !== day) {
    return null;
  }

  const end = new Date(year, month - 1, day + 1, 0, 0, 0, 0);

  return {
    from: start.toISOString(),
    to: end.toISOString(),
  };
}
