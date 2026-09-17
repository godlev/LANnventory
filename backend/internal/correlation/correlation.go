package correlation

import (
	"sort"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
)

// Confidence is a conservative assessment of whether two MAC identities may
// represent the same physical device. Candidates are suggestions only.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// AddressObservation is the minimum retained address state needed by the
// candidate engine.
type AddressObservation struct {
	Address string
	Active  bool
}

// EvidenceObservation is discovered identity evidence. Active distinguishes
// current evidence from retained historical evidence.
type EvidenceObservation struct {
	Kind   string
	Value  string
	Active bool
}

// IdentityObservation describes one MAC identity without implying that it is
// already a logical/physical device.
type IdentityObservation struct {
	Mac       string
	Active    bool
	Addresses []AddressObservation
	Evidence  []EvidenceObservation
}

// Reason is one explainable scoring input. Positive weights support a
// candidate; negative weights reduce confidence.
type Reason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Weight int    `json:"weight"`
}

// Candidate is a read-only suggestion. The engine never merges identities or
// mutates stored observations.
type Candidate struct {
	Mac        string     `json:"mac"`
	Score      int        `json:"score"`
	Confidence Confidence `json:"confidence"`
	Reasons    []Reason   `json:"reasons"`
}

// Candidates returns deterministic suggestions for one identity against the
// supplied observations. A weak fact such as a shared vendor or shared IP
// alone is intentionally insufficient.
func Candidates(target IdentityObservation, identities []IdentityObservation) []Candidate {
	target.Mac = identity.MACKey(target.Mac)
	if target.Mac == "" {
		return nil
	}

	result := make([]Candidate, 0)
	seen := make(map[string]struct{})
	for _, other := range identities {
		other.Mac = identity.MACKey(other.Mac)
		if other.Mac == "" || other.Mac == target.Mac {
			continue
		}
		if _, exists := seen[other.Mac]; exists {
			continue
		}
		seen[other.Mac] = struct{}{}

		candidate, eligible := compare(target, other)
		if eligible {
			result = append(result, candidate)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].Mac < result[j].Mac
	})
	return result
}

func compare(target, other IdentityObservation) (Candidate, bool) {
	reasons := make([]Reason, 0, 6)
	strongIdentity := false

	if value, bothCurrent, ok := sharedEvidence(target, other, "hostname"); ok {
		weight := 35
		if bothCurrent {
			weight = 50
		}
		reasons = append(reasons, Reason{Code: "shared-hostname", Detail: value, Weight: weight})
		strongIdentity = true
	}

	if value, bothCurrent, ok := sharedEvidence(target, other, "friendly-name"); ok {
		weight := 30
		if bothCurrent {
			weight = 45
		}
		reasons = append(reasons, Reason{Code: "shared-friendly-name", Detail: value, Weight: weight})
		strongIdentity = true
	}

	if detail, bothCurrent, ok := sharedDescriptor(target, other); ok {
		weight := 15
		if bothCurrent {
			weight = 25
		}
		reasons = append(reasons, Reason{Code: "shared-device-descriptor", Detail: detail, Weight: weight})
		strongIdentity = true
	}

	sharedAddress, sameAddressActive := sharedAddress(target, other)
	if sharedAddress != "" {
		reasons = append(reasons, Reason{Code: "shared-address-history", Detail: sharedAddress, Weight: 10})
	}

	localSupport := identity.ClassifyMAC(target.Mac) == identity.MACTypeLocallyAdministered ||
		identity.ClassifyMAC(other.Mac) == identity.MACTypeLocallyAdministered
	if localSupport && (strongIdentity || sharedAddress != "") {
		reasons = append(reasons, Reason{
			Code:   "locally-administered-support",
			Detail: "At least one MAC is locally administered; this supports possible rotation but is not proof.",
			Weight: 5,
		})
	}

	// Shared address history is only enough to form a weak candidate when a
	// locally administered address also supports a possible MAC rotation.
	eligible := strongIdentity || (sharedAddress != "" && localSupport)
	if !eligible {
		return Candidate{}, false
	}

	if target.Active && other.Active {
		reasons = append(reasons, Reason{
			Code:   "concurrent-active",
			Detail: "Both MAC identities are currently active; this argues against a simple MAC rotation and may indicate separate interfaces or devices.",
			Weight: -35,
		})
	}
	if sameAddressActive {
		reasons = append(reasons, Reason{
			Code:   "same-address-concurrent-conflict",
			Detail: "Both MAC identities are currently active on the same IP address.",
			Weight: -50,
		})
	}

	score := 0
	for _, reason := range reasons {
		score += reason.Weight
	}
	if score < 0 {
		score = 0
	}

	return Candidate{
		Mac:        other.Mac,
		Score:      score,
		Confidence: confidence(score),
		Reasons:    reasons,
	}, true
}

func confidence(score int) Confidence {
	switch {
	case score >= 70:
		return ConfidenceHigh
	case score >= 40:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

func sharedEvidence(a, b IdentityObservation, kind string) (value string, bothCurrent bool, ok bool) {
	aValues := evidenceValues(a.Evidence, kind)
	bValues := evidenceValues(b.Evidence, kind)
	keys := make([]string, 0)
	for candidate := range aValues {
		if _, exists := bValues[candidate]; exists {
			keys = append(keys, candidate)
		}
	}
	if len(keys) == 0 {
		return "", false, false
	}
	sort.Strings(keys)
	selected := keys[0]
	return selected, aValues[selected] && bValues[selected], true
}

func sharedDescriptor(a, b IdentityObservation) (detail string, bothCurrent bool, ok bool) {
	manufacturer, manufacturerCurrent, manufacturerOK := sharedEvidence(a, b, "manufacturer")
	modelNumber, modelCurrent, modelOK := sharedEvidence(a, b, "model-number")
	if !manufacturerOK || !modelOK {
		return "", false, false
	}
	return manufacturer + " / " + modelNumber, manufacturerCurrent && modelCurrent, true
}

func evidenceValues(items []EvidenceObservation, kind string) map[string]bool {
	values := make(map[string]bool)
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Kind)) != kind {
			continue
		}
		value := normalizeEvidenceValue(kind, item.Value)
		if value == "" {
			continue
		}
		values[value] = values[value] || item.Active
	}
	return values
}

func normalizeEvidenceValue(kind, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if kind == "hostname" {
		value = strings.TrimSuffix(value, ".")
	}
	return strings.Join(strings.Fields(value), " ")
}

func sharedAddress(a, b IdentityObservation) (address string, bothActive bool) {
	aAddresses := make(map[string]bool)
	for _, item := range a.Addresses {
		value := strings.TrimSpace(item.Address)
		if value == "" {
			continue
		}
		aAddresses[value] = aAddresses[value] || item.Active
	}

	matches := make([]string, 0)
	bActive := make(map[string]bool)
	for _, item := range b.Addresses {
		value := strings.TrimSpace(item.Address)
		if _, exists := aAddresses[value]; !exists {
			continue
		}
		matches = append(matches, value)
		bActive[value] = bActive[value] || item.Active
	}
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	selected := matches[0]
	return selected, aAddresses[selected] && bActive[selected]
}
