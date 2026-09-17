package identity

import (
	"errors"
	"net"
	"strings"
)

// MACType is the technical IEEE administration/group classification of a 48-bit MAC address.
type MACType string

const (
	MACTypeGloballyAdministered MACType = "globally-administered"
	MACTypeLocallyAdministered  MACType = "locally-administered"
	MACTypeMulticast            MACType = "multicast"
	MACTypeInvalid              MACType = "invalid"
)

var errInvalidMAC = errors.New("invalid 48-bit MAC address")

// NormalizeMAC returns a canonical upper-case, colon-separated 48-bit MAC address.
func NormalizeMAC(raw string) (string, error) {
	hardware, err := parse48BitMAC(raw)
	if err != nil {
		return "", err
	}

	return strings.ToUpper(hardware.String()), nil
}

// MACKey returns a stable comparison key without making invalid input fatal.
// Valid 48-bit addresses use the canonical representation; malformed values
// fall back to a trimmed upper-case key so legacy data remains comparable.
func MACKey(raw string) string {
	canonical, err := NormalizeMAC(raw)
	if err == nil {
		return canonical
	}

	return strings.ToUpper(strings.TrimSpace(raw))
}

// ClassifyMAC derives the technical address type from the I/G and U/L bits.
// A locally administered address is not, by itself, evidence that the address
// is randomized or privacy-related.
func ClassifyMAC(raw string) MACType {
	hardware, err := parse48BitMAC(raw)
	if err != nil {
		return MACTypeInvalid
	}

	firstOctet := hardware[0]
	if firstOctet&0x01 != 0 {
		return MACTypeMulticast
	}
	if firstOctet&0x02 != 0 {
		return MACTypeLocallyAdministered
	}

	return MACTypeGloballyAdministered
}

func parse48BitMAC(raw string) (net.HardwareAddr, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errInvalidMAC
	}

	hardware, err := net.ParseMAC(value)
	if err != nil || len(hardware) != 6 {
		return nil, errInvalidMAC
	}

	return hardware, nil
}
