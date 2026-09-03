package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	defaultActivityLimit  = 20
	maxActivityLimit      = 100
	defaultActivityOffset = 0
	activityCursorLayout  = "2006-01-02 15:04:05"

	activityCategoryAll          = "all"
	activityCategoryConnectivity = "connectivity"
	activityCategoryChanges      = "changes"
)

var (
	errInvalidActivityLimit    = errors.New("invalid limit")
	errInvalidActivityOffset   = errors.New("invalid offset")
	errInvalidActivityCategory = errors.New("invalid category")
	errInvalidActivityEvent    = errors.New("invalid eventType")
	errInvalidActivityCursor   = errors.New("invalid cursor")
	errMixedActivityPagination = errors.New("offset cannot be combined with cursor")
)

// getActivity godoc
// @Summary      Get recent activity
// @Description  Retrieve recent host activity events ordered by Date descending, then ID descending.
// @Description  First page requests omit beforeDate and beforeId. To request the next page, send the Date and ID from the final event returned by the previous page as beforeDate and beforeId.
// @Description  Cursor pagination requires both beforeDate and beforeId. beforeDate must use YYYY-MM-DD HH:mm:ss, beforeId must be greater than 0, and nonzero legacy offset cannot be combined with a cursor. offset=0 with a cursor is accepted.
// @Description  Legacy offset pagination remains supported when beforeDate and beforeId are omitted.
// @Tags         activity
// @Produce      json
// @Param        limit      query     int     false  "Event limit from 1 to 100. Defaults to 20." minimum(1) maximum(100)
// @Param        offset     query     int     false  "Legacy event offset, 0 or greater. Used only when beforeDate and beforeId are omitted." minimum(0)
// @Param        beforeDate query     string  false  "Cursor event date in YYYY-MM-DD HH:mm:ss format. Requires beforeId."
// @Param        beforeId   query     int     false  "Cursor event ID, greater than 0. Requires beforeDate." minimum(1)
// @Param        category   query     string  false  "Event category" Enums(all, connectivity, changes)
// @Param        eventType  query     string  false  "Repeatable event type filter" Enums(discovered, online, offline, known, unknown, device-type-changed)
// @Param        mac        query     string  false  "Repeatable MAC address filter"
// @Success      200        {array}   models.HostEvent
// @Failure      400        {object}  map[string]string  "Invalid query or cursor parameters"
// @Failure      500        {object}  map[string]string  "Database query failure"
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

	cursor, err := parseActivityCursor(c, offset)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	categoryTypes, err := parseActivityCategory(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	requestedTypes, err := parseActivityEventTypes(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eventTypes, empty := combineActivityEventTypes(categoryTypes, requestedTypes)
	if empty {
		c.IndentedJSON(http.StatusOK, []models.HostEvent{})
		return
	}

	events, ok := gdb.SelectEventsFiltered(gdb.EventQuery{
		Limit:      limit,
		Offset:     offset,
		BeforeDate: cursor.BeforeDate,
		BeforeID:   cursor.BeforeID,
		Macs:       parseActivityMacs(c),
		EventTypes: eventTypes,
	})
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load activity events"})
		return
	}

	models.AddHostEventDisplayTimes(events, time.Local)
	c.IndentedJSON(http.StatusOK, events)
}

// getActivityStats godoc
// @Summary      Get activity event stats
// @Description  Retrieve faceted retained activity event counts
// @Tags         activity
// @Produce      json
// @Param        mac  query     string  false  "Repeatable MAC address filter"
// @Success      200  {object}  models.ActivityStats
// @Failure      500  {object}  map[string]string  "Database query failure"
// @Router       /activity/stats [get]
func getActivityStats(c *gin.Context) {
	stats, ok := gdb.SelectEventStats(parseActivityMacs(c))
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load activity stats"})
		return
	}

	c.IndentedJSON(http.StatusOK, stats)
}

// getActivityDevices godoc
// @Summary      Get activity device filter options
// @Description  Retrieve devices represented in current hosts and retained activity events
// @Tags         activity
// @Produce      json
// @Success      200  {array}  models.ActivityDeviceOption
// @Failure      500  {object} map[string]string  "Database query failure"
// @Router       /activity/devices [get]
func getActivityDevices(c *gin.Context) {
	devices, ok := gdb.SelectActivityDeviceOptions()
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load activity devices"})
		return
	}

	c.IndentedJSON(http.StatusOK, devices)
}

// getHostActivity godoc
// @Summary      Get recent activity for one host
// @Description  Retrieve recent host activity events for a host ID
// @Tags         activity
// @Produce      json
// @Param        id     path      string  true   "Host ID"
// @Param        limit  query     int     false  "Event limit from 1 to 100. Defaults to 20." minimum(1) maximum(100)
// @Success      200    {array}   models.HostEvent
// @Failure      400    {object}  map[string]string  "Invalid host ID or limit"
// @Failure      500    {object}  map[string]string  "Database query failure"
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

	events, ok := gdb.SelectEventsByHostID(host.ID, limit)
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load host activity"})
		return
	}

	models.AddHostEventDisplayTimes(events, time.Local)
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

type activityCursor struct {
	BeforeDate string
	BeforeID   int
}

func parseActivityCursor(c *gin.Context, offset int) (activityCursor, error) {
	beforeDate, hasBeforeDate := c.GetQuery("beforeDate")
	beforeIDRaw, hasBeforeID := c.GetQuery("beforeId")
	if !hasBeforeDate && !hasBeforeID {
		return activityCursor{}, nil
	}
	if !hasBeforeDate || !hasBeforeID {
		return activityCursor{}, errInvalidActivityCursor
	}
	if offset > 0 {
		return activityCursor{}, errMixedActivityPagination
	}
	if _, err := time.Parse(activityCursorLayout, beforeDate); err != nil {
		return activityCursor{}, errInvalidActivityCursor
	}

	beforeID, err := strconv.Atoi(beforeIDRaw)
	if err != nil || beforeID < 1 {
		return activityCursor{}, errInvalidActivityCursor
	}

	return activityCursor{
		BeforeDate: beforeDate,
		BeforeID:   beforeID,
	}, nil
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

func parseActivityEventTypes(c *gin.Context) ([]models.HostEventType, error) {
	values := c.QueryArray("eventType")
	if len(values) == 0 {
		return nil, nil
	}

	eventTypes := make([]models.HostEventType, 0, len(values))
	for _, value := range values {
		if !models.IsValidHostEventType(value) {
			return nil, errInvalidActivityEvent
		}
		eventTypes = append(eventTypes, models.HostEventType(value))
	}

	return eventTypes, nil
}

func combineActivityEventTypes(categoryTypes, requestedTypes []models.HostEventType) ([]models.HostEventType, bool) {
	if len(categoryTypes) == 0 {
		return requestedTypes, false
	}
	if len(requestedTypes) == 0 {
		return categoryTypes, false
	}

	categorySet := make(map[models.HostEventType]struct{}, len(categoryTypes))
	for _, eventType := range categoryTypes {
		categorySet[eventType] = struct{}{}
	}

	combined := make([]models.HostEventType, 0, len(requestedTypes))
	seen := make(map[models.HostEventType]struct{}, len(requestedTypes))
	for _, eventType := range requestedTypes {
		if _, allowed := categorySet[eventType]; !allowed {
			continue
		}
		if _, exists := seen[eventType]; exists {
			continue
		}
		seen[eventType] = struct{}{}
		combined = append(combined, eventType)
	}

	return combined, len(combined) == 0
}

func parseActivityMacs(c *gin.Context) []string {
	rawMacs := c.QueryArray("mac")
	macs := make([]string, 0, len(rawMacs))
	seen := make(map[string]struct{}, len(rawMacs))
	for _, mac := range rawMacs {
		if mac == "" {
			continue
		}
		if _, ok := seen[mac]; ok {
			continue
		}
		seen[mac] = struct{}{}
		macs = append(macs, mac)
	}

	return macs
}
