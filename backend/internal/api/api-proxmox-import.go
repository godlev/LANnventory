package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

const maxProxmoxImportBodyBytes int64 = 2 * 1024 * 1024

type ProxmoxImportApplyRequest struct {
	PreviewToken string                   `json:"previewToken"`
	Confirmed    bool                     `json:"confirmed"`
	Snapshot     proxmoxsnapshot.Snapshot `json:"snapshot"`
}

type ProxmoxImportApplyResponse struct {
	Applied    bool                         `json:"applied"`
	ImportedAt string                       `json:"importedAt"`
	Summary    proxmoximport.PreviewSummary `json:"summary"`
}

// previewProxmoxScriptImport godoc
// @Summary      Preview Proxmox script import
// @Description  Strictly validate a read-only collector snapshot and return a deterministic diff. This endpoint never persists the snapshot.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                    true  "Proxmox Host ID"
// @Param        body  body      proxmoxsnapshot.Snapshot  true  "Collector snapshot"
// @Success      200   {object}  proxmoximport.Preview
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/proxmox/import/preview [post]
func previewProxmoxScriptImport(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	var snapshot proxmoxsnapshot.Snapshot
	if !decodeStrictProxmoxImportJSON(c, &snapshot) {
		return
	}

	current, err := loadProxmoxImportCurrentState(host.Mac)
	if err != nil {
		slog.Error("Failed to load Proxmox import state", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox import state"})
		return
	}

	preview, err := proxmoximport.BuildPreview(host.Mac, snapshot, current)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, preview)
}

// applyProxmoxScriptImport godoc
// @Summary      Apply confirmed Proxmox script import
// @Description  Revalidate a previously previewed snapshot and atomically apply it only when the preview token still matches current persisted state.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "Proxmox Host ID"
// @Param        body  body      ProxmoxImportApplyRequest  true  "Confirmed preview token and collector snapshot"
// @Success      200   {object}  ProxmoxImportApplyResponse
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/proxmox/import/apply [post]
func applyProxmoxScriptImport(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	var payload ProxmoxImportApplyRequest
	if !decodeStrictProxmoxImportJSON(c, &payload) {
		return
	}
	payload.PreviewToken = strings.TrimSpace(payload.PreviewToken)
	if !payload.Confirmed {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "explicit confirmation is required"})
		return
	}
	if len(payload.PreviewToken) != 64 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "valid previewToken is required"})
		return
	}

	normalized, err := proxmoximport.ValidateAndNormalize(payload.Snapshot)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, err := loadProxmoxImportCurrentState(host.Mac)
	if err != nil {
		slog.Error("Failed to load current Proxmox import state", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox import state"})
		return
	}
	preview, err := proxmoximport.BuildPreview(host.Mac, normalized, current)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.PreviewToken != preview.PreviewToken {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": "preview is stale; generate a new preview before applying"})
		return
	}
	if !preview.ApplyAllowed {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "preview cannot be applied until blocking issues are resolved"})
		return
	}

	inputs, err := proxmoximport.SnapshotWorkloadInputs(normalized)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	importedAt := time.Now().UTC().Format(time.RFC3339)
	state, err := proxmoximport.SourceStateFromSnapshot(host.Mac, normalized, preview.SnapshotDigest, importedAt)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := gdb.ApplyProxmoxScriptImport(host.Mac, state, inputs); err != nil {
		if errors.Is(err, gdb.ErrInfrastructureWorkloadSourceConflict) {
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "workload state changed since preview; generate a new preview"})
			return
		}
		slog.Error("Failed to apply Proxmox script import", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to apply Proxmox import"})
		return
	}

	c.IndentedJSON(http.StatusOK, ProxmoxImportApplyResponse{
		Applied:    true,
		ImportedAt: importedAt,
		Summary:    preview.Summary,
	})
}

func proxmoxHypervisorHostFromRequest(c *gin.Context) (models.Host, bool) {
	host, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return models.Host{}, false
	}
	profile, found, err := gdb.SelectHypervisorProfileByMAC(host.Mac)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load hypervisor profile"})
		return models.Host{}, false
	}
	if !found || profile.Platform != string(models.HypervisorPlatformProxmoxVE) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "Proxmox import requires a Proxmox VE hypervisor profile"})
		return models.Host{}, false
	}
	return host, true
}

func loadProxmoxImportCurrentState(mac string) (proxmoximport.CurrentState, error) {
	var current proxmoximport.CurrentState

	state, found, err := gdb.SelectProxmoxSourceState(mac, models.InfrastructureWorkloadSourceScriptImport)
	if err != nil {
		return current, err
	}
	if found {
		current.SourceState = &state
	}

	managed, found, err := gdb.SelectHypervisorProfileByMAC(mac)
	if err != nil {
		return current, err
	}
	if found {
		current.ManagedHypervisor = &managed
	}

	workloads, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(mac)
	if err != nil {
		return current, err
	}
	current.Workloads = workloads
	return current, nil
}

func decodeStrictProxmoxImportJSON(c *gin.Context, destination any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProxmoxImportBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid Proxmox import JSON"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid Proxmox import JSON"})
		return false
	}
	return true
}
