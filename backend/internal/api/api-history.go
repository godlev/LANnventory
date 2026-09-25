package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

const historyDateLayout = "2006-01-02 15:04:05"

// getHistory godoc
// @Summary      Get full history
// @Description  Retrieve the complete history of all hosts. Not recommended, the output can be a lot
// @Description  Inventory metadata fields are not included on history rows.
// @Tags         history
// @Produce      json
// @Success      200  {array}   models.Host
// @Router       /history [get]
func getHistory(c *gin.Context) {
	hosts, _ := gdb.Select("history")
	c.IndentedJSON(http.StatusOK, hosts)
}

// getHistoryByMAC godoc
// @Summary      Get history by MAC
// @Description  Retrieve the latest history entries for a specific host by MAC address
// @Description  Inventory metadata fields are not included on history rows.
// @Tags         history
// @Produce      json
// @Param        mac       path      string  true  "MAC address of the host"
// @Param        num       query     int     true  "Number of history entries to return"
// @Param        timeZone  query     string  false "IANA browser time zone used to render DATE values"
// @Success      200       {array}   models.Host
// @Failure      400       {object}  map[string]string
// @Router       /history/{mac} [get]
func getHistoryByMAC(c *gin.Context) {
	mac := c.Param("mac")
	numStr := c.Query("num")
	num, err := parsePositiveInt(numStr)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid num"})
		return
	}

	hosts := gdb.SelectLatest(mac, num)
	clientLocation, err := requestedHistoryLocation(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid timeZone"})
		return
	}
	if clientLocation != nil {
		convertHistoryDates(hosts, time.Local, clientLocation)
	}
	c.IndentedJSON(http.StatusOK, hosts)
}

// getHistoryByDate godoc
// @Summary      Get history by date
// @Description  Retrieve history for a specific host on a given date
// @Description  Inventory metadata fields are not included on history rows.
// @Description  Legacy callers may filter by DATE prefix. Browser clients can provide an explicit UTC range so the selected date represents the browser-local calendar day.
// @Description  The date format is flexible and can be:
// @Description  - Year only: `2025`
// @Description  - Year + month: `2025-09`
// @Description  - Full date: `2025-09-06`
// @Description  - Full timestamp: `2025-09-06 00:58:26`
// @Tags         history
// @Produce      json
// @Param        mac       path      string  true  "MAC address of the host"
// @Param        date      path      string  true  "Date filter (supports YYYY, YYYY-MM, YYYY-MM-DD, YYYY-MM-DD HH:mm:ss)"
// @Param        from      query     string  false "Browser-local day start as RFC3339 UTC instant"
// @Param        to        query     string  false "Next browser-local day start as RFC3339 UTC instant"
// @Param        timeZone  query     string  false "IANA browser time zone used to render DATE values"
// @Success      200       {array}   models.Host
// @Failure      400       {object}  map[string]string
// @Failure      500       {object}  map[string]string
// @Router       /history/{mac}/{date} [get]
func getHistoryByDate(c *gin.Context) {
	mac := c.Param("mac")
	date := c.Param("date")
	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))

	var hosts []models.Host
	if fromRaw != "" || toRaw != "" {
		from, to, err := historyDateRange(fromRaw, toRaw, time.Local)
		if err != nil {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid presence date range"})
			return
		}

		var ok bool
		hosts, ok = gdb.SelectByDateRange(mac, from, to)
		if !ok {
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "presence history could not be loaded"})
			return
		}
	} else {
		hosts = gdb.SelectByDate(mac, date)
	}

	clientLocation, err := requestedHistoryLocation(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid timeZone"})
		return
	}
	if clientLocation != nil {
		convertHistoryDates(hosts, time.Local, clientLocation)
	}
	c.IndentedJSON(http.StatusOK, hosts)
}

func historyDateRange(fromRaw, toRaw string, serverLocation *time.Location) (string, string, error) {
	if strings.TrimSpace(fromRaw) == "" || strings.TrimSpace(toRaw) == "" {
		return "", "", fmt.Errorf("both range bounds are required")
	}
	if serverLocation == nil {
		serverLocation = time.UTC
	}

	from, err := time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		return "", "", fmt.Errorf("parse from: %w", err)
	}
	to, err := time.Parse(time.RFC3339, toRaw)
	if err != nil {
		return "", "", fmt.Errorf("parse to: %w", err)
	}
	if !from.Before(to) {
		return "", "", fmt.Errorf("from must be before to")
	}

	return from.In(serverLocation).Format(historyDateLayout), to.In(serverLocation).Format(historyDateLayout), nil
}

func requestedHistoryLocation(c *gin.Context) (*time.Location, error) {
	name := strings.TrimSpace(c.Query("timeZone"))
	if name == "" {
		return nil, nil
	}
	return time.LoadLocation(name)
}

func convertHistoryDates(hosts []models.Host, sourceLocation, targetLocation *time.Location) {
	if sourceLocation == nil || targetLocation == nil {
		return
	}
	for i := range hosts {
		value := strings.TrimSpace(hosts[i].Date)
		if value == "" {
			continue
		}
		parsed, err := time.ParseInLocation(historyDateLayout, value, sourceLocation)
		if err != nil {
			continue
		}
		hosts[i].Date = parsed.In(targetLocation).Format(historyDateLayout)
	}
}
