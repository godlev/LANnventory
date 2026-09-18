package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	deviceProfileManufacturerMaxRunes      = 255
	deviceProfileModelMaxRunes             = 255
	deviceProfileManagementAddressMaxRunes = 255
)

// DeviceProfilePatchRequest describes a partial update to manually managed
// generic profile fields.
type DeviceProfilePatchRequest struct {
	Manufacturer      *string `json:"manufacturer,omitempty"`
	Model             *string `json:"model,omitempty"`
	ManagementAddress *string `json:"managementAddress,omitempty"`
}

// DeviceProfileResponse keeps managed profile values explicitly separated from
// future imported/discovered integration state.
type DeviceProfileResponse struct {
	Managed *models.DeviceProfile `json:"managed"`
}

// getHostDeviceProfile godoc
// @Summary      Get managed device profile
// @Description  Return manually managed generic device-profile information. Imported/discovered integration data is stored separately and never overwrites this profile.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string                 true  "Host ID"
// @Success      200  {object}  DeviceProfileResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/profile [get]
func getHostDeviceProfile(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	if strings.TrimSpace(host.Mac) == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no MAC address"})
		return
	}

	profile, found, err := gdb.SelectDeviceProfileByMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load device profile", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load device profile"})
		return
	}
	if !found {
		c.IndentedJSON(http.StatusOK, DeviceProfileResponse{Managed: nil})
		return
	}

	c.IndentedJSON(http.StatusOK, DeviceProfileResponse{Managed: &profile})
}

// setHostDeviceProfile godoc
// @Summary      Update managed device profile
// @Description  Partially update manually managed generic device-profile fields. Empty values clear fields; clearing all fields removes the managed profile row.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "Host ID"
// @Param        body  body      DeviceProfilePatchRequest  true  "Managed profile payload"
// @Success      200   {object}  DeviceProfileResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/profile [patch]
func setHostDeviceProfile(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}
	if strings.TrimSpace(host.Mac) == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no MAC address"})
		return
	}

	var payload DeviceProfilePatchRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	update, hasChanges, err := validateDeviceProfilePatch(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !hasChanges {
		getHostDeviceProfile(c)
		return
	}

	profile, found, err := gdb.UpdateDeviceProfile(host.Mac, update)
	if err != nil {
		slog.Error("Failed to update device profile", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update device profile"})
		return
	}
	if !found {
		c.IndentedJSON(http.StatusOK, DeviceProfileResponse{Managed: nil})
		return
	}

	c.IndentedJSON(http.StatusOK, DeviceProfileResponse{Managed: &profile})
}

func validateDeviceProfilePatch(payload DeviceProfilePatchRequest) (models.DeviceProfileUpdate, bool, error) {
	var update models.DeviceProfileUpdate
	hasChanges := false

	if payload.Manufacturer != nil {
		value, err := validateMetadataText("manufacturer", strings.TrimSpace(*payload.Manufacturer), deviceProfileManufacturerMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Manufacturer = &value
		hasChanges = true
	}
	if payload.Model != nil {
		value, err := validateMetadataText("model", strings.TrimSpace(*payload.Model), deviceProfileModelMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Model = &value
		hasChanges = true
	}
	if payload.ManagementAddress != nil {
		value, err := validateMetadataText("managementAddress", strings.TrimSpace(*payload.ManagementAddress), deviceProfileManagementAddressMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.ManagementAddress = &value
		hasChanges = true
	}

	return update, hasChanges, nil
}
