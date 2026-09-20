package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

// InfrastructureWorkloadMembershipResponse exposes the reverse side of an
// existing workload-to-Host MATCHES relation so a LANnventory Host can show
// which Proxmox VM/LXC it represents and which hypervisor owns that workload.
type InfrastructureWorkloadMembershipResponse struct {
	WorkloadID      uint   `json:"workloadId"`
	NativeID        string `json:"nativeId"`
	WorkloadType    string `json:"workloadType"`
	WorkloadName    string `json:"workloadName"`
	WorkloadStatus  string `json:"workloadStatus"`
	RetiredAt       string `json:"retiredAt"`
	HostID          int    `json:"hostId"`
	HostMac         string `json:"hostMac"`
	LinkSource      string `json:"linkSource"`
	HypervisorHostID int   `json:"hypervisorHostId"`
	HypervisorMac    string `json:"hypervisorMac"`
	HypervisorName   string `json:"hypervisorName"`
	HypervisorIP     string `json:"hypervisorIp"`
}

// getInfrastructureWorkloadMemberships godoc
// @Summary      Get workload-to-Host memberships
// @Description  Return read-only reverse projections for persisted workload MATCHES relations. Optional hostId limits the response to one current LANnventory Host.
// @Tags         hosts
// @Produce      json
// @Param        hostId  query     int  false  "Current Host ID"
// @Success      200     {array}   InfrastructureWorkloadMembershipResponse
// @Failure      400     {object}  map[string]string
// @Failure      500     {object}  map[string]string
// @Router       /infrastructure/workload-memberships [get]
func getInfrastructureWorkloadMemberships(c *gin.Context) {
	filterHostID := 0
	if raw := strings.TrimSpace(c.Query("hostId")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid hostId"})
			return
		}
		filterHostID = parsed
	}

	rows, err := gdb.SelectInfrastructureWorkloadMemberships()
	if err != nil {
		slog.Error("Failed to load workload memberships", "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load workload memberships"})
		return
	}
	hosts, ok := gdb.Select("now")
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load current hosts"})
		return
	}

	hostByID := make(map[int]models.Host, len(hosts))
	hostByMAC := make(map[string]models.Host, len(hosts))
	for _, host := range hosts {
		hostByID[host.ID] = host
		if mac := identity.MACKey(host.Mac); mac != "" {
			hostByMAC[mac] = host
		}
	}

	response := make([]InfrastructureWorkloadMembershipResponse, 0, len(rows))
	for _, row := range rows {
		link := row.Link
		if filterHostID > 0 && link.HostID != filterHostID {
			continue
		}
		target, exists := hostByID[link.HostID]
		if !exists || !strings.EqualFold(strings.TrimSpace(target.Mac), strings.TrimSpace(link.HostMac)) {
			continue
		}
		parent := hostByMAC[identity.MACKey(row.Workload.HypervisorMac)]
		response = append(response, InfrastructureWorkloadMembershipResponse{
			WorkloadID:       row.Workload.ID,
			NativeID:         row.Workload.NativeID,
			WorkloadType:     row.Workload.WorkloadType,
			WorkloadName:     row.Workload.Name,
			WorkloadStatus:   row.Workload.Status,
			RetiredAt:        row.Workload.RetiredAt,
			HostID:           target.ID,
			HostMac:          target.Mac,
			LinkSource:       link.LinkSource,
			HypervisorHostID: parent.ID,
			HypervisorMac:    row.Workload.HypervisorMac,
			HypervisorName:   parent.Name,
			HypervisorIP:     parent.IP,
		})
	}

	c.IndentedJSON(http.StatusOK, response)
}
