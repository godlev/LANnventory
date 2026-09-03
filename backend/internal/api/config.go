package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/routines"
)

var errInvalidPositiveInt = errors.New("invalid positive integer")

var restartScanner = routines.ScanRestart

type colorRequest struct {
	Color string `json:"color"`
}

type retentionRequest struct {
	PresenceRetention     int `form:"presenceRetention" json:"presenceRetention"`
	ConnectivityRetention int `form:"connectivityRetention" json:"connectivityRetention"`
}

// saveConfigHandler godoc
// @Summary      Save general configuration
// @Description  Update general UI and server configuration from a form submission.
// @Tags         configuration
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        host         formData  string  false  "Bind host"
// @Param        port         formData  string  false  "Bind port"
// @Param        theme        formData  string  false  "Theme name"
// @Param        color        formData  string  false  "Color mode" Enums(dark, light)
// @Param        node         formData  string  false  "Legacy local node-bootstrap URL"
// @Param        shout        formData  string  false  "Notification URL. Leave blank to keep configured secret."
// @Param        clear_shout  formData  string  false  "Truthy value clears the stored notification URL"
// @Success      302          {string}  string  "Redirect to referrer"
// @Failure      500          {object}  map[string]string  "Failed to write config"
// @Router       /config/ [post]
func saveConfigHandler(c *gin.Context) {

	_, err := conf.UpdateAppConfig(func(nextConfig *models.Conf) error {
		nextConfig.Host = c.PostForm("host")
		nextConfig.Port = c.PostForm("port")
		nextConfig.Theme = c.PostForm("theme")
		nextConfig.Color = c.PostForm("color")
		nextConfig.NodePath = c.PostForm("node")
		nextConfig.ShoutURL = applySecretUpdate(nextConfig.ShoutURL, c.PostForm("shout"), c.PostForm("clear_shout"))
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	c.Redirect(http.StatusFound, c.Request.Referer())
}

// saveColorHandler godoc
// @Summary      Save color mode
// @Description  Update the UI color mode. JSON is the documented API request format; form data is also accepted for UI compatibility.
// @Tags         configuration
// @Accept       json
// @Produce      json
// @Param        body  body      colorRequest  true  "Color payload"
// @Success      200   {object}  models.Conf
// @Failure      400   {object}  map[string]string  "Invalid color"
// @Failure      500   {object}  map[string]string  "Failed to write config"
// @Router       /config/color [post]
func saveColorHandler(c *gin.Context) {
	color := c.PostForm("color")

	if color == "" {
		var req colorRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			color = req.Color
		}
	}

	if color != "dark" && color != "light" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid color"})
		return
	}

	nextConfig, err := conf.UpdateAppConfig(func(nextConfig *models.Conf) error {
		nextConfig.Color = color
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	c.IndentedJSON(http.StatusOK, toPublicConfig(nextConfig))
}

// saveRetentionHandler godoc
// @Summary      Save retention settings
// @Description  Update presence and connectivity event retention windows. JSON is the documented API request format; form data is also accepted for UI compatibility.
// @Tags         configuration
// @Accept       json
// @Produce      json
// @Param        body  body      retentionRequest  true  "Retention payload"
// @Success      200   {object}  models.Conf
// @Failure      400   {object}  map[string]string  "Invalid retention value"
// @Failure      500   {object}  map[string]string  "Failed to write config"
// @Router       /config/retention [post]
func saveRetentionHandler(c *gin.Context) {
	var req retentionRequest
	if err := c.ShouldBind(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid retention"})
		return
	}

	if req.PresenceRetention < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid presenceRetention"})
		return
	}
	if req.ConnectivityRetention < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid connectivityRetention"})
		return
	}

	nextConfig, err := conf.UpdateAppConfig(func(nextConfig *models.Conf) error {
		nextConfig.TrimHist = req.PresenceRetention
		nextConfig.ConnectivityRetention = req.ConnectivityRetention
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	c.IndentedJSON(http.StatusOK, toPublicConfig(nextConfig))
}

// saveSettingsHandler godoc
// @Summary      Save scan and database settings
// @Description  Update scan and database settings from a form submission, then restart scanning.
// @Tags         configuration
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        log                     formData  string    true   "Log level" Enums(debug, info, warn, error)
// @Param        arpargs                 formData  string    false  "arp-scan arguments"
// @Param        ifaces                  formData  string    false  "Interfaces to scan"
// @Param        usedb                   formData  string    true   "Database backend" Enums(sqlite, postgres)
// @Param        pgconnect               formData  string    false  "PostgreSQL connection URL. Leave blank to keep configured secret."
// @Param        clear_pgconnect         formData  string    false  "Truthy value clears the stored PostgreSQL connection URL"
// @Param        timeout                 formData  int       true   "Scan interval in seconds" minimum(1)
// @Param        trim                    formData  int       false  "Presence history retention in hours" minimum(1)
// @Param        connectivity_retention  formData  int       false  "Connectivity event retention in hours" minimum(1)
// @Param        arpstrs                 formData  []string  false  "Repeatable static ARP result strings" collectionFormat(multi)
// @Success      302                     {string}  string    "Redirect to referrer"
// @Failure      400                     {object}  map[string]string  "Invalid settings"
// @Failure      500                     {object}  map[string]string  "Failed to reconnect database or write config"
// @Router       /config_settings/ [post]
func saveSettingsHandler(c *gin.Context) {

	currentConfig := conf.GetAppConfig()
	nextConfig := currentConfig
	nextConfig.LogLevel = c.PostForm("log")
	nextConfig.ArpArgs = c.PostForm("arpargs")
	nextConfig.Ifaces = c.PostForm("ifaces")

	useDB := c.PostForm("usedb")
	if useDB != "sqlite" && useDB != "postgres" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid usedb"})
		return
	}
	nextConfig.UseDB = useDB
	nextConfig.PGConnect = applySecretUpdate(nextConfig.PGConnect, c.PostForm("pgconnect"), c.PostForm("clear_pgconnect"))

	if !isValidLogLevel(nextConfig.LogLevel) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid log"})
		return
	}

	timeout, err := parsePositiveInt(c.PostForm("timeout"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid timeout"})
		return
	}

	trimHist := nextConfig.TrimHist
	if rawTrimHist := c.PostForm("trim"); rawTrimHist != "" {
		trimHist, err = parsePositiveInt(rawTrimHist)
		if err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid trim"})
			return
		}
	}

	connectivityRetention := nextConfig.ConnectivityRetention
	if connectivityRetention < 1 {
		connectivityRetention = trimHist
	}
	if rawConnectivityRetention := c.PostForm("connectivity_retention"); rawConnectivityRetention != "" {
		connectivityRetention, err = parsePositiveInt(rawConnectivityRetention)
		if err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid connectivity_retention"})
			return
		}
	}

	nextConfig.Timeout = timeout
	nextConfig.TrimHist = trimHist
	nextConfig.ConnectivityRetention = connectivityRetention

	arpStrs := c.PostFormArray("arpstrs")
	nextConfig.ArpStrs = []string{}
	for _, s := range arpStrs {
		if s != "" {
			nextConfig.ArpStrs = append(nextConfig.ArpStrs, s)
		}
	}

	dbChanged := nextConfig.UseDB != currentConfig.UseDB || nextConfig.PGConnect != currentConfig.PGConnect
	if dbChanged {
		if err := gdb.Reconnect(nextConfig); err != nil {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to reconnect database"})
			return
		}
	}

	_, err = conf.UpdateAppConfig(func(config *models.Conf) error {
		config.LogLevel = nextConfig.LogLevel
		config.ArpArgs = nextConfig.ArpArgs
		config.Ifaces = nextConfig.Ifaces
		config.UseDB = nextConfig.UseDB
		config.PGConnect = nextConfig.PGConnect
		config.Timeout = nextConfig.Timeout
		config.TrimHist = nextConfig.TrimHist
		config.ConnectivityRetention = nextConfig.ConnectivityRetention
		config.ArpStrs = append([]string(nil), nextConfig.ArpStrs...)
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	restartScanner()

	c.Redirect(http.StatusFound, c.Request.Referer())
}

func parsePositiveInt(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errInvalidPositiveInt
	}

	return parsed, nil
}

func isValidLogLevel(value string) bool {
	switch value {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// saveInfluxHandler godoc
// @Summary      Save InfluxDB configuration
// @Description  Update InfluxDB integration settings from a form submission.
// @Tags         configuration
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        enable              formData  string  false  "Truthy value enables InfluxDB"
// @Param        addr                formData  string  false  "InfluxDB address"
// @Param        token               formData  string  false  "InfluxDB token. Leave blank to keep configured secret."
// @Param        clear_influx_token  formData  string  false  "Truthy value clears the stored InfluxDB token"
// @Param        org                 formData  string  false  "InfluxDB organization"
// @Param        bucket              formData  string  false  "InfluxDB bucket"
// @Param        skip                formData  string  false  "Truthy value skips TLS verification"
// @Success      302                 {string}  string  "Redirect to referrer"
// @Failure      500                 {object}  map[string]string  "Failed to write config"
// @Router       /config_influx/ [post]
func saveInfluxHandler(c *gin.Context) {

	enable := c.PostForm("enable")
	skip := c.PostForm("skip")
	_, err := conf.UpdateAppConfig(func(nextConfig *models.Conf) error {
		nextConfig.InfluxAddr = c.PostForm("addr")
		nextConfig.InfluxToken = applySecretUpdate(nextConfig.InfluxToken, c.PostForm("token"), c.PostForm("clear_influx_token"))
		nextConfig.InfluxOrg = c.PostForm("org")
		nextConfig.InfluxBucket = c.PostForm("bucket")
		nextConfig.InfluxEnable = enable == "on"
		nextConfig.InfluxSkipTLS = skip == "on"
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	c.Redirect(http.StatusFound, c.Request.Referer())
}

// savePrometheusHandler godoc
// @Summary      Save Prometheus configuration
// @Description  Update Prometheus integration settings from a form submission.
// @Tags         configuration
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        enable  formData  string  false  "Truthy value enables Prometheus metrics"
// @Success      302     {string}  string  "Redirect to referrer"
// @Failure      500     {object}  map[string]string  "Failed to write config"
// @Router       /config_prometheus/ [post]
func savePrometheusHandler(c *gin.Context) {
	enable := c.PostForm("enable")

	_, err := conf.UpdateAppConfig(func(nextConfig *models.Conf) error {
		nextConfig.PrometheusEnable = enable == "on"
		return nil
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	c.Redirect(http.StatusFound, c.Request.Referer())
}

func applySecretUpdate(currentValue, submittedValue, clearValue string) string {
	if isTruthyFormValue(clearValue) {
		return ""
	}
	if submittedValue != "" {
		return submittedValue
	}

	return currentValue
}

func isTruthyFormValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "on", "true", "yes":
		return true
	default:
		return false
	}
}
