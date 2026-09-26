package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/discovery"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

var (
	identificationHostnameDiscovery = discovery.LocalHostnames
	identificationRefreshNow        = time.Now
)

type IdentificationNameRefreshSource struct {
	Source string   `json:"source"`
	Values []string `json:"values"`
}

type IdentificationNameRefreshResponse struct {
	RefreshedAt string                            `json:"refreshedAt"`
	Sources     []IdentificationNameRefreshSource `json:"sources"`
}

// refreshHostIdentificationNames performs an explicit, bounded local hostname
// refresh for one Host. It never runs ARP/SSDP discovery and never changes
// presence, lifecycle, managed inventory, or LastSeen host state.
func refreshHostIdentificationNames(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	if strings.TrimSpace(host.IP) == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no current IP"})
		return
	}

	observations := identificationHostnameDiscovery(c.Request.Context(), host.IP)
	if c.Request.Context().Err() != nil {
		return
	}

	currentHost, err := getHostByID(c.Param("id"))
	if err != nil || currentHost.ID < 1 || !sameHostProbeBinding(host, currentHost) {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": "host address changed during name refresh; results were not recorded"})
		return
	}
	host = currentHost

	observedAt := identificationRefreshNow().UTC().Format(models.HostEventDateLayout)
	response := IdentificationNameRefreshResponse{
		RefreshedAt: observedAt,
		Sources:     make([]IdentificationNameRefreshSource, 0, len(observations)),
	}

	for _, observation := range observations {
		if !identificationHostnameSourceAllowed(observation.Source) || len(observation.Values) == 0 {
			continue
		}

		values := make([]string, 0, len(observation.Values))
		for _, value := range observation.Values {
			if value = boundedIdentificationString(value); value != "" {
				values = append(values, value)
			}
		}
		if len(values) == 0 {
			continue
		}

		if err := gdb.RecordHostDiscoveryEvidence(
			host.Mac,
			host.IP,
			observation.Source,
			models.DiscoveryKindHostname,
			values,
			observedAt,
		); err != nil {
			slog.Error(
				"Failed to record targeted identification hostname evidence",
				"hostID", host.ID,
				"source", observation.Source,
				"err", err,
			)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to record refreshed name evidence"})
			return
		}

		response.Sources = append(response.Sources, IdentificationNameRefreshSource{
			Source: observation.Source,
			Values: values,
		})
	}

	c.IndentedJSON(http.StatusOK, response)
}

func identificationHostnameSourceAllowed(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case models.DiscoverySourceReverseDNS,
		models.DiscoverySourceSystemResolver,
		models.DiscoverySourceMDNS:
		return true
	default:
		return false
	}
}

