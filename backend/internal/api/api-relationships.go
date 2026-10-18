package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
	"gorm.io/gorm"
)

const (
	relationshipBodyLimit = 16 << 10
	relationshipSearchLimit = 20
	relationshipSearchMaxRunes = 100
)

type RelationshipResponse struct {
	ID              uint64                    `json:"id"`
	Type            relationships.Type        `json:"type"`
	Source          relationships.Endpoint    `json:"source"`
	Target          relationships.Endpoint    `json:"target"`
	Directionality  relationships.Directionality `json:"directionality"`
	SourceLabel     string                    `json:"sourceLabel"`
	InverseLabel    string                    `json:"inverseLabel"`
	Provenance      relationships.Provenance  `json:"provenance"`
	SourceDetail    string                    `json:"sourceDetail,omitempty"`
	SourceInstance  string                    `json:"sourceInstance,omitempty"`
	Note            string                    `json:"note,omitempty"`
	CreatedAt       string                    `json:"createdAt"`
	UpdatedAt       string                    `json:"updatedAt"`
}

type RelationshipTargetResponse struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	DNS        string `json:"dns"`
	IP         string `json:"ip"`
	Mac        string `json:"mac"`
	DeviceType string `json:"deviceType"`
}

type RelationshipNotePatchRequest struct {
	Note *string `json:"note"`
}

// getRelationshipGraph returns one bounded canonical read-only snapshot. It
// resolves existing Proxmox owners/matches without writing another relation.
func getRelationshipGraph(c *gin.Context) {
	var hostID uint64
	if raw := strings.TrimSpace(c.Query("hostId")); raw != "" {
		id, ok := parsePositiveRelationshipID(raw)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hostId"})
			return
		}
		hostID = id
	}
	limit := relationships.DefaultGraphLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > relationships.MaxGraphLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 500"})
			return
		}
		limit = n
	}
	graph, err := gdb.SelectRelationshipGraph(hostID, limit)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}
	if err != nil {
		slog.Error("Failed to project relationship graph", "hostId", hostID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load relationship graph"})
		return
	}
	c.JSON(http.StatusOK, graph)
}

// getRelationshipTypes exposes the canonical registry, including source-owned
// predicates that generic manual CRUD cannot modify.
func getRelationshipTypes(c *gin.Context) {
	c.JSON(http.StatusOK, relationships.Definitions())
}

// getRelationshipTargets returns a maximum of 20 matched CURRENT LAN Hosts.
func getRelationshipTargets(c *gin.Context) {
	q := strings.TrimSpace(c.Query("query"))
	n := utf8.RuneCountInString(q)
	if !utf8.ValidString(q) || n < 2 || n > relationshipSearchMaxRunes || containsRelationshipControl(q) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query must contain 2 to 100 valid characters"})
		return
	}
	var exclude uint64
	if raw := strings.TrimSpace(c.Query("excludeHostId")); raw != "" {
		var ok bool
		exclude, ok = parsePositiveRelationshipID(raw)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid excludeHostId"})
			return
		}
	}
	rows, err := gdb.SelectRelationshipHostTargets(q, exclude, relationshipSearchLimit)
	if err != nil {
		slog.Error("Failed to search manual relationship targets", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search targets"})
		return
	}
	results := make([]RelationshipTargetResponse, 0, len(rows))
	for _, host := range rows {
		results = append(results, RelationshipTargetResponse{
			ID: host.ID, Name: host.Name, DNS: host.DNS, IP: host.IP,
			Mac: host.Mac, DeviceType: host.DeviceType,
		})
	}
	c.JSON(http.StatusOK, results)
}

// getHostRelationships currently lists only persisted manual assertions.
// Phase 38A.4 adds canonical read-only projections of Proxmox HOSTS/MATCHES.
func getHostRelationships(c *gin.Context) {
	id, ok := parsePositiveRelationshipID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid host id"})
		return
	}
	if !relationshipHostExists(c, id) {
		return
	}
	rows, err := gdb.ListManualRelationshipsByHost(id)
	if err != nil {
		slog.Error("Failed to list manual relationships", "hostId", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load relationships"})
		return
	}
	result := make([]RelationshipResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, relationshipResponse(row))
	}
	c.JSON(http.StatusOK, result)
}

func getRelationship(c *gin.Context) {
	id, ok := relationshipIDFromRequest(c)
	if !ok { return }
	row, err := gdb.SelectManualRelationshipByID(id)
	if err != nil {
		relationshipReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, relationshipResponse(row))
}

// createRelationship accepts the domain ManualCreateInput exactly; the request
// cannot set provenance, source instance, createdAt or imported source detail.
func createRelationship(c *gin.Context) {
	var payload relationships.ManualCreateInput
	if !decodeStrictRelationshipJSON(c, &payload) { return }
	row, err := gdb.CreateManualRelationship(payload)
	if err != nil {
		relationshipWriteError(c, err)
		return
	}
	c.Header("Location", "/api/relationships/"+strconv.FormatUint(row.ID, 10))
	c.JSON(http.StatusCreated, relationshipResponse(row))
}

// patchRelationship only allows note editing. Edges cannot be redirected,
// promoted to system/proxmox provenance, or implicitly recreated.
func patchRelationship(c *gin.Context) {
	id, ok := relationshipIDFromRequest(c)
	if !ok { return }
	var payload RelationshipNotePatchRequest
	if !decodeStrictRelationshipJSON(c, &payload) { return }
	if payload.Note == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note field is required"})
		return
	}
	row, err := gdb.UpdateManualRelationshipNote(id, *payload.Note)
	if err != nil {
		relationshipWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, relationshipResponse(row))
}

func deleteRelationship(c *gin.Context) {
	id, ok := relationshipIDFromRequest(c)
	if !ok { return }
	if err := gdb.DeleteManualRelationship(id); err != nil {
		relationshipWriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func relationshipResponse(row models.RelationshipAssertion) RelationshipResponse {
	def, _ := relationships.LookupType(relationships.Type(row.RelationshipType))
	return RelationshipResponse{
		ID: row.ID, Type: relationships.Type(row.RelationshipType),
		Source: relationships.Endpoint{EntityType: relationships.EntityType(row.SourceEntityType), EntityID: row.SourceEntityID},
		Target: relationships.Endpoint{EntityType: relationships.EntityType(row.TargetEntityType), EntityID: row.TargetEntityID},
		Directionality: def.Directionality,
		SourceLabel: def.SourceLabel, InverseLabel: def.InverseLabel,
		Provenance: relationships.Provenance(row.Provenance),
		SourceDetail: row.SourceDetail, SourceInstance: row.SourceInstance,
		Note: row.Note, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func relationshipHostExists(c *gin.Context, id uint64) bool {
	host, err := gdb.SelectHostWithMetadataByID(int(id))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && host.ID < 1) {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return false
	}
	if err != nil {
		slog.Error("Failed to check current Host for relationships", "hostId", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load host"})
		return false
	}
	return true
}

func relationshipIDFromRequest(c *gin.Context) (uint64, bool) {
	id, ok := parsePositiveRelationshipID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid relationship id"})
	}
	return id, ok
}

func parsePositiveRelationshipID(value string) (uint64, bool) {
	id, err := strconv.ParseUint(value, 10, 64)
	return id, err == nil && id > 0 && id <= uint64(^uint(0)>>1)
}

func containsRelationshipControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func decodeStrictRelationshipJSON(c *gin.Context, destination any) bool {
	if c.Request.Body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, relationshipBodyLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	return true
}

func relationshipReadError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "manual relationship not found"})
		return
	}
	slog.Error("Failed to read manual relationship", "err", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load relationship"})
}

func relationshipWriteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gdb.ErrRelationshipAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "relationship already exists"})
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, relationships.ErrEntityNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "manual relationship or endpoint not found"})
	case errors.Is(err, relationships.ErrInvalidEntityType),
		errors.Is(err, relationships.ErrInvalidEndpoint),
		errors.Is(err, relationships.ErrInvalidType),
		errors.Is(err, relationships.ErrEndpointCombination),
		errors.Is(err, relationships.ErrSelfRelationship),
		errors.Is(err, relationships.ErrManualNotAllowed),
		errors.Is(err, relationships.ErrInvalidNote),
		errors.Is(err, relationships.ErrNoteTooLong),
		errors.Is(err, relationships.ErrInvalidProvenance):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		slog.Error("Failed to write manual relationship", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save relationship"})
	}
}
