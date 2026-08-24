package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/aceberg/WatchYourLAN/internal/gdb"
)

const (
	defaultActivityLimit = 20
	maxActivityLimit     = 100
)

var errInvalidActivityLimit = errors.New("invalid limit")

// getActivity godoc
// @Summary      Get recent activity
// @Description  Retrieve recent host activity events
// @Tags         activity
// @Produce      json
// @Param        limit  query     int     false  "Event limit from 1 to 100"
// @Param        mac    query     string  false  "Filter by MAC address"
// @Success      200    {array}   models.HostEvent
// @Router       /activity [get]
func getActivity(c *gin.Context) {
	limit, err := parseActivityLimit(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events, _ := gdb.SelectEvents(limit, c.Query("mac"))
	c.IndentedJSON(http.StatusOK, events)
}

// getHostActivity godoc
// @Summary      Get recent activity for one host
// @Description  Retrieve recent host activity events for a host ID
// @Tags         activity
// @Produce      json
// @Param        id     path      string  true   "Host ID"
// @Param        limit  query     int     false  "Event limit from 1 to 100"
// @Success      200    {array}   models.HostEvent
// @Router       /host/{id}/activity [get]
func getHostActivity(c *gin.Context) {
	idStr := c.Param("id")
	host, err := getHostByID(idStr)
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	limit, err := parseActivityLimit(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events, _ := gdb.SelectEventsByHostID(host.ID, limit)
	c.IndentedJSON(http.StatusOK, events)
}

func parseActivityLimit(c *gin.Context) (int, error) {
	rawLimit := c.DefaultQuery("limit", strconv.Itoa(defaultActivityLimit))
	limit, err := strconv.Atoi(rawLimit)
	if err != nil || limit < 1 || limit > maxActivityLimit {
		return 0, errInvalidActivityLimit
	}

	return limit, nil
}
