package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/proxmoxapi"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
	"github.com/godlev/LANnventory/internal/proxmoxsync"
)

var proxmoxSyncService = proxmoxsync.NewService()

type ProxmoxAPIConnectionTestResultDoc struct {
	ConnectionOK bool   `json:"connectionOk"`
	PVEVersion   string `json:"pveVersion"`
	NodeAccess   bool   `json:"nodeAccess"`
	VMInventory  bool   `json:"vmInventory"`
	LXCInventory bool   `json:"lxcInventory"`
	NodeCount    int    `json:"nodeCount"`
	VMCount      int    `json:"vmCount"`
	LXCCount     int    `json:"lxcCount"`
}

type ProxmoxAPITestConnectionResponse struct {
	Status string                              `json:"status"`
	Result ProxmoxAPIConnectionTestResultDoc  `json:"result"`
}

type ProxmoxAPISyncPreviewResponse struct {
	Snapshot proxmoxsnapshot.Snapshot `json:"snapshot"`
	Preview  proxmoximport.Preview     `json:"preview"`
}

type ProxmoxAPISyncPreviewDoc struct {
	Snapshot ProxmoxImportSnapshotDoc `json:"snapshot"`
	Preview  ProxmoxImportPreviewDoc  `json:"preview"`
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

	result, err := proxmoxSyncService.TestConnection(c.Request.Context(), host.Mac)
	if err != nil {
		if proxmoxsync.KindOf(err) == proxmoxsync.ErrorConfig {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		writeProxmoxAPIError(c, err)
		return
	}

	c.IndentedJSON(http.StatusOK, ProxmoxAPITestConnectionResponse{
		Status: "connected",
		Result: ProxmoxAPIConnectionTestResultDoc{
			ConnectionOK: result.ConnectionOK,
			PVEVersion:   result.PVEVersion,
			NodeAccess:   result.NodeAccess,
			VMInventory:  result.VMInventory,
			LXCInventory: result.LXCInventory,
			NodeCount:    result.NodeCount,
			VMCount:      result.VMCount,
			LXCCount:     result.LXCCount,
		},
	})
}

// previewHostProxmoxAPISync godoc
// @Summary      Collect and preview Proxmox API inventory
// @Description  Perform a read-only API collection and feed the normalized Snapshot into the existing Preview/Diff pipeline. No inventory changes are applied.
// @Tags         proxmox
// @Produce      json
// @Param        id   path      string  true  "Proxmox Host ID"
// @Success      200  {object}  ProxmoxAPISyncPreviewDoc
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /host/{id}/proxmox/api/sync-preview [post]
func previewHostProxmoxAPISync(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	result, err := proxmoxSyncService.CollectPreview(c.Request.Context(), host.Mac, proxmoxsync.CollectOptions{
		RequireEnabled: false,
		Trigger:        proxmoxsync.TriggerManual,
	})
	if err != nil {
		switch proxmoxsync.KindOf(err) {
		case proxmoxsync.ErrorConfig:
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case proxmoxsync.ErrorBusy:
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "Sync already in progress"})
		case proxmoxsync.ErrorState:
			slog.Error("Failed to load Proxmox API preview state", "hostID", host.ID, "mac", host.Mac, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox inventory state"})
		case proxmoxsync.ErrorValidation:
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			writeProxmoxAPIError(c, err)
		}
		return
	}

	c.IndentedJSON(http.StatusOK, ProxmoxAPISyncPreviewResponse{
		Snapshot: result.Snapshot,
		Preview:  result.Preview,
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

	result, err := proxmoxSyncService.ApplyPreview(
		c.Request.Context(),
		host.Mac,
		payload.Snapshot,
		payload.PreviewToken,
		proxmoxsync.ApplyOptions{Trigger: proxmoxsync.TriggerManual},
	)
	if err != nil {
		switch proxmoxsync.KindOf(err) {
		case proxmoxsync.ErrorValidation:
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case proxmoxsync.ErrorBusy:
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "Sync already in progress"})
		case proxmoxsync.ErrorState:
			slog.Error("Failed to load current Proxmox API state", "hostID", host.ID, "mac", host.Mac, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox inventory state"})
		case proxmoxsync.ErrorStalePreview:
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "preview is stale; run Sync now again before applying"})
		case proxmoxsync.ErrorApplyBlocked:
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "preview cannot be applied until blocking issues are resolved"})
		case proxmoxsync.ErrorConflict:
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "workload state changed since preview; run Sync now again"})
		case proxmoxsync.ErrorStaleConfig:
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "Proxmox API configuration changed; run Sync now again"})
		case proxmoxsync.ErrorPersistence:
			slog.Error("Failed to apply Proxmox API inventory", "hostID", host.ID, "mac", host.Mac, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to apply Proxmox inventory"})
		default:
			slog.Error("Unexpected Proxmox API apply failure", "hostID", host.ID, "mac", host.Mac, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to apply Proxmox inventory"})
		}
		return
	}

	notifyProxmoxSyncSchedulerConfigChanged()
	c.IndentedJSON(http.StatusOK, ProxmoxImportApplyResponse{
		Applied:    true,
		ImportedAt: result.ImportedAt,
		Summary:    result.Summary,
	})
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
