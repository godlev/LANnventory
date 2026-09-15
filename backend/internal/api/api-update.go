package api

import (
	"errors"
	"net"
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

// getUpdateStatus checks the selected GitHub release channel for a newer LANnventory release.
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

// saveUpdateChannel persists Stable or Beta and immediately refreshes update status.
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

// applyUpdate verifies and schedules installation of the newest release in the configured channel.
func applyUpdate(c *gin.Context) {
	var req updateApplyRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid update request"})
			return
		}
	}

	config := conf.GetAppConfig()
	result, err := updateService.Schedule(
		c.Request.Context(),
		config.Version,
		config.UpdateChannel,
		strings.TrimSpace(req.Version),
		updateHealthURL(config.Host, config.Port),
	)
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

func updateHealthURL(host, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if port == "" {
		port = "8840"
	}

	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::", "[::]":
		host = "::1"
	default:
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		}
	}

	return "http://" + net.JoinHostPort(host, port) + "/api/health"
}
