package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/correlation"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

// HostIdentityCandidate is a read-only suggestion that two MAC identities may
// belong to the same physical device. It never implies an automatic merge.
type HostIdentityCandidate struct {
	Mac        string               `json:"mac"`
	HostID     int                  `json:"hostId"`
	Name       string               `json:"name"`
	DeviceType string               `json:"deviceType"`
	Exists     bool                 `json:"exists"`
	Active     bool                 `json:"active"`
	Score      int                  `json:"score"`
	Confidence correlation.Confidence `json:"confidence"`
	Reasons    []correlation.Reason `json:"reasons"`
}

// HostIdentityCandidatesResponse returns explainable correlation suggestions.
type HostIdentityCandidatesResponse struct {
	Mac        string                  `json:"mac"`
	Candidates []HostIdentityCandidate `json:"candidates"`
}

// getHostIdentityCandidates godoc
// @Summary      Get host identity correlation candidates
// @Description  Return read-only, explainable suggestions for MAC identities that may represent the same physical device. This endpoint never merges or mutates identities.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Success      200  {object}  HostIdentityCandidatesResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/identity/candidates [get]
func getHostIdentityCandidates(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	addressRows, err := gdb.SelectAllHostAddresses()
	if err != nil {
		slog.Error("Failed to load identity address observations for correlation", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load identity correlation data"})
		return
	}
	evidenceRows, err := gdb.SelectAllHostDiscoveryEvidence()
	if err != nil {
		slog.Error("Failed to load identity evidence for correlation", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load identity correlation data"})
		return
	}
	currentHosts, ok := gdb.Select("now")
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load current identities"})
		return
	}

	observations := buildCorrelationObservations(currentHosts, addressRows, evidenceRows)
	targetMAC := identity.MACKey(host.Mac)
	target := observations[targetMAC]
	target.Mac = targetMAC

	all := make([]correlation.IdentityObservation, 0, len(observations))
	for _, observation := range observations {
		all = append(all, observation)
	}

	currentByMAC := make(map[string]models.Host, len(currentHosts))
	for _, current := range currentHosts {
		currentByMAC[identity.MACKey(current.Mac)] = current
	}

	engineCandidates := correlation.Candidates(target, all)
	candidates := make([]HostIdentityCandidate, 0, len(engineCandidates))
	for _, candidate := range engineCandidates {
		item := HostIdentityCandidate{
			Mac:        candidate.Mac,
			Score:      candidate.Score,
			Confidence: candidate.Confidence,
			Reasons:    candidate.Reasons,
		}
		if current, exists := currentByMAC[candidate.Mac]; exists {
			item.HostID = current.ID
			item.Name = current.Name
			item.DeviceType = current.DeviceType
			item.Exists = true
			item.Active = current.Now == 1
		}
		candidates = append(candidates, item)
	}

	c.IndentedJSON(http.StatusOK, HostIdentityCandidatesResponse{
		Mac:        targetMAC,
		Candidates: candidates,
	})
}

func buildCorrelationObservations(currentHosts []models.Host, addressRows []models.HostAddress, evidenceRows []models.HostDiscoveryEvidence) map[string]correlation.IdentityObservation {
	observations := make(map[string]correlation.IdentityObservation)

	for _, host := range currentHosts {
		mac := identity.MACKey(host.Mac)
		if mac == "" {
			continue
		}
		observation := observations[mac]
		observation.Mac = mac
		observation.Active = observation.Active || host.Now == 1
		observations[mac] = observation
	}

	for _, row := range addressRows {
		mac := identity.MACKey(row.Mac)
		if mac == "" {
			continue
		}
		observation := observations[mac]
		observation.Mac = mac
		observation.Addresses = append(observation.Addresses, correlation.AddressObservation{
			Address: row.Address,
			Active:  row.Active,
		})
		observations[mac] = observation
	}

	for _, row := range evidenceRows {
		mac := identity.MACKey(row.Mac)
		if mac == "" {
			continue
		}
		observation := observations[mac]
		observation.Mac = mac
		observation.Evidence = append(observation.Evidence, correlation.EvidenceObservation{
			Kind:   row.Kind,
			Value:  row.Value,
			Active: row.Active,
		})
		observations[mac] = observation
	}

	return observations
}
