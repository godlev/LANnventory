package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

// InfrastructureWorkloadSummaryResponse exposes current workload counts for one
// hypervisor Host without projecting workloads into the Host table.
type InfrastructureWorkloadSummaryResponse struct {
	HypervisorHostID int    `json:"hypervisorHostId"`
	HypervisorMac    string `json:"hypervisorMac"`
	HypervisorName   string `json:"hypervisorName"`
	HypervisorIP     string `json:"hypervisorIp"`
	Platform         string `json:"platform"`
	VMCount          int    `json:"vmCount"`
	ContainerCount   int    `json:"containerCount"`
	TotalCount       int    `json:"totalCount"`
}

// getInfrastructureWorkloadSummaries godoc
// @Summary      Get current workload counts by hypervisor
// @Description  Return read-only VM/container counts from current non-retired workload inventory. Running and stopped workloads both count.
// @Tags         hosts
// @Produce      json
// @Success      200  {array}   InfrastructureWorkloadSummaryResponse
// @Failure      500  {object}  map[string]string
// @Router       /infrastructure/workload-summaries [get]
func getInfrastructureWorkloadSummaries(c *gin.Context) {
	rows, err := gdb.SelectInfrastructureWorkloadSummaries()
	if err != nil {
		slog.Error("Failed to load workload summaries", "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load workload summaries"})
		return
	}

	hosts, ok := gdb.Select("now")
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load current hosts"})
		return
	}

	hostByMAC := make(map[string]models.Host, len(hosts))
	for _, host := range hosts {
		if mac, err := identity.NormalizeMAC(host.Mac); err == nil {
			hostByMAC[mac] = host
		}
	}

	response := make([]InfrastructureWorkloadSummaryResponse, 0, len(rows))
	for _, row := range rows {
		mac, err := identity.NormalizeMAC(row.HypervisorMac)
		if err != nil {
			continue
		}
		host, exists := hostByMAC[mac]
		if !exists {
			continue
		}

		platform := ""
		profile, found, err := gdb.SelectHypervisorProfileByMAC(mac)
		if err != nil {
			slog.Error("Failed to load hypervisor profile for workload summary", "mac", mac, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load hypervisor profile"})
			return
		}
		if found {
			platform = profile.Platform
		}

		response = append(response, InfrastructureWorkloadSummaryResponse{
			HypervisorHostID: host.ID,
			HypervisorMac:    mac,
			HypervisorName:   host.Name,
			HypervisorIP:     host.IP,
			Platform:         platform,
			VMCount:          row.VMCount,
			ContainerCount:   row.ContainerCount,
			TotalCount:       row.VMCount + row.ContainerCount,
		})
	}

	c.IndentedJSON(http.StatusOK, response)
}
