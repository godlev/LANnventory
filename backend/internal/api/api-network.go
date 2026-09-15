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

type hostPortScanResponse struct {
	Port int  `json:"port"`
	Open bool `json:"open"`
}

type hostPortScanRangeRequest struct {
	StartPort int `json:"startPort"`
	EndPort   int `json:"endPort"`
}

type hostPortStateResponse struct {
	HostID      int    `json:"hostId"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Open        bool   `json:"open"`
	Service     string `json:"service"`
	FirstSeen   string `json:"firstSeen"`
	LastScanned string `json:"lastScanned"`
	LastChanged string `json:"lastChanged"`
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
	if _, err := gdb.ReconcileHostPortObservations(
		host,
		map[int]bool{portNumber: open},
		time.Now().Format(models.HostEventDateLayout),
	); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to persist port scan state"})
		return
	}

	c.IndentedJSON(http.StatusOK, hostPortScanResponse{Port: portNumber, Open: open})
}

// startHostPortScan godoc
// @Summary      Start host port range scan
// @Description  Start an asynchronous bounded-concurrency TCP port scan for a current host.
// @Tags         network
// @Accept       json
// @Produce      json
// @Param        id       path      int                       true  "Host ID"
// @Param        request  body      hostPortScanRangeRequest  true  "Port range"
// @Success      202      {object}  portScanJobStatus
// @Failure      400      {object}  map[string]string
// @Failure      409      {object}  map[string]string
// @Router       /host/{id}/ports/scan [post]
func startHostPortScan(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	if strings.TrimSpace(host.IP) == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no current IP"})
		return
	}

	var req hostPortScanRangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid port scan request"})
		return
	}
	if req.StartPort < 1 || req.StartPort > 65535 || req.EndPort < 1 || req.EndPort > 65535 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "port range must be between 1 and 65535"})
		return
	}
	if req.StartPort > req.EndPort {
		req.StartPort, req.EndPort = req.EndPort, req.StartPort
	}

	status, err := portScanJobs.Start(host, req.StartPort, req.EndPort)
	if err == errPortScanInProgress {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to start port scan"})
		return
	}

	c.IndentedJSON(http.StatusAccepted, status)
}

// getActiveHostPortScan godoc
// @Summary      Get active host port scan
// @Description  Return the currently running port scan for a host, if one exists.
// @Tags         network
// @Produce      json
// @Param        id  path      int  true  "Host ID"
// @Success      200 {object}  portScanJobStatus
// @Failure      404 {object}  map[string]string
// @Router       /host/{id}/ports/scan/active [get]
func getActiveHostPortScan(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	status, ok := portScanJobs.Active(host.ID)
	if !ok {
		c.IndentedJSON(http.StatusNotFound, gin.H{"error": "no active port scan"})
		return
	}
	c.IndentedJSON(http.StatusOK, status)
}

// getHostPortScanStatus godoc
// @Summary      Get host port scan status
// @Tags         network
// @Produce      json
// @Param        id      path      int     true  "Host ID"
// @Param        scanId  path      string  true  "Scan job ID"
// @Success      200     {object}  portScanJobStatus
// @Failure      404     {object}  map[string]string
// @Router       /host/{id}/ports/scan/{scanId} [get]
func getHostPortScanStatus(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	status, ok := portScanJobs.Status(host.ID, c.Param("scanId"))
	if !ok {
		c.IndentedJSON(http.StatusNotFound, gin.H{"error": "port scan job not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, status)
}

// cancelHostPortScan godoc
// @Summary      Cancel host port scan
// @Tags         network
// @Produce      json
// @Param        id      path      int     true  "Host ID"
// @Param        scanId  path      string  true  "Scan job ID"
// @Success      200     {object}  portScanJobStatus
// @Failure      404     {object}  map[string]string
// @Router       /host/{id}/ports/scan/{scanId} [delete]
func cancelHostPortScan(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	status, ok := portScanJobs.Cancel(host.ID, c.Param("scanId"))
	if !ok {
		c.IndentedJSON(http.StatusNotFound, gin.H{"error": "port scan job not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, status)
}

// getHostPorts godoc
// @Summary      Get persisted host port states
// @Description  Return ports previously observed open for a host, plus any later closed transitions retained for history.
// @Tags         network
// @Produce      json
// @Param        id  path      int  true  "Host ID"
// @Success      200 {array}   hostPortStateResponse
// @Failure      400 {object}  map[string]string
// @Failure      500 {object}  map[string]string
// @Router       /host/{id}/ports [get]
func getHostPorts(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	states, err := gdb.SelectHostPorts(host.ID)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load host port states"})
		return
	}

	response := make([]hostPortStateResponse, 0, len(states))
	for _, state := range states {
		response = append(response, hostPortStateResponse{
			HostID:      state.HostID,
			Port:        state.Port,
			Protocol:    state.Protocol,
			Open:        state.Open,
			Service:     portscan.ServiceName(state.Port),
			FirstSeen:   state.FirstSeen,
			LastScanned: state.LastScanned,
			LastChanged: state.LastChanged,
		})
	}
	c.IndentedJSON(http.StatusOK, response)
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
