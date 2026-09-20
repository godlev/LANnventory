package workloadmatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	StrengthExactMAC = "exact-mac"
	StrengthAddress  = "address"
	StrengthName     = "name"
)

type Evidence struct {
	Code         string `json:"code"`
	Detail       string `json:"detail"`
	Strength     string `json:"strength"`
	Active       bool   `json:"active"`
	MatchedValue string `json:"matchedValue,omitempty"`
	FirstSeen    string `json:"firstSeen,omitempty"`
	LastSeen     string `json:"lastSeen,omitempty"`
}

type Candidate struct {
	HostID              int        `json:"hostId"`
	Mac                 string     `json:"mac"`
	Name                string     `json:"name"`
	IP                  string     `json:"ip"`
	DeviceType          string     `json:"deviceType"`
	Active              bool       `json:"active"`
	Strength            string     `json:"strength"`
	Assessment          string     `json:"assessment"`
	PossibleIPConflict  bool       `json:"possibleIpConflict"`
	MatchedAddresses    []string   `json:"matchedAddresses"`
	WorkloadMACs        []string   `json:"workloadMacs"`
	EvidenceFingerprint string     `json:"evidenceFingerprint"`
	Rejected            bool       `json:"rejected"`
	Evidence            []Evidence `json:"evidence"`
}

type Result struct {
	DeterministicExactHostID int         `json:"deterministicExactHostId,omitempty"`
	ExactAmbiguous           bool        `json:"exactAmbiguous"`
	Candidates               []Candidate `json:"candidates"`
}

// Match returns explainable Host candidates for one infrastructure workload.
// Only a single, deterministic exact MAC match is eligible for automatic linking.
// Address and name evidence are suggestions only.
func Match(
	record models.InfrastructureWorkloadRecord,
	hypervisorMAC string,
	hosts []models.Host,
	addresses []models.HostAddress,
	discovery []models.HostDiscoveryEvidence,
) Result {
	sourceMAC := identity.MACKey(hypervisorMAC)
	hostByID := make(map[int]models.Host, len(hosts))
	hostIDsByMAC := make(map[string][]int)
	for _, host := range hosts {
		if host.ID < 1 {
			continue
		}
		mac := identity.MACKey(host.Mac)
		if mac == "" || mac == sourceMAC {
			continue
		}
		hostByID[host.ID] = host
		hostIDsByMAC[mac] = append(hostIDsByMAC[mac], host.ID)
	}
	for mac := range hostIDsByMAC {
		sort.Ints(hostIDsByMAC[mac])
	}

	candidates := make(map[int]*Candidate)
	addEvidence := func(hostID int, evidence Evidence) {
		host, ok := hostByID[hostID]
		if !ok {
			return
		}
		candidate := candidates[hostID]
		if candidate == nil {
			candidate = &Candidate{
				HostID:     host.ID,
				Mac:        identity.MACKey(host.Mac),
				Name:       host.Name,
				IP:         host.IP,
				DeviceType: host.DeviceType,
				Active:     host.Now == 1,
				Strength:   evidence.Strength,
				Evidence:   []Evidence{},
			}
			candidates[hostID] = candidate
		}
		if strengthRank(evidence.Strength) < strengthRank(candidate.Strength) {
			candidate.Strength = evidence.Strength
		}
		for _, existing := range candidate.Evidence {
			if existing.Code == evidence.Code && existing.Detail == evidence.Detail && existing.Active == evidence.Active {
				return
			}
		}
		candidate.Evidence = append(candidate.Evidence, evidence)
	}

	workloadMACSet := make(map[string]struct{})
	for _, iface := range record.Interfaces {
		mac, err := identity.NormalizeMAC(iface.Mac)
		if err != nil {
			continue
		}
		workloadMACSet[mac] = struct{}{}
		for _, hostID := range hostIDsByMAC[mac] {
			addEvidence(hostID, Evidence{
				Code:         "exact-interface-mac",
				Detail:       fmt.Sprintf("%s MAC %s exactly matches the Host MAC", interfaceLabel(iface.Name), mac),
				Strength:     StrengthExactMAC,
				Active:       hostByID[hostID].Now == 1,
				MatchedValue: mac,
			})
		}
	}

	workloadAddresses := make(map[string][]string)
	for _, iface := range record.Interfaces {
		if address := normalizedAddress(iface.ConfiguredAddress); address != "" {
			workloadAddresses[address] = append(workloadAddresses[address], iface.Name)
		}
	}

	for _, host := range hostByID {
		address := normalizedAddress(host.IP)
		if address == "" {
			continue
		}
		ifaces, matched := workloadAddresses[address]
		if !matched {
			continue
		}
		addEvidence(host.ID, Evidence{
			Code:         "current-address",
			Detail:       fmt.Sprintf("%s configured address %s matches the Host current address", joinedInterfaceLabel(ifaces), address),
			Strength:     StrengthAddress,
			Active:       host.Now == 1,
			MatchedValue: address,
		})
	}

	for _, row := range addresses {
		address := normalizedAddress(row.Address)
		if address == "" {
			continue
		}
		ifaces, matched := workloadAddresses[address]
		if !matched {
			continue
		}
		for _, hostID := range hostIDsByMAC[identity.MACKey(row.Mac)] {
			code := "address-history"
			state := "retained address history"
			if row.Active {
				code = "active-address-observation"
				state = "active address observation"
			}
			addEvidence(hostID, Evidence{
				Code:         code,
				Detail:       fmt.Sprintf("%s configured address %s matches a Host %s", joinedInterfaceLabel(ifaces), address, state),
				Strength:     StrengthAddress,
				Active:       row.Active,
				MatchedValue: address,
				FirstSeen:    row.FirstSeen,
				LastSeen:     row.LastSeen,
			})
		}
	}

	workloadName := normalizedName(record.Workload.Name)
	if workloadName != "" {
		for _, host := range hostByID {
			if workloadName == normalizedName(host.Name) && strings.TrimSpace(host.Name) != "" {
				addEvidence(host.ID, Evidence{
					Code:         "host-name",
					Detail:       fmt.Sprintf("Workload name %q matches Host name %q", record.Workload.Name, host.Name),
					Strength:     StrengthName,
					Active:       host.Now == 1,
					MatchedValue: workloadName,
				})
			}
			if workloadName == normalizedName(host.DNS) && strings.TrimSpace(host.DNS) != "" {
				addEvidence(host.ID, Evidence{
					Code:         "host-dns",
					Detail:       fmt.Sprintf("Workload name %q matches Host DNS name %q", record.Workload.Name, host.DNS),
					Strength:     StrengthName,
					Active:       host.Now == 1,
					MatchedValue: workloadName,
				})
			}
		}

		for _, row := range discovery {
			kind := strings.ToLower(strings.TrimSpace(row.Kind))
			if kind != models.DiscoveryKindHostname && kind != models.DiscoveryKindFriendlyName {
				continue
			}
			if workloadName != normalizedName(row.Value) {
				continue
			}
			for _, hostID := range hostIDsByMAC[identity.MACKey(row.Mac)] {
				addEvidence(hostID, Evidence{
					Code:         "discovered-" + kind,
					Detail:       fmt.Sprintf("Workload name %q matches discovered %s %q", record.Workload.Name, kind, row.Value),
					Strength:     StrengthName,
					Active:       row.Active,
					MatchedValue: workloadName,
					FirstSeen:    row.FirstSeen,
					LastSeen:     row.LastSeen,
				})
			}
		}
	}

	workloadMACs := make([]string, 0, len(workloadMACSet))
	for mac := range workloadMACSet {
		workloadMACs = append(workloadMACs, mac)
	}
	sort.Strings(workloadMACs)

	result := Result{Candidates: []Candidate{}}
	exactHostIDs := make([]int, 0)
	for hostID, candidate := range candidates {
		sort.SliceStable(candidate.Evidence, func(i, j int) bool {
			left, right := candidate.Evidence[i], candidate.Evidence[j]
			if strengthRank(left.Strength) != strengthRank(right.Strength) {
				return strengthRank(left.Strength) < strengthRank(right.Strength)
			}
			if left.Active != right.Active {
				return left.Active
			}
			if left.Code != right.Code {
				return left.Code < right.Code
			}
			return left.Detail < right.Detail
		})
		finalizeCandidate(candidate, workloadMACs)
		if candidate.Strength == StrengthExactMAC {
			exactHostIDs = append(exactHostIDs, hostID)
		}
		result.Candidates = append(result.Candidates, *candidate)
	}
	sort.Ints(exactHostIDs)
	if len(exactHostIDs) == 1 {
		result.DeterministicExactHostID = exactHostIDs[0]
	}
	result.ExactAmbiguous = len(exactHostIDs) > 1

	sort.Slice(result.Candidates, func(i, j int) bool {
		left, right := result.Candidates[i], result.Candidates[j]
		if strengthRank(left.Strength) != strengthRank(right.Strength) {
			return strengthRank(left.Strength) < strengthRank(right.Strength)
		}
		if left.Active != right.Active {
			return left.Active
		}
		if left.Name != right.Name {
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
		return left.HostID < right.HostID
	})
	return result
}

func finalizeCandidate(candidate *Candidate, workloadMACs []string) {
	candidate.WorkloadMACs = append([]string(nil), workloadMACs...)
	addressSet := make(map[string]struct{})
	hasExact := false
	hasAddress := false
	hasName := false
	for _, evidence := range candidate.Evidence {
		switch evidence.Strength {
		case StrengthExactMAC:
			hasExact = true
		case StrengthAddress:
			hasAddress = true
			if evidence.MatchedValue != "" {
				addressSet[evidence.MatchedValue] = struct{}{}
			}
		case StrengthName:
			hasName = true
		}
	}
	candidate.MatchedAddresses = make([]string, 0, len(addressSet))
	for address := range addressSet {
		candidate.MatchedAddresses = append(candidate.MatchedAddresses, address)
	}
	sort.Strings(candidate.MatchedAddresses)

	candidate.PossibleIPConflict = hasAddress && !hasExact && candidate.Mac != "" && len(workloadMACs) > 0 && !containsString(workloadMACs, candidate.Mac)
	switch {
	case hasExact:
		candidate.Assessment = "exact-mac"
	case candidate.PossibleIPConflict:
		candidate.Assessment = "possible-ip-conflict"
	case hasAddress:
		candidate.Assessment = "address-only"
	case hasName:
		candidate.Assessment = "name-only"
	default:
		candidate.Assessment = "unknown"
	}
	candidate.EvidenceFingerprint = evidenceFingerprint(*candidate)
}

func evidenceFingerprint(candidate Candidate) string {
	parts := []string{
		candidate.Mac,
		candidate.Strength,
		candidate.Assessment,
		strings.Join(candidate.WorkloadMACs, ","),
	}
	evidenceParts := make([]string, 0, len(candidate.Evidence))
	for _, evidence := range candidate.Evidence {
		evidenceParts = append(evidenceParts, fmt.Sprintf("%s|%s|%t", evidence.Code, evidence.MatchedValue, evidence.Active))
	}
	sort.Strings(evidenceParts)
	parts = append(parts, evidenceParts...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func strengthRank(value string) int {
	switch value {
	case StrengthExactMAC:
		return 0
	case StrengthAddress:
		return 1
	case StrengthName:
		return 2
	default:
		return 3
	}
}

func normalizedAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if ip := net.ParseIP(value); ip != nil {
		return canonicalIP(ip)
	}
	ip, _, err := net.ParseCIDR(value)
	if err != nil {
		return ""
	}
	return canonicalIP(ip)
}

func canonicalIP(ip net.IP) string {
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return ip.String()
}

func normalizedName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".")
	return strings.Join(strings.Fields(value), " ")
}

func interfaceLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Workload interface"
	}
	return "Workload interface " + name
}

func joinedInterfaceLabel(names []string) string {
	normalized := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			name = "unnamed"
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	if len(normalized) == 1 {
		return "Workload interface " + normalized[0]
	}
	return "Workload interfaces " + strings.Join(normalized, ", ")
}
