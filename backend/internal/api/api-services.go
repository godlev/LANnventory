package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

// getHostServices godoc
// @Summary      Get retained services for a host
// @Description  Return durable service summary records for the host MAC across retained IPv4 and IPv6 addresses.
// @Tags         hosts
// @Produce      json
// @Param        id   path      int  true  "Host ID"
// @Success      200  {array}   models.Service
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/services [get]
func getHostServices(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	services, err := gdb.SelectServicesByMAC(host.Mac)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load host services"})
		return
	}
	if services == nil {
		services = make([]models.Service, 0)
	}

	c.IndentedJSON(http.StatusOK, services)
}
