package routines

import (
	"bytes"
	"net"
	"sort"
	"strings"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

// buildCompatibilityHostMap collapses scanner observations to the legacy one-row-per-MAC
// Host model without discarding the complete address observations recorded before it.
// The selected compatibility IP is stable across scanner output ordering.
func buildCompatibilityHostMap(foundHosts []models.Host) map[string]models.Host {
	currentIPByMAC := make(map[string]string)
	if currentHosts, ok := gdb.Select("now"); ok {
		for _, host := range currentHosts {
			currentIPByMAC[identity.MACKey(host.Mac)] = host.IP
		}
	}

	grouped := make(map[string][]models.Host)
	for _, host := range foundHosts {
		key := identity.MACKey(host.Mac)
		if canonical, err := identity.NormalizeMAC(host.Mac); err == nil {
			host.Mac = canonical
		}
		grouped[key] = append(grouped[key], host)
	}

	selected := make(map[string]models.Host, len(grouped))
	for key, candidates := range grouped {
		selected[key] = selectCompatibilityHost(candidates, currentIPByMAC[key])
	}
	return selected
}

func selectCompatibilityHost(candidates []models.Host, currentIP string) models.Host {
	if len(candidates) == 0 {
		return models.Host{}
	}

	ordered := append([]models.Host(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return compatibilityHostLess(ordered[i], ordered[j], currentIP)
	})

	selected := ordered[0]
	// Presence/lifecycle timestamps describe the MAC observation, not only the
	// selected compatibility address. Keep the newest observation time from the group.
	for _, candidate := range candidates {
		if candidate.Date > selected.Date {
			selected.Date = candidate.Date
		}
	}
	return selected
}

func compatibilityHostLess(left, right models.Host, currentIP string) bool {
	currentAddress, _, _, currentValid := compatibilityIPAddress(currentIP)
	leftAddress, leftBytes, leftFamily, leftValid := compatibilityIPAddress(left.IP)
	rightAddress, rightBytes, rightFamily, rightValid := compatibilityIPAddress(right.IP)

	leftCurrent := currentValid && leftValid && leftAddress == currentAddress
	rightCurrent := currentValid && rightValid && rightAddress == currentAddress
	if leftCurrent != rightCurrent {
		return leftCurrent
	}
	if leftValid != rightValid {
		return leftValid
	}
	if leftFamily != rightFamily {
		return leftFamily < rightFamily
	}
	if leftValid && !bytes.Equal(leftBytes, rightBytes) {
		return bytes.Compare(leftBytes, rightBytes) < 0
	}
	if !leftValid && leftAddress != rightAddress {
		return leftAddress < rightAddress
	}

	leftIface := strings.TrimSpace(left.Iface)
	rightIface := strings.TrimSpace(right.Iface)
	if leftIface != rightIface {
		return leftIface < rightIface
	}
	if left.Date != right.Date {
		return left.Date > right.Date
	}
	return strings.TrimSpace(left.Hw) < strings.TrimSpace(right.Hw)
}

// compatibilityIPAddress returns a canonical address plus bytes suitable for
// deterministic numeric ordering. family is 0 for IPv4, 1 for IPv6, 2 invalid.
func compatibilityIPAddress(value string) (address string, raw []byte, family int, valid bool) {
	trimmed := strings.TrimSpace(value)
	ip := net.ParseIP(trimmed)
	if ip == nil {
		return trimmed, nil, 2, false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String(), append([]byte(nil), ipv4...), 0, true
	}
	ipv6 := ip.To16()
	if ipv6 == nil {
		return trimmed, nil, 2, false
	}
	return ip.String(), append([]byte(nil), ipv6...), 1, true
}
