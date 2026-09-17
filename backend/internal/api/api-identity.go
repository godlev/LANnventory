package api

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

// AddressMACObservation describes one MAC identity observed using an IP address.
type AddressMACObservation struct {
	Mac       string `json:"mac"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	Active    bool   `json:"active"`
}

// HostIdentityAddress describes one address used by a host MAC together with
// the complete retained MAC history for that address.
type HostIdentityAddress struct {
	Address    string                  `json:"address"`
	Family     string                  `json:"family"`
	Iface      string                  `json:"iface"`
	FirstSeen  string                  `json:"firstSeen"`
	LastSeen   string                  `json:"lastSeen"`
	Active     bool                    `json:"active"`
	MacHistory []AddressMACObservation `json:"macHistory"`
}

// HostIdentityResponse keeps user-managed inventory separate from discovered
// identity evidence while exposing the retained MAC <-> IP observation graph.
type HostIdentityResponse struct {
	Mac         string                         `json:"mac"`
	Addresses   []HostIdentityAddress          `json:"addresses"`
	Evidence    []models.HostDiscoveryEvidence `json:"evidence"`
	DataSources []string                       `json:"dataSources"`
}

// AddressIdentityResponse exposes the reverse IP -> MAC observation history.
type AddressIdentityResponse struct {
	Address    string                  `json:"address"`
	MacHistory []AddressMACObservation `json:"macHistory"`
}

// getHostIdentity godoc
// @Summary      Get host identity observations
// @Description  Return retained addresses, reverse address-to-MAC history, discovery evidence, and discovery sources for a host without changing user-managed inventory fields.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Success      200  {object}  HostIdentityResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/identity [get]
func getHostIdentity(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	addresses, err := gdb.SelectHostAddressesByMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load host address identity", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load host identity"})
		return
	}

	evidence, err := gdb.SelectHostDiscoveryEvidenceByMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load host discovery evidence", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load host identity"})
		return
	}

	identityAddresses := make([]HostIdentityAddress, 0, len(addresses))
	for _, address := range addresses {
		macRows, err := gdb.SelectHostAddressesByAddress(address.Address)
		if err != nil {
			slog.Error("Failed to load reverse address identity", "address", address.Address, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load host identity"})
			return
		}
		identityAddresses = append(identityAddresses, HostIdentityAddress{
			Address:    address.Address,
			Family:     address.Family,
			Iface:      address.Iface,
			FirstSeen:  address.FirstSeen,
			LastSeen:   address.LastSeen,
			Active:     address.Active,
			MacHistory: addressMACHistory(macRows),
		})
	}

	canonicalMAC := host.Mac
	if normalized, normalizeErr := identity.NormalizeMAC(host.Mac); normalizeErr == nil {
		canonicalMAC = normalized
	}

	c.IndentedJSON(http.StatusOK, HostIdentityResponse{
		Mac:         canonicalMAC,
		Addresses:   identityAddresses,
		Evidence:    evidence,
		DataSources: discoverySources(evidence),
	})
}

// getAddressIdentity godoc
// @Summary      Get address identity observations
// @Description  Return every retained MAC identity observed using one IP address, including first/last seen timestamps.
// @Tags         hosts
// @Produce      json
// @Param        address  query     string  true  "IPv4 or IPv6 address"
// @Success      200      {object}  AddressIdentityResponse
// @Failure      400      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /identity/address [get]
func getAddressIdentity(c *gin.Context) {
	address, ok := canonicalIdentityAddress(c.Query("address"))
	if !ok {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
		return
	}

	rows, err := gdb.SelectHostAddressesByAddress(address)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid ip") {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
			return
		}
		slog.Error("Failed to load address identity", "address", address, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load address identity"})
		return
	}

	c.IndentedJSON(http.StatusOK, AddressIdentityResponse{
		Address:    address,
		MacHistory: addressMACHistory(rows),
	})
}

func addressMACHistory(rows []models.HostAddress) []AddressMACObservation {
	result := make([]AddressMACObservation, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		mac := row.Mac
		if normalized, err := identity.NormalizeMAC(row.Mac); err == nil {
			mac = normalized
		}
		if _, exists := seen[mac]; exists {
			continue
		}
		seen[mac] = struct{}{}
		result = append(result, AddressMACObservation{
			Mac:       mac,
			FirstSeen: row.FirstSeen,
			LastSeen:  row.LastSeen,
			Active:    row.Active,
		})
	}
	return result
}

func discoverySources(evidence []models.HostDiscoveryEvidence) []string {
	seen := make(map[string]struct{}, len(evidence))
	for _, item := range evidence {
		source := strings.TrimSpace(item.Source)
		if source != "" {
			seen[source] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for source := range seen {
		result = append(result, source)
	}
	sort.Strings(result)
	return result
}

func canonicalIdentityAddress(value string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return "", false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String(), true
	}
	return ip.String(), true
}

var _ = errors.Is
