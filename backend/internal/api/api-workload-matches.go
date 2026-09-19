package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/workloadmatch"
)

type WorkloadMatchEvidenceDoc struct {
	Code     string `json:"code"`
	Detail   string `json:"detail"`
	Strength string `json:"strength"`
	Active   bool   `json:"active"`
}

type WorkloadMatchCandidateDoc struct {
	HostID     int                        `json:"hostId"`
	Mac        string                     `json:"mac"`
	Name       string                     `json:"name"`
	IP         string                     `json:"ip"`
	DeviceType string                     `json:"deviceType"`
	Active     bool                       `json:"active"`
	Strength   string                     `json:"strength"`
	Evidence   []WorkloadMatchEvidenceDoc `json:"evidence"`
}

type InfrastructureWorkloadMatchResponseDoc struct {
	WorkloadID               uint                                   `json:"workloadId"`
	NativeID                 string                                 `json:"nativeId"`
	WorkloadType             string                                 `json:"workloadType"`
	Name                     string                                 `json:"name"`
	CurrentLink              *models.InfrastructureWorkloadHostLink `json:"currentLink"`
	DeterministicExactHostID int                                    `json:"deterministicExactHostId,omitempty"`
	ExactAmbiguous           bool                                   `json:"exactAmbiguous"`
	Candidates               []WorkloadMatchCandidateDoc            `json:"candidates"`
}

type InfrastructureWorkloadMatchResponse struct {
	WorkloadID               uint                                   `json:"workloadId"`
	NativeID                 string                                 `json:"nativeId"`
	WorkloadType             string                                 `json:"workloadType"`
	Name                     string                                 `json:"name"`
	CurrentLink              *models.InfrastructureWorkloadHostLink `json:"currentLink"`
	DeterministicExactHostID int                                    `json:"deterministicExactHostId,omitempty"`
	ExactAmbiguous           bool                                   `json:"exactAmbiguous"`
	Candidates               []workloadmatch.Candidate              `json:"candidates"`
}

// getHostInfrastructureWorkloadMatches godoc
// @Summary      Get Proxmox workload Host match candidates
// @Description  Return explainable workload-to-Host candidates. Exact MAC is the only automatic evidence; address and name evidence remain suggestions. This endpoint never mutates links.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string                                  true  "Proxmox Host ID"
// @Success      200  {array}   InfrastructureWorkloadMatchResponseDoc
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/workload-matches [get]
func getHostInfrastructureWorkloadMatches(c *gin.Context) {
	hypervisor, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	workloads, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(hypervisor.Mac)
	if err != nil {
		slog.Error("Failed to load workloads for matching", "hostID", hypervisor.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load workloads for matching"})
		return
	}
	hosts, ok := gdb.Select("now")
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load current hosts for matching"})
		return
	}
	addresses, err := gdb.SelectAllHostAddresses()
	if err != nil {
		slog.Error("Failed to load address observations for workload matching", "hostID", hypervisor.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load address evidence for matching"})
		return
	}
	discovery, err := gdb.SelectAllHostDiscoveryEvidence()
	if err != nil {
		slog.Error("Failed to load discovery evidence for workload matching", "hostID", hypervisor.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load discovery evidence for matching"})
		return
	}

	response := make([]InfrastructureWorkloadMatchResponse, 0, len(workloads))
	for _, record := range workloads {
		result := workloadmatch.Match(record, hypervisor.Mac, hosts, addresses, discovery)
		response = append(response, InfrastructureWorkloadMatchResponse{
			WorkloadID:               record.Workload.ID,
			NativeID:                 record.Workload.NativeID,
			WorkloadType:             record.Workload.WorkloadType,
			Name:                     record.Workload.Name,
			CurrentLink:              record.Link,
			DeterministicExactHostID: result.DeterministicExactHostID,
			ExactAmbiguous:           result.ExactAmbiguous,
			Candidates:               result.Candidates,
		})
	}
	c.IndentedJSON(http.StatusOK, response)
}
