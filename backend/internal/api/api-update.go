package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/updater"
)

var updateService = updater.NewService()

type updateChannelRequest struct {
	Channel string `json:"channel"`
}

type updateApplyRequest struct {
	Version string `json:"version"`
}

// getUpdateStatus godoc
// @Summary      Check for LANnventory updates
// @Description  Check the selected release channel and report whether a newer release is available.
// @Tags         system
// @Produce      json
// @Param        refresh  query     bool  false  "Bypass the short release cache"
// @Success      200      {object}  updater.Status
// @Failure      502      {object}  map[string]string
// @Router       /update/status [get]
func getUpdateStatus(c *gin.Context) {
	config := conf.GetAppConfig()
	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	status, err := updateService.Check(c.Request.Context(), config.Version, config.UpdateChannel, refresh)
	if err != nil {
		c.IndentedJSON(http.StatusBadGateway, gin.H{"error": "update check failed", "detail": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, status)
}

// saveUpdateChannel godoc
// @Summary      Set update channel
// @Description  Persist Stable or Beta as the release channel used for update checks.
// @Tags         configuration
// @Accept       json
// @Produce      json
// @Param        body  body      updateChannelRequest  true  "Update channel"
// @Success      200   {object}  updater.Status
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Failure      502   {object}  map[string]string
// @Router       /update/channel [post]
func saveUpdateChannel(c *gin.Context) {
	var req updateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid update channel request"})
		return
	}

	channel := strings.ToLower(strings.TrimSpace(req.Channel))
	if !updater.ValidChannel(channel) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "update channel must be stable or beta"})
		return
	}

	nextConfig, err := conf.UpdateAppConfig(func(next *models.Conf) error {
		next.UpdateChannel = channel
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to persist update channel"})
		return
	}

	status, err := updateService.Check(c.Request.Context(), nextConfig.Version, channel, true)
	if err != nil {
		c.IndentedJSON(http.StatusBadGateway, gin.H{"error": "channel saved but update check failed", "detail": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, status)
}

// applyUpdate godoc
// @Summary      Install available LANnventory update
// @Description  Verify the latest release for the selected channel, schedule Debian package installation, and restart LANnventory.
// @Tags         system
// @Accept       json
// @Produce      json
// @Param        body  body      updateApplyRequest  false  "Expected target version"
// @Success      202   {object}  updater.ApplyResult
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Failure      502   {object}  map[string]string
// @Router       /update/apply [post]
func applyUpdate(c *gin.Context) {
	var req updateApplyRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid update request"})
			return
		}
	}

	config := conf.GetAppConfig()
	result, err := updateService.Schedule(c.Request.Context(), config.Version, config.UpdateChannel, strings.TrimSpace(req.Version))
	if err != nil {
		switch {
		case errors.Is(err, updater.ErrNoUpdate):
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "no update is available"})
		case errors.Is(err, updater.ErrUpdateInProgress):
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "an update is already in progress"})
		case errors.Is(err, updater.ErrUnsupportedInstall):
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.IndentedJSON(http.StatusBadGateway, gin.H{"error": "failed to schedule update", "detail": err.Error()})
		}
		return
	}

	c.IndentedJSON(http.StatusAccepted, result)
}
