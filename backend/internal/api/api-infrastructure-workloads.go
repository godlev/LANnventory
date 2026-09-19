package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const (
	workloadNativeIDMaxRunes      = 128
	workloadNameMaxRunes          = 255
	workloadInterfaceNameMaxRunes = 128
	workloadNetworkTextMaxRunes   = 255
)

type InfrastructureWorkloadInterfaceRequest struct {
	Name              string `json:"name"`
	Mac               string `json:"mac,omitempty"`
	Bridge            string `json:"bridge,omitempty"`
	VLANTag           string `json:"vlanTag,omitempty"`
	ConfiguredAddress string `json:"configuredAddress,omitempty"`
	ConfiguredNetwork string `json:"configuredNetwork,omitempty"`
}

type InfrastructureWorkloadCreateRequest struct {
	NativeID     string                                   `json:"nativeId"`
	WorkloadType string                                   `json:"workloadType"`
	Name         string                                   `json:"name,omitempty"`
	Status       string                                   `json:"status,omitempty"`
	Interfaces   []InfrastructureWorkloadInterfaceRequest `json:"interfaces,omitempty"`
}

type InfrastructureWorkloadPatchRequest struct {
	Name       *string                                    `json:"name,omitempty"`
	Status     *string                                    `json:"status,omitempty"`
	Interfaces *[]InfrastructureWorkloadInterfaceRequest  `json:"interfaces,omitempty"`
}

type InfrastructureWorkloadLinkRequest struct {
	HostID int `json:"hostId"`
}

type InfrastructureWorkloadMatchedHost struct {
	HostID     int    `json:"hostId"`
	Mac        string `json:"mac"`
	Name       string `json:"name"`
	IP         string `json:"ip"`
	DeviceType string `json:"deviceType"`
}

type InfrastructureWorkloadResponse struct {
	models.InfrastructureWorkload
	Interfaces  []models.InfrastructureWorkloadInterface `json:"interfaces"`
	Link        *models.InfrastructureWorkloadHostLink    `json:"link"`
	MatchedHost *InfrastructureWorkloadMatchedHost        `json:"matchedHost"`
}

// getHostInfrastructureWorkloads godoc
// @Summary      Get hypervisor workloads
// @Description  Return infrastructure workloads hosted by one configured hypervisor. Workloads remain separate from LANnventory Hosts.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Hypervisor Host ID"
// @Success      200  {array}   InfrastructureWorkloadResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/workloads [get]
func getHostInfrastructureWorkloads(c *gin.Context) {
	host, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	records, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load infrastructure workloads", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load workloads"})
		return
	}

	response := make([]InfrastructureWorkloadResponse, 0, len(records))
	for _, record := range records {
		response = append(response, infrastructureWorkloadResponse(record))
	}
	c.IndentedJSON(http.StatusOK, response)
}

// createHostInfrastructureWorkload godoc
// @Summary      Add or update a manual hypervisor workload
// @Description  Add or update a manual VM/LXC inventory object by native ID and type. This never creates a LANnventory Host.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                               true  "Hypervisor Host ID"
// @Param        body  body      InfrastructureWorkloadCreateRequest  true  "Manual workload"
// @Success      200   {object}  InfrastructureWorkloadResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/workloads [post]
func createHostInfrastructureWorkload(c *gin.Context) {
	host, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	var payload InfrastructureWorkloadCreateRequest
	if !decodeStrictWorkloadJSON(c, &payload) {
		return
	}
	input, err := validateInfrastructureWorkloadCreate(payload)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(host.Mac)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to check existing workloads"})
		return
	}
	for _, candidate := range existing {
		if candidate.Workload.NativeID == input.NativeID &&
			candidate.Workload.WorkloadType == input.WorkloadType &&
			candidate.Workload.Source != models.InfrastructureWorkloadSourceManual {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "imported workloads cannot be overwritten through manual mode"})
			return
		}
	}

	record, err := gdb.UpsertInfrastructureWorkload(host.Mac, input, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		slog.Error("Failed to save manual infrastructure workload", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to save workload"})
		return
	}
	c.IndentedJSON(http.StatusOK, infrastructureWorkloadResponse(record))
}

// patchHostInfrastructureWorkload godoc
// @Summary      Edit a manual hypervisor workload
// @Description  Update manually maintained workload fields while keeping native ID and workload type immutable.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id          path      string                              true  "Hypervisor Host ID"
// @Param        workloadId  path      string                              true  "Workload ID"
// @Param        body        body      InfrastructureWorkloadPatchRequest  true  "Manual workload changes"
// @Success      200         {object}  InfrastructureWorkloadResponse
// @Failure      400         {object}  map[string]string
// @Failure      500         {object}  map[string]string
// @Router       /host/{id}/workloads/{workloadId} [patch]
func patchHostInfrastructureWorkload(c *gin.Context) {
	host, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}
	record, ok := workloadRecordFromRequest(c, host)
	if !ok {
		return
	}
	if record.Workload.Source != models.InfrastructureWorkloadSourceManual {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "only manual workloads can be edited directly"})
		return
	}

	var payload InfrastructureWorkloadPatchRequest
	if !decodeStrictWorkloadJSON(c, &payload) {
		return
	}

	name := record.Workload.Name
	status := record.Workload.Status
	interfaces := requestsFromWorkloadInterfaces(record.Interfaces)
	if payload.Name != nil {
		name = *payload.Name
	}
	if payload.Status != nil {
		status = *payload.Status
	}
	if payload.Interfaces != nil {
		interfaces = *payload.Interfaces
	}

	input, err := validateInfrastructureWorkloadCreate(InfrastructureWorkloadCreateRequest{
		NativeID:     record.Workload.NativeID,
		WorkloadType: record.Workload.WorkloadType,
		Name:         name,
		Status:       status,
		Interfaces:   interfaces,
	})
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.Source = models.InfrastructureWorkloadSourceManual

	updated, err := gdb.UpsertInfrastructureWorkload(host.Mac, input, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		slog.Error("Failed to update manual infrastructure workload", "workloadID", record.Workload.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to update workload"})
		return
	}
	c.IndentedJSON(http.StatusOK, infrastructureWorkloadResponse(updated))
}

// deleteHostInfrastructureWorkload godoc
// @Summary      Delete a manual hypervisor workload
// @Description  Delete a manually maintained workload inventory object. Imported workload retirement uses snapshot reconciliation instead.
// @Tags         hosts
// @Produce      json
// @Param        id          path      string  true  "Hypervisor Host ID"
// @Param        workloadId  path      string  true  "Workload ID"
// @Success      200         {object}  map[string]string
// @Failure      400         {object}  map[string]string
// @Failure      500         {object}  map[string]string
// @Router       /host/{id}/workloads/{workloadId} [delete]
func deleteHostInfrastructureWorkload(c *gin.Context) {
	host, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}
	record, ok := workloadRecordFromRequest(c, host)
	if !ok {
		return
	}

	if err := gdb.DeleteManualInfrastructureWorkload(record.Workload.ID, host.Mac); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "only manual workloads") {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		slog.Error("Failed to delete manual infrastructure workload", "workloadID", record.Workload.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to delete workload"})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"status": "deleted"})
}

// setHostInfrastructureWorkloadLink godoc
// @Summary      Link workload to an existing Host
// @Description  Create or replace the logical MATCHES relation without merging workload or Host identity/history.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id          path      string                             true  "Hypervisor Host ID"
// @Param        workloadId  path      string                             true  "Workload ID"
// @Param        body        body      InfrastructureWorkloadLinkRequest  true  "Target Host"
// @Success      200         {object}  InfrastructureWorkloadResponse
// @Failure      400         {object}  map[string]string
// @Failure      500         {object}  map[string]string
// @Router       /host/{id}/workloads/{workloadId}/link [put]
func setHostInfrastructureWorkloadLink(c *gin.Context) {
	hypervisor, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}
	record, ok := workloadRecordFromRequest(c, hypervisor)
	if !ok {
		return
	}

	var payload InfrastructureWorkloadLinkRequest
	if !decodeStrictWorkloadJSON(c, &payload) {
		return
	}
	if payload.HostID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "valid hostId is required"})
		return
	}

	target, err := gdb.SelectHostWithMetadataByID(payload.HostID)
	if errors.Is(err, gorm.ErrRecordNotFound) || target.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "target host does not exist"})
		return
	}
	if err != nil {
		slog.Error("Failed to load workload link target", "targetHostID", payload.HostID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load target host"})
		return
	}
	if strings.EqualFold(strings.TrimSpace(target.Mac), strings.TrimSpace(hypervisor.Mac)) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "workload cannot match its source hypervisor host"})
		return
	}

	if _, err := gdb.SetInfrastructureWorkloadHostLink(
		record.Workload.ID,
		target,
		models.InfrastructureWorkloadLinkSourceManual,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		slog.Error("Failed to link infrastructure workload", "workloadID", record.Workload.ID, "targetHostID", target.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to link workload"})
		return
	}

	updated, found, err := gdb.SelectInfrastructureWorkloadByID(record.Workload.ID)
	if err != nil || !found {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to reload linked workload"})
		return
	}
	c.IndentedJSON(http.StatusOK, infrastructureWorkloadResponse(updated))
}

// deleteHostInfrastructureWorkloadLink godoc
// @Summary      Unlink workload from Host
// @Description  Remove only the logical MATCHES relation. Workload and Host records remain unchanged.
// @Tags         hosts
// @Produce      json
// @Param        id          path      string  true  "Hypervisor Host ID"
// @Param        workloadId  path      string  true  "Workload ID"
// @Success      200         {object}  InfrastructureWorkloadResponse
// @Failure      400         {object}  map[string]string
// @Failure      500         {object}  map[string]string
// @Router       /host/{id}/workloads/{workloadId}/link [delete]
func deleteHostInfrastructureWorkloadLink(c *gin.Context) {
	hypervisor, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return
	}
	record, ok := workloadRecordFromRequest(c, hypervisor)
	if !ok {
		return
	}

	if err := gdb.DeleteInfrastructureWorkloadHostLink(record.Workload.ID); err != nil {
		slog.Error("Failed to unlink infrastructure workload", "workloadID", record.Workload.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to unlink workload"})
		return
	}
	updated, found, err := gdb.SelectInfrastructureWorkloadByID(record.Workload.ID)
	if err != nil || !found {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to reload unlinked workload"})
		return
	}
	c.IndentedJSON(http.StatusOK, infrastructureWorkloadResponse(updated))
}

func workloadHypervisorHostFromRequest(c *gin.Context) (models.Host, bool) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return models.Host{}, false
	}
	if host.DeviceType != "server" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "workloads require Device Type Server"})
		return models.Host{}, false
	}
	if _, found, err := gdb.SelectHypervisorProfileByMAC(host.Mac); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load hypervisor profile"})
		return models.Host{}, false
	} else if !found {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "host is not configured as a hypervisor"})
		return models.Host{}, false
	}
	return host, true
}

func workloadRecordFromRequest(c *gin.Context, hypervisor models.Host) (models.InfrastructureWorkloadRecord, bool) {
	rawID := strings.TrimSpace(c.Param("workloadId"))
	parsed, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || parsed == 0 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid workload id"})
		return models.InfrastructureWorkloadRecord{}, false
	}
	record, found, err := gdb.SelectInfrastructureWorkloadByID(uint(parsed))
	if err != nil {
		slog.Error("Failed to load infrastructure workload", "workloadID", parsed, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load workload"})
		return models.InfrastructureWorkloadRecord{}, false
	}
	if !found || !strings.EqualFold(record.Workload.HypervisorMac, hypervisor.Mac) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "workload does not belong to this hypervisor"})
		return models.InfrastructureWorkloadRecord{}, false
	}
	return record, true
}

func infrastructureWorkloadResponse(record models.InfrastructureWorkloadRecord) InfrastructureWorkloadResponse {
	response := InfrastructureWorkloadResponse{
		InfrastructureWorkload: record.Workload,
		Interfaces:             record.Interfaces,
		Link:                   record.Link,
	}
	if response.Interfaces == nil {
		response.Interfaces = []models.InfrastructureWorkloadInterface{}
	}
	if record.Link != nil {
		if host, err := gdb.SelectHostWithMetadataByID(record.Link.HostID); err == nil &&
			host.ID > 0 &&
			strings.EqualFold(strings.TrimSpace(host.Mac), strings.TrimSpace(record.Link.HostMac)) {
			response.MatchedHost = &InfrastructureWorkloadMatchedHost{
				HostID:     host.ID,
				Mac:        host.Mac,
				Name:       host.Name,
				IP:         host.IP,
				DeviceType: host.DeviceType,
			}
		}
	}
	return response
}

func validateInfrastructureWorkloadCreate(payload InfrastructureWorkloadCreateRequest) (models.InfrastructureWorkloadUpsert, error) {
	nativeID, err := validateMetadataText("nativeId", strings.TrimSpace(payload.NativeID), workloadNativeIDMaxRunes, false)
	if err != nil || nativeID == "" {
		if err != nil {
			return models.InfrastructureWorkloadUpsert{}, err
		}
		return models.InfrastructureWorkloadUpsert{}, errors.New("nativeId is required")
	}
	workloadType := strings.ToLower(strings.TrimSpace(payload.WorkloadType))
	if workloadType != models.InfrastructureWorkloadTypeVM && workloadType != models.InfrastructureWorkloadTypeContainer {
		return models.InfrastructureWorkloadUpsert{}, errors.New("workloadType must be vm or container")
	}
	name, err := validateMetadataText("name", strings.TrimSpace(payload.Name), workloadNameMaxRunes, false)
	if err != nil {
		return models.InfrastructureWorkloadUpsert{}, err
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "" {
		status = models.InfrastructureWorkloadStatusUnknown
	}
	if status != models.InfrastructureWorkloadStatusUnknown &&
		status != models.InfrastructureWorkloadStatusRunning &&
		status != models.InfrastructureWorkloadStatusStopped {
		return models.InfrastructureWorkloadUpsert{}, errors.New("status must be unknown, running, or stopped")
	}

	interfaces := make([]models.InfrastructureWorkloadInterface, 0, len(payload.Interfaces))
	names := make(map[string]struct{}, len(payload.Interfaces))
	for _, raw := range payload.Interfaces {
		iface, err := validateInfrastructureWorkloadInterface(raw)
		if err != nil {
			return models.InfrastructureWorkloadUpsert{}, err
		}
		if _, exists := names[iface.Name]; exists {
			return models.InfrastructureWorkloadUpsert{}, errors.New("duplicate workload interface name")
		}
		names[iface.Name] = struct{}{}
		interfaces = append(interfaces, iface)
	}

	return models.InfrastructureWorkloadUpsert{
		NativeID:     nativeID,
		WorkloadType: workloadType,
		Name:         name,
		Status:       status,
		Source:       models.InfrastructureWorkloadSourceManual,
		Interfaces:   interfaces,
	}, nil
}

func validateInfrastructureWorkloadInterface(payload InfrastructureWorkloadInterfaceRequest) (models.InfrastructureWorkloadInterface, error) {
	name, err := validateMetadataText("interface.name", strings.TrimSpace(payload.Name), workloadInterfaceNameMaxRunes, false)
	if err != nil || name == "" {
		if err != nil {
			return models.InfrastructureWorkloadInterface{}, err
		}
		return models.InfrastructureWorkloadInterface{}, errors.New("interface name is required")
	}

	values := []struct {
		field string
		value string
	}{
		{"interface.mac", payload.Mac},
		{"interface.bridge", payload.Bridge},
		{"interface.vlanTag", payload.VLANTag},
		{"interface.configuredAddress", payload.ConfiguredAddress},
		{"interface.configuredNetwork", payload.ConfiguredNetwork},
	}
	validated := make([]string, len(values))
	for i, item := range values {
		value, err := validateMetadataText(item.field, strings.TrimSpace(item.value), workloadNetworkTextMaxRunes, false)
		if err != nil {
			return models.InfrastructureWorkloadInterface{}, err
		}
		validated[i] = value
	}

	return models.InfrastructureWorkloadInterface{
		Name:              name,
		Mac:               validated[0],
		Bridge:            validated[1],
		VLANTag:           validated[2],
		ConfiguredAddress: validated[3],
		ConfiguredNetwork: validated[4],
	}, nil
}

func requestsFromWorkloadInterfaces(interfaces []models.InfrastructureWorkloadInterface) []InfrastructureWorkloadInterfaceRequest {
	response := make([]InfrastructureWorkloadInterfaceRequest, 0, len(interfaces))
	for _, iface := range interfaces {
		response = append(response, InfrastructureWorkloadInterfaceRequest{
			Name:              iface.Name,
			Mac:               iface.Mac,
			Bridge:            iface.Bridge,
			VLANTag:           iface.VLANTag,
			ConfiguredAddress: iface.ConfiguredAddress,
			ConfiguredNetwork: iface.ConfiguredNetwork,
		})
	}
	return response
}

func decodeStrictWorkloadJSON(c *gin.Context, destination any) bool {
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
