package api

import (
	"encoding/json"
	"errors"
	"io"
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
	deviceProfileTextMaxRunes              = 255
	networkPortCapabilityNotesMaxRunes     = 2000
	maxPhysicalPortCount                   = 65535
)

type DeviceProfilePatchRequest struct {
	Manufacturer      *string `json:"manufacturer,omitempty"`
	Model             *string `json:"model,omitempty"`
	ManagementAddress *string `json:"managementAddress,omitempty"`
}

type NetworkDeviceProfilePatchRequest struct {
	ManagementMode      *string `json:"managementMode,omitempty"`
	PhysicalPortCount   *int    `json:"physicalPortCount,omitempty"`
	PortCapabilityNotes *string `json:"portCapabilityNotes,omitempty"`
}

type SystemDeviceProfilePatchRequest struct {
	Role            *string `json:"role,omitempty"`
	OperatingSystem *string `json:"operatingSystem,omitempty"`
	Version         *string `json:"version,omitempty"`
}

type HypervisorProfilePatchRequest struct {
	Platform    *string `json:"platform,omitempty"`
	Version     *string `json:"version,omitempty"`
	NodeName    *string `json:"nodeName,omitempty"`
	ClusterName *string `json:"clusterName,omitempty"`
}

// DeviceProfileResponse keeps every user-managed profile layer separate from
// future imported/discovered integration state.
type DeviceProfileResponse struct {
	Managed    *models.DeviceProfile        `json:"managed"`
	Network    *models.NetworkDeviceProfile `json:"network"`
	System     *models.SystemDeviceProfile  `json:"system"`
	Hypervisor *models.HypervisorProfile    `json:"hypervisor"`
}

// getHostDeviceProfile godoc
// @Summary      Get managed device profile
// @Description  Return manually managed generic and typed device-profile information. Imported/discovered integration data is stored separately and never overwrites these fields.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string                 true  "Host ID"
// @Success      200  {object}  DeviceProfileResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/profile [get]
func getHostDeviceProfile(c *gin.Context) {
	host, ok := profileHostFromRequest(c)
	if !ok {
		return
	}
	writeDeviceProfileResponse(c, host)
}

// setHostDeviceProfile godoc
// @Summary      Update managed device profile
// @Description  Partially update manually managed generic device-profile fields. Empty values clear fields; clearing all fields removes the managed base profile row.
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
	host, ok := profileHostFromRequest(c)
	if !ok {
		return
	}
	var payload DeviceProfilePatchRequest
	if !decodeStrictProfileJSON(c, &payload) {
		return
	}
	update, changed, err := validateDeviceProfilePatch(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if changed {
		if _, _, err := gdb.UpdateDeviceProfile(host.Mac, update); err != nil {
			profileWriteError(c, "generic", host, err)
			return
		}
	}
	writeDeviceProfileResponse(c, host)
}

// setHostNetworkDeviceProfile godoc
// @Summary      Update managed network profile
// @Description  Partially update network-device-specific managed fields. Physical port count 0 means not set.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                            true  "Host ID"
// @Param        body  body      NetworkDeviceProfilePatchRequest  true  "Network profile payload"
// @Success      200   {object}  DeviceProfileResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/profile/network [patch]
func setHostNetworkDeviceProfile(c *gin.Context) {
	host, ok := profileHostFromRequest(c)
	if !ok {
		return
	}
	var payload NetworkDeviceProfilePatchRequest
	if !decodeStrictProfileJSON(c, &payload) {
		return
	}
	update, changed, err := validateNetworkDeviceProfilePatch(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if changed {
		if _, _, err := gdb.UpdateNetworkDeviceProfile(host.Mac, update); err != nil {
			profileWriteError(c, "network", host, err)
			return
		}
	}
	writeDeviceProfileResponse(c, host)
}

// setHostSystemDeviceProfile godoc
// @Summary      Update managed system profile
// @Description  Partially update lightweight Server/NAS system inventory without changing the Host Device Type.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                           true  "Host ID"
// @Param        body  body      SystemDeviceProfilePatchRequest  true  "System profile payload"
// @Success      200   {object}  DeviceProfileResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/profile/system [patch]
func setHostSystemDeviceProfile(c *gin.Context) {
	host, ok := profileHostFromRequest(c)
	if !ok {
		return
	}
	var payload SystemDeviceProfilePatchRequest
	if !decodeStrictProfileJSON(c, &payload) {
		return
	}
	update, changed, err := validateSystemDeviceProfilePatch(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if changed {
		if _, _, err := gdb.UpdateSystemDeviceProfile(host.Mac, update); err != nil {
			profileWriteError(c, "system", host, err)
			return
		}
	}
	writeDeviceProfileResponse(c, host)
}

// setHostHypervisorProfile godoc
// @Summary      Create or update managed hypervisor profile
// @Description  Mark a Host as a managed hypervisor capability and store manually maintained platform details. This does not change Device Type and does not use credentials.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                         true  "Host ID"
// @Param        body  body      HypervisorProfilePatchRequest  true  "Hypervisor profile payload"
// @Success      200   {object}  DeviceProfileResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/profile/hypervisor [patch]
func setHostHypervisorProfile(c *gin.Context) {
	host, ok := profileHostFromRequest(c)
	if !ok {
		return
	}
	if host.DeviceType != "server" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "hypervisor profile requires Device Type Server"})
		return
	}
	var payload HypervisorProfilePatchRequest
	if !decodeStrictProfileJSON(c, &payload) {
		return
	}
	update, changed, err := validateHypervisorProfilePatch(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, exists, err := gdb.SelectHypervisorProfileByMAC(host.Mac)
	if err != nil {
		profileWriteError(c, "hypervisor", host, err)
		return
	}
	if !exists && update.Platform == nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "platform is required when enabling hypervisor profile"})
		return
	}
	if changed {
		if _, _, err := gdb.UpdateHypervisorProfile(host.Mac, update); err != nil {
			profileWriteError(c, "hypervisor", host, err)
			return
		}
	}
	writeDeviceProfileResponse(c, host)
}

// deleteHostHypervisorProfile godoc
// @Summary      Remove managed hypervisor profile
// @Description  Remove only the manual Hypervisor capability/profile. The Host and its generic/system inventory remain unchanged.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Success      200  {object}  DeviceProfileResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/profile/hypervisor [delete]
func deleteHostHypervisorProfile(c *gin.Context) {
	host, ok := profileHostFromRequest(c)
	if !ok {
		return
	}
	if err := gdb.DeleteHypervisorProfileByMAC(host.Mac); err != nil {
		profileWriteError(c, "hypervisor", host, err)
		return
	}
	writeDeviceProfileResponse(c, host)
}

func loadDeviceProfileResponse(mac string) (DeviceProfileResponse, error) {
	var response DeviceProfileResponse
	if profile, found, err := gdb.SelectDeviceProfileByMAC(mac); err != nil {
		return response, err
	} else if found {
		response.Managed = &profile
	}
	if profile, found, err := gdb.SelectNetworkDeviceProfileByMAC(mac); err != nil {
		return response, err
	} else if found {
		response.Network = &profile
	}
	if profile, found, err := gdb.SelectSystemDeviceProfileByMAC(mac); err != nil {
		return response, err
	} else if found {
		response.System = &profile
	}
	if profile, found, err := gdb.SelectHypervisorProfileByMAC(mac); err != nil {
		return response, err
	} else if found {
		response.Hypervisor = &profile
	}
	return response, nil
}

func writeDeviceProfileResponse(c *gin.Context, host models.Host) {
	response, err := loadDeviceProfileResponse(host.Mac)
	if err != nil {
		slog.Error("Failed to load device profile", "id", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load device profile"})
		return
	}
	c.IndentedJSON(http.StatusOK, response)
}

func profileHostFromRequest(c *gin.Context) (models.Host, bool) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return models.Host{}, false
	}
	if strings.TrimSpace(host.Mac) == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host has no MAC address"})
		return models.Host{}, false
	}
	return host, true
}

func decodeStrictProfileJSON(c *gin.Context, destination any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	return true
}

func profileWriteError(c *gin.Context, layer string, host models.Host, err error) {
	slog.Error("Failed to update device profile", "layer", layer, "id", host.ID, "mac", host.Mac, "err", err)
	c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update device profile"})
}

func validateDeviceProfilePatch(payload DeviceProfilePatchRequest) (models.DeviceProfileUpdate, bool, error) {
	var update models.DeviceProfileUpdate
	changed := false
	if payload.Manufacturer != nil {
		value, err := profileText("manufacturer", *payload.Manufacturer, deviceProfileManufacturerMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Manufacturer = &value
		changed = true
	}
	if payload.Model != nil {
		value, err := profileText("model", *payload.Model, deviceProfileModelMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Model = &value
		changed = true
	}
	if payload.ManagementAddress != nil {
		value, err := profileText("managementAddress", *payload.ManagementAddress, deviceProfileManagementAddressMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.ManagementAddress = &value
		changed = true
	}
	return update, changed, nil
}

func validateNetworkDeviceProfilePatch(payload NetworkDeviceProfilePatchRequest) (models.NetworkDeviceProfileUpdate, bool, error) {
	var update models.NetworkDeviceProfileUpdate
	changed := false
	if payload.ManagementMode != nil {
		value := strings.ToLower(strings.TrimSpace(*payload.ManagementMode))
		if !models.IsValidNetworkManagementMode(value) {
			return update, false, errors.New("managementMode must be managed, unmanaged, or empty")
		}
		update.ManagementMode = &value
		changed = true
	}
	if payload.PhysicalPortCount != nil {
		if *payload.PhysicalPortCount < 0 || *payload.PhysicalPortCount > maxPhysicalPortCount {
			return update, false, errors.New("physicalPortCount must be between 0 and 65535")
		}
		update.PhysicalPortCount = payload.PhysicalPortCount
		changed = true
	}
	if payload.PortCapabilityNotes != nil {
		value, err := profileText("portCapabilityNotes", *payload.PortCapabilityNotes, networkPortCapabilityNotesMaxRunes, true)
		if err != nil {
			return update, false, err
		}
		update.PortCapabilityNotes = &value
		changed = true
	}
	return update, changed, nil
}

func validateSystemDeviceProfilePatch(payload SystemDeviceProfilePatchRequest) (models.SystemDeviceProfileUpdate, bool, error) {
	var update models.SystemDeviceProfileUpdate
	changed := false
	if payload.Role != nil {
		value, err := profileText("role", *payload.Role, deviceProfileTextMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Role = &value
		changed = true
	}
	if payload.OperatingSystem != nil {
		value, err := profileText("operatingSystem", *payload.OperatingSystem, deviceProfileTextMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.OperatingSystem = &value
		changed = true
	}
	if payload.Version != nil {
		value, err := profileText("version", *payload.Version, deviceProfileTextMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Version = &value
		changed = true
	}
	return update, changed, nil
}

func validateHypervisorProfilePatch(payload HypervisorProfilePatchRequest) (models.HypervisorProfileUpdate, bool, error) {
	var update models.HypervisorProfileUpdate
	changed := false
	if payload.Platform != nil {
		value := strings.ToLower(strings.TrimSpace(*payload.Platform))
		if !models.IsValidHypervisorPlatform(value) {
			return update, false, errors.New("platform must be proxmox-ve, vmware-esxi, hyper-v, or other")
		}
		update.Platform = &value
		changed = true
	}
	if payload.Version != nil {
		value, err := profileText("version", *payload.Version, deviceProfileTextMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.Version = &value
		changed = true
	}
	if payload.NodeName != nil {
		value, err := profileText("nodeName", *payload.NodeName, deviceProfileTextMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.NodeName = &value
		changed = true
	}
	if payload.ClusterName != nil {
		value, err := profileText("clusterName", *payload.ClusterName, deviceProfileTextMaxRunes, false)
		if err != nil {
			return update, false, err
		}
		update.ClusterName = &value
		changed = true
	}
	return update, changed, nil
}

func profileText(field, value string, maxRunes int, allowLineBreaks bool) (string, error) {
	return validateMetadataText(field, strings.TrimSpace(value), maxRunes, allowLineBreaks)
}
