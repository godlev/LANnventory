package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/servicescan"
)

const defaultServiceScanIntervalMinutes = 1440

type serviceScanSettingsRequest struct {
	Enabled         bool  `json:"enabled"`
	IntervalMinutes int   `json:"intervalMinutes"`
	Ports           []int `json:"ports"`
}

type serviceScanSettingsResponse struct {
	Enabled          bool   `json:"enabled"`
	IntervalMinutes  int    `json:"intervalMinutes"`
	Ports            []int  `json:"ports"`
	NextScanAt       string `json:"nextScanAt"`
	LastAttemptAt    string `json:"lastAttemptAt"`
	LastSuccessfulAt string `json:"lastSuccessfulAt"`
	LastError        string `json:"lastError"`
}

// getHostServiceScanSettings godoc
// @Summary      Get scheduled service scan settings
// @Description  Return opt-in scheduled TCP service scan settings and runtime state for a host.
// @Tags         hosts
// @Produce      json
// @Param        id   path      int  true  "Host ID"
// @Success      200  {object}  serviceScanSettingsResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/service-scan-settings [get]
func getHostServiceScanSettings(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	response, err := loadServiceScanSettingsResponse(host.Mac)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load service scan settings"})
		return
	}
	c.IndentedJSON(http.StatusOK, response)
}

// setHostServiceScanSettings godoc
// @Summary      Save scheduled service scan settings
// @Description  Enable or disable scheduled TCP service scanning for a host. Ports are validated, de-duplicated and sorted. Enabling or changing settings schedules the next scan immediately.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      int                         true  "Host ID"
// @Param        body  body      serviceScanSettingsRequest  true  "Service scan settings"
// @Success      200   {object}  serviceScanSettingsResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/service-scan-settings [put]
func setHostServiceScanSettings(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	var request serviceScanSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid service scan settings"})
		return
	}
	if request.IntervalMinutes <= 0 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "intervalMinutes must be greater than zero"})
		return
	}

	portsJSON, normalizedPorts, err := servicescan.EncodePortsJSON(request.Ports)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid scheduled service scan ports"})
		return
	}
	if request.Enabled && len(normalizedPorts) == 0 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "at least one port is required when scheduled scanning is enabled"})
		return
	}

	settings, found, err := gdb.SelectServiceScanSettingsByMAC(host.Mac)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load service scan settings"})
		return
	}
	if !found {
		settings = models.ServiceScanSettings{Mac: host.Mac}
	}

	settings.Enabled = request.Enabled
	settings.IntervalMinutes = request.IntervalMinutes
	settings.PortsJSON = portsJSON
	settings.LastError = ""
	if request.Enabled {
		settings.NextScanAt = time.Now().Format(models.HostEventDateLayout)
	} else {
		settings.NextScanAt = ""
	}

	settings, err = gdb.UpsertServiceScanSettings(settings)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to save service scan settings"})
		return
	}

	response, err := serviceScanSettingsResponseFromModel(settings)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to read saved service scan settings"})
		return
	}
	c.IndentedJSON(http.StatusOK, response)
}

func loadServiceScanSettingsResponse(mac string) (serviceScanSettingsResponse, error) {
	settings, found, err := gdb.SelectServiceScanSettingsByMAC(mac)
	if err != nil {
		return serviceScanSettingsResponse{}, err
	}
	if !found {
		return serviceScanSettingsResponse{
			Enabled:         false,
			IntervalMinutes: defaultServiceScanIntervalMinutes,
			Ports:           []int{},
		}, nil
	}
	return serviceScanSettingsResponseFromModel(settings)
}

func serviceScanSettingsResponseFromModel(settings models.ServiceScanSettings) (serviceScanSettingsResponse, error) {
	ports, err := servicescan.DecodePortsJSON(settings.PortsJSON)
	if err != nil {
		return serviceScanSettingsResponse{}, err
	}
	return serviceScanSettingsResponse{
		Enabled:          settings.Enabled,
		IntervalMinutes:  settings.IntervalMinutes,
		Ports:            ports,
		NextScanAt:       settings.NextScanAt,
		LastAttemptAt:    settings.LastAttemptAt,
		LastSuccessfulAt: settings.LastSuccessfulAt,
		LastError:        settings.LastError,
	}, nil
}
