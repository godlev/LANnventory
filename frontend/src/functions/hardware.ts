export function isUnknownHardware(value: string | null | undefined): boolean {
  const normalized = (value ?? "").trim().toLowerCase();
  if (normalized === "" || normalized === "unknown" || normalized === "(unknown)") {
    return true;
  }

  return normalized.startsWith("unknown:") || normalized.startsWith("(unknown:");
}
