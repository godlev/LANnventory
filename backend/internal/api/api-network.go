package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linde12/gowol"

	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/portscan"
)

var portIsOpen = portscan.IsOpen

type hostPortScanResponse struct {
	Port int  `json:"port"`
	Open bool `json:"open"`
}

// getPortState godoc
// @Summary      Check port state
// @Description  Check whether a given TCP port on an address is open or closed
// @Tags         network
// @Produce      json
// @Param        addr  path      string  true  "IP address or hostname"
// @Param        port  path      string  true  "Port number"
// @Success      200   {boolean}  bool   "true if open, false if closed"
// @Router       /port/{addr}/{port} [get]
func getPortState(c *gin.Context) {
	addr := c.Param("addr")
	port := c.Param("port")

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
		return
	}

	state := portIsOpen(addr, port)
	c.IndentedJSON(http.StatusOK, state)
}

// scanHostPort godoc
// @Summary      Scan one port for a host
// @Description  Scan a TCP port using the host's current IP. When the port is open, persist an activity event for that host.
// @Tags         network
// @Produce      json
// @Param        id    path      int  true  "Host ID"
// @Param        port  path      int  true  "Port number"
// @Success      200   {object}  hostPortScanResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/port/{port}/scan [post]
func scanHostPort(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	portNumber, err := strconv.Atoi(c.Param("port"))
	if err != nil || portNumber < 1 || portNumber > 65535 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
		return
	}
	if strings.TrimSpace(host.IP) == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no current IP"})
		return
	}

	port := strconv.Itoa(portNumber)
	open := portIsOpen(host.IP, port)
	if open {
		if err := gdb.AddEvent(models.NewHostEvent(host, models.EventPortOpen, "", port)); err != nil {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to record port scan event"})
			return
		}
	}

	c.IndentedJSON(http.StatusOK, hostPortScanResponse{Port: portNumber, Open: open})
}

// sendWOL godoc
// @Summary      Send Wake-on-LAN packet
// @Description  Send a magic packet to wake up a host by its MAC address
// @Tags         network
// @Produce      json
// @Param        mac   path      string  true  "MAC address of the host"
// @Success      200   {boolean} bool    "true if sent successfully"
// @Router       /wol/{mac} [get]
func sendWOL(c *gin.Context) {

	mac := c.Param("mac")

	packet, err := gowol.NewMagicPacket(mac)

	if !check.IfError(err) {
		err = packet.Send("255.255.255.255")

		slog.Info("Wake-on-LAN: " + mac)
	}

	c.IndentedJSON(http.StatusOK, !check.IfError(err))
}
