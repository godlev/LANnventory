package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/backup"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/version"
)

// getBackupExport godoc
// @Summary      Download portable data backup
// @Description  Export a versioned logical backup containing current hosts, host history and activity events. Configuration secrets are not included.
// @Tags         export
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]string  "Backup generation failure"
// @Router       /export/backup [get]
func getBackupExport(c *gin.Context) {
	now := time.Now().UTC()
	data, err := gdb.ExportData()
	if err != nil {
		slog.Error("Failed to create backup export snapshot", "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup export"})
		return
	}

	document := backup.NewDocument(data, version.Version, now)
	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		slog.Error("Failed to encode backup export", "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to encode backup export"})
		return
	}

	setAttachmentHeaders(c, "application/json; charset=utf-8", "lannventory-backup-"+downloadTimestamp(now)+".json")
	c.Data(http.StatusOK, "application/json; charset=utf-8", append(payload, '\n'))
}

// getInventoryCSVExport godoc
// @Summary      Download current inventory CSV
// @Description  Export the current device inventory as CSV. This is not a full backup and does not include history or events.
// @Tags         export
// @Produce      text/csv
// @Success      200  {string}  string
// @Failure      500  {object}  map[string]string  "CSV generation failure"
// @Router       /export/inventory.csv [get]
func getInventoryCSVExport(c *gin.Context) {
	now := time.Now().UTC()
	currentHosts, err := gdb.ExportCurrentHosts()
	if err != nil {
		slog.Error("Failed to create inventory CSV export snapshot", "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to create inventory export"})
		return
	}

	var payload bytes.Buffer
	if err := backup.WriteInventoryCSV(&payload, currentHosts); err != nil {
		slog.Error("Failed to encode inventory CSV export", "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to encode inventory export"})
		return
	}

	setAttachmentHeaders(c, "text/csv; charset=utf-8", "lannventory-inventory-"+downloadTimestamp(now)+".csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", payload.Bytes())
}

func setAttachmentHeaders(c *gin.Context, contentType, filename string) {
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Cache-Control", "no-store")
}

func downloadTimestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}
