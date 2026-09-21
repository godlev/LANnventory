package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

// getHostProxmoxSourceState godoc
// @Summary      Get imported Proxmox source state
// @Description  Return the last successfully applied Proxmox node state for one source. source defaults to script-import and may also be proxmox-api. Managed HypervisorProfile fields remain separate.
// @Tags         hosts
// @Produce      json
// @Param        id      path   string  true   "Proxmox Host ID"
// @Param        source  query  string  false  "Source provenance" Enums(script-import, proxmox-api)
// @Success      200  {object}  models.ProxmoxSourceState
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/proxmox/source-state [get]
func getHostProxmoxSourceState(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	source := c.DefaultQuery("source", models.InfrastructureWorkloadSourceScriptImport)
	if source != models.InfrastructureWorkloadSourceScriptImport && source != models.InfrastructureWorkloadSourceProxmoxAPI {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid Proxmox source"})
		return
	}
	state, found, err := gdb.SelectProxmoxSourceState(host.Mac, source)
	if err != nil {
		slog.Error("Failed to load Proxmox source state", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox source state"})
		return
	}
	if !found {
		c.IndentedJSON(http.StatusOK, nil)
		return
	}
	c.IndentedJSON(http.StatusOK, state)
}
