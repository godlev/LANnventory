package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

const hostNameMaxRunes = 255

// HostInventoryPatchRequest describes a partial atomic host and inventory update.
type HostInventoryPatchRequest struct {
	Name       *string   `json:"name,omitempty"`
	Known      *bool     `json:"known,omitempty"`
	DeviceType *string   `json:"deviceType,omitempty"`
	Owner      *string   `json:"owner,omitempty"`
	Location   *string   `json:"location,omitempty"`
	Notes      *string   `json:"notes,omitempty"`
	Tags       *[]string `json:"tags,omitempty"`
}

// getAllHosts godoc
// @Summary      Get all hosts
// @Description  Retrieve all current hosts from the database, enriched with inventory metadata and lifecycle fields.
// @Tags         hosts
// @Produce      json
// @Success      200  {array}   models.Host
// @Router       /all [get]
func getAllHosts(c *gin.Context) {
	allHosts, ok := gdb.SelectCurrentHostsWithMetadata()
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load hosts"})
		return
	}

	c.IndentedJSON(http.StatusOK, allHosts)
}

// getHost godoc
// @Summary      Get host by ID
// @Description  Retrieve detailed information about a current host by its unique ID, enriched with inventory metadata and lifecycle fields.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Success      200  {object}  models.Host
// @Router       /host/{id} [get]
func getHost(c *gin.Context) {
	idStr := c.Param("id")
	host, err := getHostWithMetadataByID(idStr) // functions.go
	if err != nil {
		if errors.Is(err, errInvalidHostID) {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		slog.Error("Failed to load host", "id", idStr, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load host"})
		return
	}

	_, host.DNS = check.DNS(host)
	c.IndentedJSON(http.StatusOK, host)
}

// setHostInventory godoc
// @Summary      Update host inventory
// @Description  Atomically update editable host fields and manually managed inventory metadata. Actual known, device type, and metadata changes are recorded as Device change events. Name changes do not create events.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "Host ID"
// @Param        body  body      HostInventoryPatchRequest  true  "Host inventory payload"
// @Success      200   {object}  models.Host
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id} [patch]
func setHostInventory(c *gin.Context) {
	idStr := c.Param("id")
	host, err := getHostByID(idStr)
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	var payload HostInventoryPatchRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	update, hasChanges, err := validateHostInventoryPatch(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !hasChanges {
		updatedHost, err := gdb.SelectHostWithMetadataByID(host.ID)
		if err != nil {
			slog.Error("Failed to reload unchanged host inventory", "id", host.ID, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load updated host"})
			return
		}
		c.IndentedJSON(http.StatusOK, updatedHost)
		return
	}

	updatedHost, err := gdb.UpdateHostInventoryWithEvents(host.ID, update)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	if gdb.IsEmptyMetadataMACError(err) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no MAC address"})
		return
	}
	if err != nil {
		slog.Error("Failed to update host inventory", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update host inventory"})
		return
	}

	c.IndentedJSON(http.StatusOK, updatedHost)
}

// setHostDeviceType godoc
// @Summary      Set host device type
// @Description  Update only a host's manually assigned device type
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Host ID"
// @Param        body  body      object  true  "Device type payload"
// @Success      200   {object}  models.Host
// @Router       /host/{id}/type [patch]
func setHostDeviceType(c *gin.Context) {
	idStr := c.Param("id")
	host, err := getHostByID(idStr) // functions.go
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	rawDeviceType, ok := payload["deviceType"]
	if !ok {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "missing deviceType"})
		return
	}

	deviceType, ok := rawDeviceType.(string)
	if !ok || !models.IsValidDeviceType(deviceType) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid deviceType"})
		return
	}

	oldDeviceType := host.DeviceType
	updatedHost, err := gdb.UpdateDeviceType(host.ID, deviceType)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	if err != nil {
		slog.Error("Failed to update host device type", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update host device type"})
		return
	}

	if oldDeviceType != updatedHost.DeviceType {
		gdb.RecordHostEvent(updatedHost, models.EventDeviceTypeChanged, oldDeviceType, updatedHost.DeviceType)
	}

	updatedHost, err = gdb.SelectHostWithMetadataByID(updatedHost.ID)
	if err != nil {
		slog.Error("Failed to reload host metadata after device type update", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load updated host"})
		return
	}

	c.IndentedJSON(http.StatusOK, updatedHost)
}

// delHost godoc
// @Summary      Delete host
// @Description  Remove a host from the database by its unique ID
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Success      200  {string}  string  "OK"
// @Router       /host/del/{id} [get]
func delHost(c *gin.Context) {
	idStr := c.Param("id")
	host, err := getHostByID(idStr) // functions.go
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := gdb.DeleteCurrentHostWithMetadata(host); err != nil {
		slog.Error("Failed to delete host", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to delete host"})
		return
	}
	slog.Info("Deleting from DB", "host", host)
	c.IndentedJSON(http.StatusOK, "OK")
}

func validateHostInventoryPatch(payload HostInventoryPatchRequest) (models.HostInventoryUpdate, bool, error) {
	var update models.HostInventoryUpdate
	hasChanges := false

	if payload.Name != nil {
		name, err := validateMetadataText("name", *payload.Name, hostNameMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Name = &name
		hasChanges = true
	}
	if payload.Known != nil {
		known := 0
		if *payload.Known {
			known = 1
		}
		update.Known = &known
		hasChanges = true
	}
	if payload.DeviceType != nil {
		if !models.IsValidDeviceType(*payload.DeviceType) {
			return update, false, errors.New("invalid deviceType")
		}
		deviceType := *payload.DeviceType
		update.DeviceType = &deviceType
		hasChanges = true
	}

	metadataUpdate, metadataHasChanges, err := validateHostMetadataPatch(HostMetadataPatchRequest{
		Owner:    payload.Owner,
		Location: payload.Location,
		Notes:    payload.Notes,
		Tags:     payload.Tags,
	})
	if err != nil {
		return update, false, err
	}
	if metadataHasChanges {
		update.Owner = metadataUpdate.Owner
		update.Location = metadataUpdate.Location
		update.Notes = metadataUpdate.Notes
		update.Tags = metadataUpdate.Tags
		hasChanges = true
	}

	return update, hasChanges, nil
}

// addHost godoc
// @Summary      Add host manually
// @Description  Add host by MAC, with optional Name, IP, Hardware
// @Description  Returns `models.Host` with this MAC form DB, either just added or existing
// @Tags         hosts
// @Produce      json
// @Param        mac   path      string  true   "Host MAC"
// @Param        name  query     string  false  "Name"
// @Param        ip    query     string  false  "IP"
// @Param        hw    query     string  false  "Hardware"
// @Success      200  {object}  models.Host
// @Router       /host/add/{mac} [get]
func addHost(c *gin.Context) {

	mac := c.Param("mac")
	hosts := gdb.SelectByMAC("now", mac)

	if len(hosts) > 0 {
		slog.Warn("Host with this MAC already exists", "host", hosts[0])
	} else {
		var host models.Host

		host.Mac = mac
		host.Name = c.Query("name")
		host.IP = c.Query("ip")
		host.Hw = c.Query("hw")

		if err := gdb.UpdateWithError("now", host); err != nil {
			slog.Error("Failed to add host", "mac", mac, "err", err)
			c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to add host"})
			return
		}
		hosts = gdb.SelectByMAC("now", mac)
		if len(hosts) > 0 {
			if err := gdb.EnsureHostLifecyclePlaceholder(hosts[0].Mac); err != nil {
				slog.Error("Failed to initialize host lifecycle placeholder", "mac", hosts[0].Mac, "err", err)
				c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize host lifecycle"})
				return
			}
			gdb.RecordHostEvent(hosts[0], models.EventDiscovered, "", "")
		}

		slog.Info("Added host to DB", "host", hosts[0])
	}

	enrichedHost, err := gdb.SelectHostWithMetadataByID(hosts[0].ID)
	if err != nil {
		c.IndentedJSON(http.StatusOK, hosts[0])
		return
	}

	c.IndentedJSON(http.StatusOK, enrichedHost)
}

// editHost godoc
// @Summary      Edit host
// @Description  Update a host's name and optionally toggle its "known" status
// @Tags         hosts
// @Produce      json
// @Param        id     path      string  true  "Host ID"
// @Param        name   path      string  true  "New name for the host"
// @Param        known  path      string  false "Pass 'toggle' to flip the known/unknown status"
// @Success      200    {string}  string  "OK"
// @Router       /edit/{id}/{name}/{known} [get]
func editHost(c *gin.Context) {

	idStr := c.Param("id")
	name := c.Param("name")
	toggleKnown := c.Param("known")

	host, err := getHostByID(idStr) // functions.go
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	host.Name = name
	oldKnown := host.Known

	if toggleKnown == "/toggle" {
		host.Known = 1 - host.Known
	}

	if err := gdb.UpdateWithError("now", host); err != nil {
		slog.Error("Failed to update host", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update host"})
		return
	}

	if oldKnown != host.Known {
		eventType := models.EventUnknown
		if host.Known == 1 {
			eventType = models.EventKnown
		}
		gdb.RecordHostEvent(host, eventType, "", "")
	}

	c.IndentedJSON(http.StatusOK, "OK")
}
