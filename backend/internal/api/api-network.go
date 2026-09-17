package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linde12/gowol"

	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/portscan"
)

var portIsOpen = portscan.IsOpen
var portProbe = portscan.Probe

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
// @Description  Scan a TCP port using the host's current IP. Definitive results update persistent service inventory and emit lifecycle activity only on state transitions.
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
	result := portProbe(c.Request.Context(), host.IP, port)
	if result.State == portscan.ProbeCanceled {
		return
	}
	if result.State == portscan.ProbeIndeterminate {
		c.IndentedJSON(http.StatusServiceUnavailable, gin.H{"error": "port scan could not determine service state"})
		return
	}

	observedAt := time.Now().Format(models.HostEventDateLayout)
	if _, _, _, err := gdb.RecordHostServiceObservation(host, models.Service{
		Mac:            host.Mac,
		Address:        host.IP,
		Protocol:       string(models.ServiceProtocolTCP),
		Port:           portNumber,
		State:          string(result.State),
		LastScanSource: "manual",
	}, observedAt); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to record service scan result"})
		return
	}

	open := result.State == portscan.ProbeOpen
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
