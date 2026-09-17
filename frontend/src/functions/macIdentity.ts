export type MACType =
  | "globally-administered"
  | "locally-administered"
  | "multicast"
  | "invalid";

export function macTypeLabel(macType?: MACType): string {
  switch (macType) {
    case "globally-administered":
      return "Globally administered";
    case "locally-administered":
      return "Locally administered";
    case "multicast":
      return "Multicast / group";
    case "invalid":
      return "Invalid / unknown";
    default:
      return "Unknown";
  }
}
