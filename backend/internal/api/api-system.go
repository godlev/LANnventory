package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/diagnostics"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/notify"
	"github.com/godlev/LANnventory/internal/routines"
)

// getVersion godoc
// @Summary      Get application version
// @Description  Returns the current running version of the application
// @Tags         system
// @Produce      json
// @Success      200  {string}  string
// @Router       /version [get]
func getVersion(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, conf.GetAppConfig().Version)
}

// triggerRescan godoc
// @Summary      Rescan all interfaces now
// @Description  Manually trigger rescan
// @Tags         system
// @Produce      json
// @Success      200  {string}  string  "OK"
// @Router       /rescan [get]
func triggerRescan(c *gin.Context) {
	routines.ScanRestart()
	c.Status(http.StatusOK)
}

// getConfig godoc
// @Summary      Get application configuration
// @Description  Returns the current configuration used by the app
// @Tags         system
// @Produce      json
// @Success      200  {object}  models.Conf
// @Router       /config [get]
func getConfig(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, toPublicConfig(conf.GetAppConfig()))
}

// getHealth godoc
// @Summary      Health check
// @Description  Returns OK without triggering scans or mutations
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "OK"
// @Router       /health [get]
func getHealth(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

type scannerErrorResponse struct {
	Source  string `json:"source"`
	Command string `json:"command"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

type scannerDatabaseResponse struct {
	Status  string `json:"status"`
	Backend string `json:"backend,omitempty"`
	Error   string `json:"error,omitempty"`
}

type scannerStatusResponse struct {
	Status               string                   `json:"status"`
	Scanning             bool                     `json:"scanning"`
	LastScanStartedAt    *time.Time               `json:"lastScanStartedAt"`
	LastScanAt           *time.Time               `json:"lastScanAt"`
	LastSuccessfulScanAt *time.Time               `json:"lastSuccessfulScanAt"`
	DurationMs           int64                    `json:"durationMs"`
	DevicesFound         int                      `json:"devicesFound"`
	Interfaces           []string                 `json:"interfaces"`
	LastError            *scannerErrorResponse    `json:"lastError"`
	NextScanAt           *time.Time               `json:"nextScanAt"`
	ServerTime           time.Time                `json:"serverTime"`
	Database             scannerDatabaseResponse  `json:"database"`
}

var (
	scannerStateSnapshot  = routines.GetScannerState
	databaseHealthSnapshot = gdb.GetDatabaseHealth
	scannerStatusNow      = func() time.Time { return time.Now().UTC() }
)

func getScannerStatus(c *gin.Context) {
	state := scannerStateSnapshot()
	dbHealth := databaseHealthSnapshot()

	status := state.Status
	if status == "" {
		status = routines.ScannerStatusProblem
	}

	dbStatus := "problem"
	if dbHealth.Connected {
		dbStatus = "connected"
	}

	var lastError *scannerErrorResponse
	if len(state.LastErrors) > 0 {
		item := state.LastErrors[len(state.LastErrors)-1]
		lastError = &scannerErrorResponse{
			Source:  item.Source,
			Command: item.Command,
			Kind:    string(item.Kind),
			Message: item.Message,
			Output:  item.Output,
		}
	}

	c.IndentedJSON(http.StatusOK, scannerStatusResponse{
		Status:               string(status),
		Scanning:             status == routines.ScannerStatusScanning,
		LastScanStartedAt:    scannerTimePointer(state.LastScanStartedAt),
		LastScanAt:           scannerTimePointer(state.LastScanAt),
		LastSuccessfulScanAt: scannerTimePointer(state.LastSuccessfulScanAt),
		DurationMs:           state.Duration.Milliseconds(),
		DevicesFound:         state.DevicesFound,
		Interfaces:           append([]string(nil), state.Interfaces...),
		LastError:            lastError,
		NextScanAt:           scannerTimePointer(state.NextScanAt),
		ServerTime:           scannerStatusNow(),
		Database: scannerDatabaseResponse{
			Status:  dbStatus,
			Backend: dbHealth.Backend,
			Error:   dbHealth.Error,
		},
	})
}

func scannerTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func getDiagnostics(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, diagnostics.Run(conf.GetAppConfig()))
}

// notifyTest godoc
// @Summary      Send test notification
// @Description  Trigger a test notification to verify notification settings
// @Tags         system
// @Produce      json
// @Success      200  {string}  string  "OK"
// @Router       /notify_test [get]
func notifyTest(c *gin.Context) {
	notify.Test()
	c.Status(http.StatusOK)
}

// getStatus godoc
// @Summary      Get network status
// @Description  Retrieve summary statistics of hosts, optionally filtered by interface
// @Tags         system
// @Produce      json
// @Param        iface  path      string  false  "Interface name (omit for all interfaces)"
// @Success      200    {object}  models.Stat
// @Router       /status/{iface} [get]
func getStatus(c *gin.Context) {
	var status models.Stat
	var searchHosts []models.Host

	allHosts, _ := gdb.Select("now")

	iface := c.Param("iface")
	iface = iface[1:]

	if iface != "" && iface != "undefined" {
		for _, host := range allHosts {
			if iface == host.Iface {
				searchHosts = append(searchHosts, host)
			}
		}
	} else {
		searchHosts = allHosts
	}

	for _, host := range searchHosts {
		status.Total = status.Total + 1

		if host.Known > 0 {
			status.Known = status.Known + 1
		} else {
			status.Unknown = status.Unknown + 1
		}
		if host.Now > 0 {
			status.Online = status.Online + 1
		} else {
			status.Offline = status.Offline + 1
		}
	}

	c.IndentedJSON(http.StatusOK, status)
}
