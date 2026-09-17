package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

var correlationDecisionNow = func() time.Time {
	return time.Now().UTC()
}

type correlationDecisionRequest struct {
	Decision string `json:"decision"`
}

// IdentityCorrelationDecisionResponse exposes explicit user decisions without
// collapsing either MAC identity into the other.
type IdentityCorrelationDecisionResponse struct {
	Mac        string `json:"mac"`
	Decision   string `json:"decision"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	HostID     int    `json:"hostId"`
	Name       string `json:"name"`
	DeviceType string `json:"deviceType"`
	Exists     bool   `json:"exists"`
	Active     bool   `json:"active"`
}

type HostIdentityDecisionsResponse struct {
	Mac       string                                `json:"mac"`
	Decisions []IdentityCorrelationDecisionResponse `json:"decisions"`
}

// getHostIdentityDecisions godoc
// @Summary      Get explicit identity correlation decisions
// @Description  Return user-confirmed or user-rejected MAC relationships for a host. Decisions do not merge host observations, events, presence, or lifecycle history.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Success      200  {object}  HostIdentityDecisionsResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/identity/decisions [get]
func getHostIdentityDecisions(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	rows, err := gdb.SelectIdentityCorrelationDecisionsForMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load identity correlation decisions", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load identity correlation decisions"})
		return
	}

	currentHosts, ok := gdb.Select("now")
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load current identities"})
		return
	}
	currentByMAC := currentHostsByMAC(currentHosts)
	targetMAC := identity.MACKey(host.Mac)

	decisions := make([]IdentityCorrelationDecisionResponse, 0, len(rows))
	for _, row := range rows {
		otherMAC := row.MacA
		if otherMAC == targetMAC {
			otherMAC = row.MacB
		}
		item := IdentityCorrelationDecisionResponse{
			Mac:       otherMAC,
			Decision:  row.Decision,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
		if current, exists := currentByMAC[otherMAC]; exists {
			item.HostID = current.ID
			item.Name = current.Name
			item.DeviceType = current.DeviceType
			item.Exists = true
			item.Active = current.Now == 1
		}
		decisions = append(decisions, item)
	}

	c.IndentedJSON(http.StatusOK, HostIdentityDecisionsResponse{
		Mac:       targetMAC,
		Decisions: decisions,
	})
}

// setHostIdentityDecision godoc
// @Summary      Set an identity correlation decision
// @Description  Confirm or reject that another observed MAC identity belongs to the same physical device. This records user intent only and never merges historical observations.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id       path      string                      true  "Host ID"
// @Param        mac      path      string                      true  "Other observed MAC"
// @Param        request  body      correlationDecisionRequest  true  "Correlation decision"
// @Success      200      {object}  IdentityCorrelationDecisionResponse
// @Failure      400      {object}  map[string]string
// @Failure      404      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /host/{id}/identity/decisions/{mac} [put]
func setHostIdentityDecision(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	otherMAC, err := identity.NormalizeMAC(c.Param("mac"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid correlation MAC"})
		return
	}
	targetMAC, err := identity.NormalizeMAC(host.Mac)
	if err != nil || targetMAC == otherMAC {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "correlation requires two different valid MAC identities"})
		return
	}
	if !observedIdentityExists(otherMAC) {
		c.IndentedJSON(http.StatusNotFound, gin.H{"error": "correlation MAC has not been observed"})
		return
	}

	var req correlationDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid correlation decision request"})
		return
	}
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if decision != models.IdentityCorrelationConfirmed && decision != models.IdentityCorrelationRejected {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "correlation decision must be confirmed or rejected"})
		return
	}

	changedAt := correlationDecisionNow().Format("2006-01-02 15:04:05")
	row, err := gdb.SetIdentityCorrelationDecision(targetMAC, otherMAC, decision, changedAt)
	if err != nil {
		slog.Error("Failed to persist identity correlation decision", "mac", targetMAC, "otherMac", otherMAC, "decision", decision, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to persist identity correlation decision"})
		return
	}

	response := IdentityCorrelationDecisionResponse{
		Mac:       otherMAC,
		Decision:  row.Decision,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if current, exists := currentHostByMAC(otherMAC); exists {
		response.HostID = current.ID
		response.Name = current.Name
		response.DeviceType = current.DeviceType
		response.Exists = true
		response.Active = current.Now == 1
	}
	c.IndentedJSON(http.StatusOK, response)
}

// deleteHostIdentityDecision godoc
// @Summary      Clear an identity correlation decision
// @Description  Remove the user's explicit confirm/reject decision for a MAC pair without modifying either identity or its historical observations.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Param        mac  path      string  true  "Other MAC"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/identity/decisions/{mac} [delete]
func deleteHostIdentityDecision(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	otherMAC, err := identity.NormalizeMAC(c.Param("mac"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid correlation MAC"})
		return
	}
	if identity.MACKey(host.Mac) == otherMAC {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "correlation requires two different MAC identities"})
		return
	}

	if err := gdb.DeleteIdentityCorrelationDecision(host.Mac, otherMAC); err != nil {
		slog.Error("Failed to clear identity correlation decision", "mac", host.Mac, "otherMac", otherMAC, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to clear identity correlation decision"})
		return
	}
	c.Status(http.StatusNoContent)
}

func observedIdentityExists(mac string) bool {
	if _, exists := currentHostByMAC(mac); exists {
		return true
	}
	addresses, err := gdb.SelectHostAddressesByMAC(mac)
	if err == nil && len(addresses) > 0 {
		return true
	}
	evidence, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	return err == nil && len(evidence) > 0
}

func currentHostByMAC(mac string) (models.Host, bool) {
	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) > 0 {
		return hosts[0], true
	}
	return models.Host{}, false
}

func currentHostsByMAC(hosts []models.Host) map[string]models.Host {
	result := make(map[string]models.Host, len(hosts))
	for _, host := range hosts {
		mac := identity.MACKey(host.Mac)
		if mac != "" {
			result[mac] = host
		}
	}
	return result
}
