package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/workloadmatch"
)

type WorkloadMatchEvidenceDoc struct {
	Code         string `json:"code"`
	Detail       string `json:"detail"`
	Strength     string `json:"strength"`
	Active       bool   `json:"active"`
	MatchedValue string `json:"matchedValue,omitempty"`
	FirstSeen    string `json:"firstSeen,omitempty"`
	LastSeen     string `json:"lastSeen,omitempty"`
}

type WorkloadMatchCandidateDoc struct {
	HostID              int                        `json:"hostId"`
	Mac                 string                     `json:"mac"`
	Name                string                     `json:"name"`
	IP                  string                     `json:"ip"`
	DeviceType          string                     `json:"deviceType"`
	Active              bool                       `json:"active"`
	Strength            string                     `json:"strength"`
	Assessment          string                     `json:"assessment"`
	PossibleIPConflict  bool                       `json:"possibleIpConflict"`
	MatchedAddresses    []string                   `json:"matchedAddresses"`
	WorkloadMACs        []string                   `json:"workloadMacs"`
	EvidenceFingerprint string                     `json:"evidenceFingerprint"`
	Rejected            bool                       `json:"rejected"`
	Evidence            []WorkloadMatchEvidenceDoc `json:"evidence"`
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
		if err := decorateRejectedWorkloadCandidates(record.Workload.ID, &result); err != nil {
			slog.Error("Failed to load workload candidate rejections", "workloadID", record.Workload.ID, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load workload candidate decisions"})
			return
		}
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

type workloadCandidateRejectionRequest struct {
	EvidenceFingerprint string `json:"evidenceFingerprint"`
}

// setHostInfrastructureWorkloadCandidateRejection godoc
// @Summary      Reject a weak workload Host candidate
// @Description  Persist a non-destructive "Not this Host" decision for the current weak address/name evidence. Exact-MAC evidence cannot be rejected here, and changed evidence is re-evaluated automatically.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id               path      string                              true  "Proxmox Host ID"
// @Param        workloadId       path      string                              true  "Workload ID"
// @Param        candidateHostId  path      string                              true  "Candidate Host ID"
// @Param        body             body      workloadCandidateRejectionRequest   true  "Reviewed evidence"
// @Success      200              {object}  WorkloadMatchCandidateDoc
// @Failure      400              {object}  map[string]string
// @Failure      409              {object}  map[string]string
// @Failure      500              {object}  map[string]string
// @Router       /host/{id}/workloads/{workloadId}/match-rejections/{candidateHostId} [put]
func setHostInfrastructureWorkloadCandidateRejection(c *gin.Context) {
	hypervisor, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}
	record, ok := workloadRecordFromRequest(c, hypervisor)
	if !ok {
		return
	}
	candidateHostID, err := strconv.Atoi(strings.TrimSpace(c.Param("candidateHostId")))
	if err != nil || candidateHostID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid candidate host id"})
		return
	}

	var payload workloadCandidateRejectionRequest
	if !decodeStrictWorkloadJSON(c, &payload) {
		return
	}
	payload.EvidenceFingerprint = strings.TrimSpace(payload.EvidenceFingerprint)
	if payload.EvidenceFingerprint == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "evidenceFingerprint is required"})
		return
	}

	result, err := currentWorkloadMatch(record, hypervisor.Mac)
	if err != nil {
		slog.Error("Failed to recompute workload candidate", "workloadID", record.Workload.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to recompute workload candidate"})
		return
	}
	candidate, found := workloadCandidateByHostID(result, candidateHostID)
	if !found {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": "candidate is no longer supported by current evidence"})
		return
	}
	if candidate.Strength == workloadmatch.StrengthExactMAC {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "exact MAC candidates cannot be rejected as weak matches"})
		return
	}
	if candidate.EvidenceFingerprint != payload.EvidenceFingerprint {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": "candidate evidence changed; review the refreshed match before rejecting it"})
		return
	}

	target, err := gdb.SelectHostWithMetadataByID(candidateHostID)
	if err != nil || target.ID < 1 || !strings.EqualFold(strings.TrimSpace(target.Mac), strings.TrimSpace(candidate.Mac)) {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": "candidate Host identity changed; refresh workload matches"})
		return
	}
	if _, err := gdb.SetInfrastructureWorkloadCandidateRejection(
		record.Workload.ID,
		target,
		candidate.EvidenceFingerprint,
		candidate.Strength,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		slog.Error("Failed to reject workload candidate", "workloadID", record.Workload.ID, "candidateHostID", candidateHostID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to save workload candidate rejection"})
		return
	}
	candidate.Rejected = true
	c.IndentedJSON(http.StatusOK, candidate)
}

// deleteHostInfrastructureWorkloadCandidateRejection godoc
// @Summary      Clear a workload Host candidate rejection
// @Description  Remove only the explicit "Not this Host" decision. Workload, Host identity/history, and links remain unchanged.
// @Tags         hosts
// @Produce      json
// @Param        id               path      string  true  "Proxmox Host ID"
// @Param        workloadId       path      string  true  "Workload ID"
// @Param        candidateHostId  path      string  true  "Candidate Host ID"
// @Success      204
// @Failure      400              {object}  map[string]string
// @Failure      500              {object}  map[string]string
// @Router       /host/{id}/workloads/{workloadId}/match-rejections/{candidateHostId} [delete]
func deleteHostInfrastructureWorkloadCandidateRejection(c *gin.Context) {
	hypervisor, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}
	record, ok := workloadRecordFromRequest(c, hypervisor)
	if !ok {
		return
	}
	candidateHostID, err := strconv.Atoi(strings.TrimSpace(c.Param("candidateHostId")))
	if err != nil || candidateHostID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid candidate host id"})
		return
	}
	if err := gdb.DeleteInfrastructureWorkloadCandidateRejection(record.Workload.ID, candidateHostID); err != nil {
		slog.Error("Failed to clear workload candidate rejection", "workloadID", record.Workload.ID, "candidateHostID", candidateHostID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to clear workload candidate rejection"})
		return
	}
	c.Status(http.StatusNoContent)
}

func currentWorkloadMatch(record models.InfrastructureWorkloadRecord, hypervisorMAC string) (workloadmatch.Result, error) {
	hosts, ok := gdb.Select("now")
	if !ok {
		return workloadmatch.Result{}, errors.New("failed to load current hosts")
	}
	addresses, err := gdb.SelectAllHostAddresses()
	if err != nil {
		return workloadmatch.Result{}, err
	}
	discovery, err := gdb.SelectAllHostDiscoveryEvidence()
	if err != nil {
		return workloadmatch.Result{}, err
	}
	return workloadmatch.Match(record, hypervisorMAC, hosts, addresses, discovery), nil
}

func workloadCandidateByHostID(result workloadmatch.Result, hostID int) (workloadmatch.Candidate, bool) {
	for _, candidate := range result.Candidates {
		if candidate.HostID == hostID {
			return candidate, true
		}
	}
	return workloadmatch.Candidate{}, false
}

func decorateRejectedWorkloadCandidates(workloadID uint, result *workloadmatch.Result) error {
	rows, err := gdb.SelectInfrastructureWorkloadCandidateRejections(workloadID)
	if err != nil {
		return err
	}
	byHostID := make(map[int]models.InfrastructureWorkloadCandidateRejection, len(rows))
	for _, row := range rows {
		byHostID[row.HostID] = row
	}
	for index := range result.Candidates {
		candidate := &result.Candidates[index]
		if candidate.Strength == workloadmatch.StrengthExactMAC {
			continue
		}
		row, exists := byHostID[candidate.HostID]
		if !exists {
			continue
		}
		candidate.Rejected = strings.EqualFold(row.HostMac, candidate.Mac) &&
			row.EvidenceFingerprint == candidate.EvidenceFingerprint
	}
	return nil
}
