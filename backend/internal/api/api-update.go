package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/updater"
)

var updateService = updater.NewService()

var (
	updateSchedulerMu sync.RWMutex
	updateScheduler   *updater.AutoScheduler
)

type updateChannelRequest struct {
	Channel string `json:"channel"`
}

type updateSettingsRequest struct {
	Channel        string `json:"channel"`
	AutomaticCheck bool   `json:"automaticCheck"`
	Automatic      bool   `json:"automatic"`
	IntervalHours  int    `json:"intervalHours"`
}

type updateApplyRequest struct {
	Version string `json:"version"`
}

type updateStatusResponse struct {
	CurrentVersion      string `json:"currentVersion"`
	Channel             string `json:"channel"`
	LatestVersion       string `json:"latestVersion"`
	Available           bool   `json:"available"`
	PublishedAt         string `json:"publishedAt"`
	ReleaseURL          string `json:"releaseUrl"`
	ReleaseSummary      string `json:"releaseSummary"`
	InstallSupported    bool   `json:"installSupported"`
	InstallReason       string `json:"installReason"`
	Message             string `json:"message"`
	Updating            bool   `json:"updating"`
	AutomaticCheck      bool   `json:"automaticCheck"`
	Automatic           bool   `json:"automatic"`
	IntervalHours       int    `json:"intervalHours"`
	LastChecked         string `json:"lastChecked"`
	SnapshotBaseVersion string `json:"snapshotBaseVersion"`
}

type updateApplyResponse struct {
	Version    string `json:"version"`
	Scheduled  bool   `json:"scheduled"`
	Message    string `json:"message"`
	BackupPath string `json:"backupPath"`
}

// StartUpdateScheduler starts automatic update polling using the same updater service
// as the manual update endpoints.
func StartUpdateScheduler(ctx context.Context) {
	scheduler := updater.NewAutoScheduler(updateService, func() updater.AutoConfig {
		config := conf.GetAppConfig()
		return updater.AutoConfig{
			CurrentVersion:   config.Version,
			Channel:          config.UpdateChannel,
			AutomaticCheck:   config.UpdateCheckAuto,
			AutomaticInstall: config.UpdateAuto,
			IntervalHours:    config.UpdateCheckIntervalHours,
			HealthURL:        updateHealthURL(config.Host, config.Port),
		}
	})

	updateSchedulerMu.Lock()
	updateScheduler = scheduler
	updateSchedulerMu.Unlock()

	scheduler.Start(ctx)

	go func() {
		<-ctx.Done()
		updateSchedulerMu.Lock()
		if updateScheduler == scheduler {
			updateScheduler = nil
		}
		updateSchedulerMu.Unlock()
	}()
}

func notifyUpdateSchedulerConfigChanged() {
	updateSchedulerMu.RLock()
	scheduler := updateScheduler
	updateSchedulerMu.RUnlock()
	if scheduler != nil {
		scheduler.NotifyConfigChanged()
	}
}

// getUpdateStatus godoc
// @Summary      Get update status
// @Description  Returns cached release metadata without contacting GitHub. Set refresh=true only for an explicit manual update check.
// @Tags         updates
// @Produce      json
// @Param        refresh  query     bool  false  "Explicitly refresh release metadata"
// @Param        cached   query     bool  false  "Deprecated compatibility flag; cached-only is the default"
// @Success      200      {object}  updateStatusResponse
// @Failure      502      {object}  map[string]string
// @Router       /update/status [get]
func getUpdateStatus(c *gin.Context) {
	config := conf.GetAppConfig()
	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	if !refresh {
		status := updateService.CheckCached(config.Version, config.UpdateChannel)
		c.IndentedJSON(http.StatusOK, withUpdateSettings(status, config))
		return
	}

	status, err := updateService.Check(c.Request.Context(), config.Version, config.UpdateChannel, true)
	if err != nil {
		c.IndentedJSON(http.StatusBadGateway, gin.H{"error": "update check failed", "detail": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, withUpdateSettings(status, config))
}

// saveUpdateChannel godoc
// @Summary      Set update channel
// @Description  Persists the Stable or Beta update channel without triggering a release check.
// @Tags         updates
// @Accept       json
// @Produce      json
// @Param        request  body      updateChannelRequest  true  "Update channel"
// @Success      200      {object}  updateStatusResponse
// @Failure      400      {object}  map[string]string
// @Failure      500      {object}  map[string]string
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
	notifyUpdateSchedulerConfigChanged()

	status := updateService.CheckCached(nextConfig.Version, channel)
	c.IndentedJSON(http.StatusOK, withUpdateSettings(status, nextConfig))
}

// saveUpdateSettings godoc
// @Summary      Set update settings
// @Description  Persists release channel, automatic check/install preferences, and automatic check interval without triggering a release check.
// @Tags         updates
// @Accept       json
// @Produce      json
// @Param        request  body      updateSettingsRequest  true  "Update settings"
// @Success      200      {object}  updateStatusResponse
// @Failure      400      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /update/settings [post]
func saveUpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid update settings request"})
		return
	}

	channel := strings.ToLower(strings.TrimSpace(req.Channel))
	if !updater.ValidChannel(channel) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "update channel must be stable or beta"})
		return
	}
	if !updater.ValidAutoIntervalHours(req.IntervalHours) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "update interval must be 6, 12, 24, or 168 hours"})
		return
	}

	automaticCheck := req.AutomaticCheck || req.Automatic
	nextConfig, err := conf.UpdateAppConfig(func(next *models.Conf) error {
		next.UpdateChannel = channel
		next.UpdateCheckAuto = automaticCheck
		next.UpdateAuto = req.Automatic && automaticCheck
		next.UpdateCheckIntervalHours = req.IntervalHours
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to persist update settings"})
		return
	}
	notifyUpdateSchedulerConfigChanged()

	status := updateService.CheckCached(nextConfig.Version, channel)
	c.IndentedJSON(http.StatusOK, withUpdateSettings(status, nextConfig))
}

// applyUpdate godoc
// @Summary      Apply update
// @Description  Verifies and schedules installation of the newest release in the configured channel. The service restarts after the package update is scheduled.
// @Tags         updates
// @Accept       json
// @Produce      json
// @Param        request  body      updateApplyRequest  false  "Expected version"
// @Success      202      {object}  updateApplyResponse
// @Failure      400      {object}  map[string]string
// @Failure      409      {object}  map[string]string
// @Failure      502      {object}  map[string]string
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

func withUpdateSettings(status updater.Status, config models.Conf) updater.Status {
	status.AutomaticCheck = config.UpdateCheckAuto || config.UpdateAuto
	status.Automatic = config.UpdateAuto
	status.IntervalHours = updater.NormalizeAutoIntervalHours(config.UpdateCheckIntervalHours)
	return status
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
