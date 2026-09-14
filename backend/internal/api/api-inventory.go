package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
)

// getInventoryOptions godoc
// @Summary      Get inventory field options
// @Description  Retrieve distinct owner and location values from current host metadata for lightweight autocomplete.
// @Tags         hosts
// @Produce      json
// @Success      200  {object}  models.InventoryOptions
// @Failure      500  {object}  map[string]string
// @Router       /inventory/options [get]
func getInventoryOptions(c *gin.Context) {
	options, err := gdb.SelectInventoryOptions()
	if err != nil {
		slog.Error("Failed to load inventory options", "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load inventory options"})
		return
	}

	c.IndentedJSON(http.StatusOK, options)
}
