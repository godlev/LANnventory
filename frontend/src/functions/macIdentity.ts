export type MACType =
  | "globally-administered"
  | "locally-administered"
  | "multicast"
  | "invalid";

export type MACAssessmentCode =
  | "globally-administered"
  | "locally-administered"
  | "expected-virtual"
  | "possible-private-randomized"
  | "group-address"
  | "invalid";

export type MACAssessmentConfidence = "none" | "low" | "high";

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

export function macAssessmentLabel(assessment?: MACAssessmentCode): string {
  switch (assessment) {
    case "expected-virtual":
      return "Expected for virtual device";
    case "possible-private-randomized":
      return "Possible private / randomized";
    case "locally-administered":
      return "Privacy status unknown";
    case "globally-administered":
      return "No local-MAC indication";
    case "group-address":
      return "Group address";
    case "invalid":
      return "Unable to assess";
    default:
      return "Unknown";
  }
}

export function macConfidenceLabel(confidence?: MACAssessmentConfidence): string {
  switch (confidence) {
    case "low":
      return "Low confidence";
    case "high":
      return "High confidence";
    default:
      return "";
  }
}
