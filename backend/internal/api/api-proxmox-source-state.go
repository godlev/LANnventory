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
// @Description  Return the last successfully imported script-collector node state for one Proxmox Host. Managed HypervisorProfile fields remain separate.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string                      true  "Proxmox Host ID"
// @Success      200  {object}  models.ProxmoxSourceState
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/proxmox/source-state [get]
func getHostProxmoxSourceState(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	state, found, err := gdb.SelectProxmoxSourceState(host.Mac, models.InfrastructureWorkloadSourceScriptImport)
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
