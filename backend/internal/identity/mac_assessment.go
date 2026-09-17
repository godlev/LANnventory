package identity

import "strings"

// MACAssessmentCode is an interpretation layered on top of the technical MAC
// classification. It is intentionally conservative: a locally administered
// MAC alone is never treated as proof of privacy randomization.
type MACAssessmentCode string

const (
	MACAssessmentGloballyAdministered    MACAssessmentCode = "globally-administered"
	MACAssessmentLocallyAdministered     MACAssessmentCode = "locally-administered"
	MACAssessmentExpectedVirtual         MACAssessmentCode = "expected-virtual"
	MACAssessmentPossiblePrivateRandom   MACAssessmentCode = "possible-private-randomized"
	MACAssessmentGroupAddress            MACAssessmentCode = "group-address"
	MACAssessmentInvalid                 MACAssessmentCode = "invalid"
)

// MACAssessmentConfidence describes confidence in the interpretation, not in
// the technical U/L classification itself.
type MACAssessmentConfidence string

const (
	MACAssessmentConfidenceNone MACAssessmentConfidence = "none"
	MACAssessmentConfidenceLow  MACAssessmentConfidence = "low"
	MACAssessmentConfidenceHigh MACAssessmentConfidence = "high"
)

// MACAssessment is derived from the MAC classification plus managed inventory
// context. It is not persisted and must never trigger automatic correlation or
// merging of device identities.
type MACAssessment struct {
	Code       MACAssessmentCode
	Confidence MACAssessmentConfidence
	Reason     string
}

// AssessMAC returns a conservative interpretation of a MAC address. Managed
// DeviceType is treated as user-provided context. Stronger privacy conclusions
// require correlation evidence and are deliberately deferred to Phase 33B.
func AssessMAC(raw, deviceType string) MACAssessment {
	switch ClassifyMAC(raw) {
	case MACTypeGloballyAdministered:
		return MACAssessment{
			Code:       MACAssessmentGloballyAdministered,
			Confidence: MACAssessmentConfidenceNone,
			Reason:     "The MAC is globally administered; no privacy inference is made.",
		}
	case MACTypeMulticast:
		return MACAssessment{
			Code:       MACAssessmentGroupAddress,
			Confidence: MACAssessmentConfidenceNone,
			Reason:     "This is a multicast/group address, not a normal unicast device identity.",
		}
	case MACTypeInvalid:
		return MACAssessment{
			Code:       MACAssessmentInvalid,
			Confidence: MACAssessmentConfidenceNone,
			Reason:     "The MAC address is invalid or unsupported.",
		}
	}

	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "virtual-machine", "container":
		return MACAssessment{
			Code:       MACAssessmentExpectedVirtual,
			Confidence: MACAssessmentConfidenceHigh,
			Reason:     "The U/L bit is set and the managed device type is virtual, where locally administered MAC addresses are expected.",
		}
	case "phone", "tablet", "laptop":
		return MACAssessment{
			Code:       MACAssessmentPossiblePrivateRandom,
			Confidence: MACAssessmentConfidenceLow,
			Reason:     "The U/L bit is set and the managed device type is a client device. This is only a suggestion; the MAC may also be virtual or manually assigned.",
		}
	default:
		return MACAssessment{
			Code:       MACAssessmentLocallyAdministered,
			Confidence: MACAssessmentConfidenceNone,
			Reason:     "The U/L bit is set, but that alone is not enough to determine whether the MAC is private, randomized, virtual, or manually assigned.",
		}
	}
}
