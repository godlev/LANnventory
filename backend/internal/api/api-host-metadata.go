package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	hostMetadataOwnerMaxRunes    = 120
	hostMetadataLocationMaxRunes = 120
	hostMetadataNotesMaxRunes    = 4000
	hostMetadataTagsMax          = 20
	hostMetadataTagMaxRunes      = 48
)

// HostMetadataPatchRequest describes a partial host metadata update.
type HostMetadataPatchRequest struct {
	Owner    *string   `json:"owner,omitempty"`
	Location *string   `json:"location,omitempty"`
	Notes    *string   `json:"notes,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
	Pinned   *bool     `json:"pinned,omitempty"`
}

// setHostMetadata godoc
// @Summary      Update host metadata
// @Description  Partially update manually managed inventory metadata. Tags are trimmed, empty tags are removed, duplicates are removed case-insensitively, and user order is preserved.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                    true  "Host ID"
// @Param        body  body      HostMetadataPatchRequest  true  "Metadata payload"
// @Success      200   {object}  models.Host
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/metadata [patch]
func setHostMetadata(c *gin.Context) {
	idStr := c.Param("id")
	host, err := getHostByID(idStr)
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	if host.Mac == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no MAC address"})
		return
	}

	var payload HostMetadataPatchRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	update, hasChanges, err := validateHostMetadataPatch(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if hasChanges {
		if _, err := gdb.UpsertHostMetadata(host.Mac, update); err != nil {
			slog.Error("Failed to update host metadata", "id", host.ID, "mac", host.Mac, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update host metadata"})
			return
		}
	}

	updatedHost, err := gdb.SelectHostWithMetadataByID(host.ID)
	if err != nil {
		slog.Error("Failed to reload host metadata", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load updated host"})
		return
	}

	c.IndentedJSON(http.StatusOK, updatedHost)
}

func validateHostMetadataPatch(payload HostMetadataPatchRequest) (models.HostMetadataUpdate, bool, error) {
	var update models.HostMetadataUpdate
	hasChanges := false

	if payload.Owner != nil {
		owner, err := validateMetadataText("owner", strings.TrimSpace(*payload.Owner), hostMetadataOwnerMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Owner = &owner
		hasChanges = true
	}
	if payload.Location != nil {
		location, err := validateMetadataText("location", strings.TrimSpace(*payload.Location), hostMetadataLocationMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Location = &location
		hasChanges = true
	}
	if payload.Notes != nil {
		notes, err := validateMetadataText("notes", *payload.Notes, hostMetadataNotesMaxRunes, true)
		if err != nil {
			return update, false, err
		}
		update.Notes = &notes
		hasChanges = true
	}
	if payload.Tags != nil {
		tags, err := normalizeMetadataTags(*payload.Tags)
		if err != nil {
			return update, false, err
		}
		update.Tags = &tags
		hasChanges = true
	}
	if payload.Pinned != nil {
		update.Pinned = payload.Pinned
		hasChanges = true
	}

	return update, hasChanges, nil
}

func validateMetadataText(fieldName, value string, maxRunes int, allowLineBreaks bool) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New(fieldName + " must be valid UTF-8")
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return "", errors.New(fieldName + " is too long")
	}
	if hasDisallowedControl(value, allowLineBreaks) {
		return "", errors.New(fieldName + " contains unsupported control characters")
	}

	return value, nil
}

func normalizeMetadataTags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))

	for _, rawTag := range tags {
		tag, err := validateMetadataText("tag", strings.TrimSpace(rawTag), hostMetadataTagMaxRunes, false)
		if err != nil {
			return nil, err
		}
		if tag == "" {
			continue
		}

		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
		if len(normalized) > hostMetadataTagsMax {
			return nil, errors.New("too many tags")
		}
	}

	return normalized, nil
}

func hasDisallowedControl(value string, allowLineBreaks bool) bool {
	for _, char := range value {
		if !unicode.IsControl(char) {
			continue
		}
		if allowLineBreaks && (char == '\n' || char == '\r' || char == '\t') {
			continue
		}
		return true
	}

	return false
}
