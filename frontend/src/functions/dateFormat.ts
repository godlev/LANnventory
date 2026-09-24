export function formatLastSeen(date: string) {
  const value = date.trim();
  const match = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2})(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?$/.exec(value);

  if (!match) {
    return date;
  }

  const [, rawYear, rawMonth, rawDay, rawHour, rawMinute, rawSecond, rawFraction, rawZone] = match;
  let year = rawYear;
  let month = rawMonth;
  let day = rawDay;
  let hour = rawHour;
  let minute = rawMinute;

  // Legacy LANnventory timestamps without an explicit timezone are already
  // display-local values. RFC3339 timestamps from newer APIs include Z/offset
  // and must be converted to the browser's local timezone before rendering.
  if (rawZone) {
    const zone = rawZone === "Z" || rawZone.includes(":")
      ? rawZone
      : rawZone.slice(0, 3) + ":" + rawZone.slice(3);
    const seconds = rawSecond ? ":" + rawSecond + (rawFraction || "") : ":00";
    const instant = new Date(
      rawYear + "-" + rawMonth + "-" + rawDay + "T" + rawHour + ":" + rawMinute + seconds + zone,
    );

    if (!Number.isNaN(instant.getTime())) {
      year = String(instant.getFullYear());
      month = String(instant.getMonth() + 1).padStart(2, "0");
      day = String(instant.getDate()).padStart(2, "0");
      hour = String(instant.getHours()).padStart(2, "0");
      minute = String(instant.getMinutes()).padStart(2, "0");
    }
  }

  const monthName = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"][Number(month) - 1];
  const currentYear = new Date().getFullYear().toString();

  if (!monthName) {
    return date;
  }

  if (year === currentYear) {
    return `${day} ${monthName} ${hour}:${minute}`;
  }

  return `${day} ${monthName} ${year} ${hour}:${minute}`;
}
