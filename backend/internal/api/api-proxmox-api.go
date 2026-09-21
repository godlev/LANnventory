package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoxapi"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

type ProxmoxAPITestConnectionResponse struct {
	Status string                          `json:"status"`
	Result proxmoxapi.ConnectionTestResult `json:"result"`
}

type ProxmoxAPISyncPreviewResponse struct {
	Snapshot proxmoxsnapshot.Snapshot `json:"snapshot"`
	Preview  proxmoximport.Preview     `json:"preview"`
}

// testHostProxmoxAPIConnection godoc
// @Summary      Test Proxmox API connection
// @Description  Authenticate and verify read-only node and guest inventory access. This never imports or changes LANnventory inventory.
// @Tags         proxmox
// @Produce      json
// @Param        id   path      string  true  "Proxmox Host ID"
// @Success      200  {object}  ProxmoxAPITestConnectionResponse
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /host/{id}/proxmox/api/test [post]
func testHostProxmoxAPIConnection(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	client, _, err := configuredProxmoxAPIClient(host.Mac, false)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	attemptedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := proxmoxapi.TestConnection(c.Request.Context(), client)
	if err != nil {
		recordProxmoxAPIStatus(host.Mac, "error", attemptedAt, "", err.Error())
		writeProxmoxAPIError(c, err)
		return
	}

	recordProxmoxAPIStatus(host.Mac, "connected", attemptedAt, "", "")
	c.IndentedJSON(http.StatusOK, ProxmoxAPITestConnectionResponse{
		Status: "connected",
		Result: result,
	})
}

// previewHostProxmoxAPISync godoc
// @Summary      Collect and preview Proxmox API inventory
// @Description  Perform a read-only API collection and feed the normalized Snapshot into the existing Preview/Diff pipeline. No inventory changes are applied.
// @Tags         proxmox
// @Produce      json
// @Param        id   path      string  true  "Proxmox Host ID"
// @Success      200  {object}  ProxmoxAPISyncPreviewResponse
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /host/{id}/proxmox/api/sync-preview [post]
func previewHostProxmoxAPISync(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	client, _, err := configuredProxmoxAPIClient(host.Mac, true)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	attemptedAt := time.Now().UTC()
	snapshot, err := proxmoxapi.CollectSnapshot(c.Request.Context(), client, attemptedAt)
	if err != nil {
		recordProxmoxAPIStatus(host.Mac, "error", attemptedAt.Format(time.RFC3339), "", err.Error())
		writeProxmoxAPIError(c, err)
		return
	}

	current, err := loadProxmoxImportCurrentState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil {
		slog.Error("Failed to load Proxmox API preview state", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox inventory state"})
		return
	}
	preview, err := proxmoximport.BuildPreview(host.Mac, snapshot, current)
	if err != nil {
		recordProxmoxAPIStatus(host.Mac, "error", attemptedAt.Format(time.RFC3339), "", "collected Proxmox data failed validation")
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := "preview-ready"
	lastError := ""
	if !snapshot.Complete {
		status = "degraded"
		lastError = strings.Join(snapshot.CollectionErrors, "; ")
	}
	recordProxmoxAPIStatus(host.Mac, status, attemptedAt.Format(time.RFC3339), "", lastError)

	c.IndentedJSON(http.StatusOK, ProxmoxAPISyncPreviewResponse{
		Snapshot: snapshot,
		Preview:  preview,
	})
}

// applyHostProxmoxAPISync godoc
// @Summary      Apply confirmed Proxmox API inventory preview
// @Description  Revalidate the API Snapshot and stale-preview token, then apply it through the existing atomic Proxmox import pipeline.
// @Tags         proxmox
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "Proxmox Host ID"
// @Param        body  body      ProxmoxImportApplyDoc      true  "Confirmed API snapshot preview"
// @Success      200   {object}  ProxmoxImportApplyResponseDoc
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/proxmox/api/sync-apply [post]
func applyHostProxmoxAPISync(c *gin.Context) {
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
	if strings.TrimSpace(payload.Snapshot.Source) != proxmoxsnapshot.SourceProxmoxAPI {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "Proxmox API sync requires a proxmox-api snapshot"})
		return
	}

	normalized, err := proxmoximport.ValidateAndNormalize(payload.Snapshot)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	current, err := loadProxmoxImportCurrentState(host.Mac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil {
		slog.Error("Failed to load current Proxmox API state", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox inventory state"})
		return
	}
	preview, err := proxmoximport.BuildPreview(host.Mac, normalized, current)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.PreviewToken != preview.PreviewToken {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": "preview is stale; run Sync now again before applying"})
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
	if err := gdb.ApplyProxmoxImport(host.Mac, state, inputs); err != nil {
		if errors.Is(err, gdb.ErrInfrastructureWorkloadSourceConflict) {
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "workload state changed since preview; run Sync now again"})
			return
		}
		slog.Error("Failed to apply Proxmox API inventory", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to apply Proxmox inventory"})
		return
	}

	recordProxmoxAPIStatus(host.Mac, "connected", importedAt, importedAt, "")
	c.IndentedJSON(http.StatusOK, ProxmoxImportApplyResponse{
		Applied:    true,
		ImportedAt: importedAt,
		Summary:    preview.Summary,
	})
}

func configuredProxmoxAPIClient(hypervisorMac string, requireEnabled bool) (*proxmoxapi.Client, models.ProxmoxAPIConfig, error) {
	config, found, err := gdb.SelectProxmoxAPIConfig(hypervisorMac)
	if err != nil {
		return nil, models.ProxmoxAPIConfig{}, errors.New("failed to load Proxmox API configuration")
	}
	if !found {
		return nil, models.ProxmoxAPIConfig{}, errors.New("Proxmox API is not configured")
	}
	if requireEnabled && !config.Enabled {
		return nil, config, errors.New("Proxmox API integration is disabled")
	}
	if strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.TokenID) == "" || config.TokenSecret == "" {
		return nil, config, errors.New("Proxmox API credentials are incomplete")
	}
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	client, err := proxmoxapi.New(proxmoxapi.Config{
		BaseURL:     config.BaseURL,
		TokenID:     config.TokenID,
		TokenSecret: config.TokenSecret,
		VerifyTLS:   config.VerifyTLS,
		Timeout:     timeout,
	})
	if err != nil {
		return nil, config, err
	}
	return client, config, nil
}

func recordProxmoxAPIStatus(mac, status, lastAttempt, lastSuccess, lastError string) {
	if err := gdb.UpdateProxmoxAPIStatus(mac, status, lastAttempt, lastSuccess, lastError); err != nil {
		slog.Warn("Failed to persist Proxmox API source status", "mac", mac, "status", status, "err", err)
	}
}

func writeProxmoxAPIError(c *gin.Context, err error) {
	status := http.StatusBadGateway
	switch proxmoxapi.KindOf(err) {
	case proxmoxapi.ErrorInvalidConfig:
		status = http.StatusBadRequest
	case proxmoxapi.ErrorTimeout:
		status = http.StatusGatewayTimeout
	}
	c.IndentedJSON(status, gin.H{
		"error": err.Error(),
		"kind":  proxmoxapi.KindOf(err),
	})
}
