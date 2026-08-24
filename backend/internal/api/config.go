package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/aceberg/WatchYourLAN/internal/conf"
	"github.com/aceberg/WatchYourLAN/internal/gdb"
	"github.com/aceberg/WatchYourLAN/internal/routines"
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

func saveConfigHandler(c *gin.Context) {

	nextConfig := conf.AppConfig
	nextConfig.Host = c.PostForm("host")
	nextConfig.Port = c.PostForm("port")
	nextConfig.Theme = c.PostForm("theme")
	nextConfig.Color = c.PostForm("color")
	nextConfig.NodePath = c.PostForm("node")
	nextConfig.ShoutURL = c.PostForm("shout")

	if err := conf.WriteErr(nextConfig); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	conf.AppConfig = nextConfig

	c.Redirect(http.StatusFound, c.Request.Referer())
}

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

	nextConfig := conf.AppConfig
	nextConfig.Color = color

	if err := conf.WriteErr(nextConfig); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	conf.AppConfig = nextConfig

	c.IndentedJSON(http.StatusOK, conf.AppConfig)
}

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

	nextConfig := conf.AppConfig
	nextConfig.TrimHist = req.PresenceRetention
	nextConfig.ConnectivityRetention = req.ConnectivityRetention

	if err := conf.WriteErr(nextConfig); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	conf.AppConfig = nextConfig

	c.IndentedJSON(http.StatusOK, conf.AppConfig)
}

func saveSettingsHandler(c *gin.Context) {

	nextConfig := conf.AppConfig
	nextConfig.LogLevel = c.PostForm("log")
	nextConfig.ArpArgs = c.PostForm("arpargs")
	nextConfig.Ifaces = c.PostForm("ifaces")

	useDB := c.PostForm("usedb")
	pgConnect := c.PostForm("pgconnect")
	if useDB != "sqlite" && useDB != "postgres" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid usedb"})
		return
	}
	nextConfig.UseDB = useDB
	nextConfig.PGConnect = pgConnect

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

	if err := conf.WriteErr(nextConfig); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	dbChanged := useDB != conf.AppConfig.UseDB || pgConnect != conf.AppConfig.PGConnect
	conf.AppConfig = nextConfig

	if dbChanged {
		gdb.Start()
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

func saveInfluxHandler(c *gin.Context) {

	nextConfig := conf.AppConfig
	nextConfig.InfluxAddr = c.PostForm("addr")
	nextConfig.InfluxToken = c.PostForm("token")
	nextConfig.InfluxOrg = c.PostForm("org")
	nextConfig.InfluxBucket = c.PostForm("bucket")

	enable := c.PostForm("enable")
	skip := c.PostForm("skip")
	nextConfig.InfluxEnable = enable == "on"
	nextConfig.InfluxSkipTLS = skip == "on"

	if err := conf.WriteErr(nextConfig); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	conf.AppConfig = nextConfig

	c.Redirect(http.StatusFound, c.Request.Referer())
}

func savePrometheusHandler(c *gin.Context) {
	enable := c.PostForm("enable")

	nextConfig := conf.AppConfig
	nextConfig.PrometheusEnable = enable == "on"

	if err := conf.WriteErr(nextConfig); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	conf.AppConfig = nextConfig

	c.Redirect(http.StatusFound, c.Request.Referer())
}
