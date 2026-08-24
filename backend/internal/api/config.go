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

type colorRequest struct {
	Color string `json:"color"`
}

func saveConfigHandler(c *gin.Context) {

	conf.AppConfig.Host = c.PostForm("host")
	conf.AppConfig.Port = c.PostForm("port")
	conf.AppConfig.Theme = c.PostForm("theme")
	conf.AppConfig.Color = c.PostForm("color")
	conf.AppConfig.NodePath = c.PostForm("node")
	conf.AppConfig.ShoutURL = c.PostForm("shout")

	conf.Write(conf.AppConfig)

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

func saveSettingsHandler(c *gin.Context) {

	conf.AppConfig.LogLevel = c.PostForm("log")
	conf.AppConfig.ArpArgs = c.PostForm("arpargs")
	conf.AppConfig.Ifaces = c.PostForm("ifaces")

	useDB := c.PostForm("usedb")
	pgConnect := c.PostForm("pgconnect")

	if useDB != conf.AppConfig.UseDB || pgConnect != conf.AppConfig.PGConnect {
		conf.AppConfig.UseDB = c.PostForm("usedb")
		conf.AppConfig.PGConnect = c.PostForm("pgconnect")
		gdb.Connect()
	}

	timeout, err := parsePositiveInt(c.PostForm("timeout"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid timeout"})
		return
	}

	trimHist, err := parsePositiveInt(c.PostForm("trim"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid trim"})
		return
	}

	conf.AppConfig.Timeout = timeout
	conf.AppConfig.TrimHist = trimHist

	arpStrs := c.PostFormArray("arpstrs")
	conf.AppConfig.ArpStrs = []string{}
	for _, s := range arpStrs {
		if s != "" {
			conf.AppConfig.ArpStrs = append(conf.AppConfig.ArpStrs, s)
		}
	}

	conf.Write(conf.AppConfig)

	routines.ScanRestart()

	c.Redirect(http.StatusFound, c.Request.Referer())
}

func parsePositiveInt(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errInvalidPositiveInt
	}

	return parsed, nil
}

func saveInfluxHandler(c *gin.Context) {

	conf.AppConfig.InfluxAddr = c.PostForm("addr")
	conf.AppConfig.InfluxToken = c.PostForm("token")
	conf.AppConfig.InfluxOrg = c.PostForm("org")
	conf.AppConfig.InfluxBucket = c.PostForm("bucket")

	enable := c.PostForm("enable")
	skip := c.PostForm("skip")
	conf.AppConfig.InfluxEnable = enable == "on"
	conf.AppConfig.InfluxSkipTLS = skip == "on"

	conf.Write(conf.AppConfig)

	c.Redirect(http.StatusFound, c.Request.Referer())
}

func savePrometheusHandler(c *gin.Context) {
	enable := c.PostForm("enable")

	conf.AppConfig.PrometheusEnable = enable == "on"

	conf.Write(conf.AppConfig)

	c.Redirect(http.StatusFound, c.Request.Referer())
}
