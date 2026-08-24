package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/aceberg/WatchYourLAN/internal/gdb"
	"github.com/aceberg/WatchYourLAN/internal/models"
)

const (
	defaultActivityLimit  = 20
	maxActivityLimit      = 100
	defaultActivityOffset = 0

	activityCategoryAll          = "all"
	activityCategoryConnectivity = "connectivity"
	activityCategoryChanges      = "changes"
)

var (
	errInvalidActivityLimit    = errors.New("invalid limit")
	errInvalidActivityOffset   = errors.New("invalid offset")
	errInvalidActivityCategory = errors.New("invalid category")
)

// getActivity godoc
// @Summary      Get recent activity
// @Description  Retrieve recent host activity events
// @Tags         activity
// @Produce      json
// @Param        limit     query     int     false  "Event limit from 1 to 100"
// @Param        offset    query     int     false  "Event offset, 0 or greater"
// @Param        category  query     string  false  "Event category: all, connectivity, changes"
// @Param        mac       query     string  false  "Filter by MAC address"
// @Success      200       {array}   models.HostEvent
// @Router       /activity [get]
func getActivity(c *gin.Context) {
	limit, err := parseActivityLimit(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	offset, err := parseActivityOffset(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eventTypes, err := parseActivityCategory(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events, _ := gdb.SelectEventsFiltered(gdb.EventQuery{
		Limit:      limit,
		Offset:     offset,
		Mac:        c.Query("mac"),
		EventTypes: eventTypes,
	})
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

func parseActivityOffset(c *gin.Context) (int, error) {
	rawOffset := c.DefaultQuery("offset", strconv.Itoa(defaultActivityOffset))
	offset, err := strconv.Atoi(rawOffset)
	if err != nil || offset < 0 {
		return 0, errInvalidActivityOffset
	}

	return offset, nil
}

func parseActivityCategory(c *gin.Context) ([]models.HostEventType, error) {
	category := c.DefaultQuery("category", activityCategoryAll)
	switch category {
	case activityCategoryAll:
		return nil, nil
	case activityCategoryConnectivity:
		return []models.HostEventType{
			models.EventOnline,
			models.EventOffline,
		}, nil
	case activityCategoryChanges:
		return []models.HostEventType{
			models.EventDiscovered,
			models.EventKnown,
			models.EventUnknown,
			models.EventDeviceTypeChanged,
		}, nil
	default:
		return nil, errInvalidActivityCategory
	}
}
