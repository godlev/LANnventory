package servicescan

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

const MaxScheduledPorts = 256

var (
	ErrInvalidPort  = errors.New("invalid service scan port")
	ErrTooManyPorts = errors.New("too many scheduled service scan ports")
)

// NormalizePorts validates, de-duplicates and sorts scheduled TCP ports.
func NormalizePorts(ports []int) ([]int, error) {
	seen := make(map[int]struct{}, len(ports))
	normalized := make([]int, 0, len(ports))

	for _, port := range ports {
		if port < 1 || port > 65535 {
			return nil, ErrInvalidPort
		}
		if _, exists := seen[port]; exists {
			continue
		}
		seen[port] = struct{}{}
		normalized = append(normalized, port)
	}

	if len(normalized) > MaxScheduledPorts {
		return nil, ErrTooManyPorts
	}

	sort.Ints(normalized)
	return normalized, nil
}

// DecodePortsJSON decodes persisted settings and applies the same validation
// used by the API before a scheduled scan can run.
func DecodePortsJSON(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []int{}, nil
	}

	var ports []int
	if err := json.Unmarshal([]byte(value), &ports); err != nil {
		return nil, err
	}
	return NormalizePorts(ports)
}

// EncodePortsJSON returns canonical persisted JSON plus normalized ports.
func EncodePortsJSON(ports []int) (string, []int, error) {
	normalized, err := NormalizePorts(ports)
	if err != nil {
		return "", nil, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", nil, err
	}
	return string(encoded), normalized, nil
}

// CanonicalPortsJSON validates and canonicalizes an existing JSON representation.
func CanonicalPortsJSON(value string) (string, []int, error) {
	ports, err := DecodePortsJSON(value)
	if err != nil {
		return "", nil, err
	}
	return EncodePortsJSON(ports)
}
