package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

const maxProxmoxImportBodyBytes int64 = 2 * 1024 * 1024


// ProxmoxImportSnapshotDoc mirrors the collector JSON contract for generated
// API documentation while runtime validation remains in proxmoxsnapshot.
type ProxmoxImportSnapshotDoc struct {
	SchemaVersion    int                         `json:"schemaVersion"`
	CollectorVersion string                      `json:"collectorVersion"`
	Source           string                      `json:"source"`
	CollectedAt      string                      `json:"collectedAt"`
	Complete         bool                        `json:"complete"`
	CollectionErrors []string                    `json:"collectionErrors,omitempty"`
	Node             ProxmoxImportNodeDoc        `json:"node"`
	Workloads        []ProxmoxImportWorkloadDoc  `json:"workloads"`
}

type ProxmoxImportNodeDoc struct {
	Hostname    string `json:"hostname"`
	PVEVersion  string `json:"pveVersion"`
	ClusterName string `json:"clusterName,omitempty"`
	Status      string `json:"status"`
}

type ProxmoxImportWorkloadDoc struct {
	NativeID     string                       `json:"nativeId"`
	WorkloadType string                       `json:"workloadType"`
	NodeName     string                       `json:"nodeName,omitempty"`
	Name         string                       `json:"name"`
	Status       string                       `json:"status"`
	Interfaces   []ProxmoxImportInterfaceDoc `json:"interfaces"`
}

type ProxmoxImportInterfaceDoc struct {
	Name              string `json:"name"`
	Mac               string `json:"mac,omitempty"`
	Bridge            string `json:"bridge,omitempty"`
	VLANTag           string `json:"vlanTag,omitempty"`
	ConfiguredAddress string `json:"configuredAddress,omitempty"`
	ConfiguredNetwork string `json:"configuredNetwork,omitempty"`
}

type ProxmoxImportPreviewSummaryDoc struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Retired   int `json:"retired"`
	Conflicts int `json:"conflicts"`
}

type ProxmoxImportFieldConflictDoc struct {
	Field    string `json:"field"`
	Managed  string `json:"managed"`
	Imported string `json:"imported"`
}

type ProxmoxImportNodeViewDoc struct {
	Hostname    string `json:"hostname"`
	PVEVersion  string `json:"pveVersion"`
	ClusterName string `json:"clusterName"`
	Status      string `json:"status"`
}

type ProxmoxImportNodeDiffDoc struct {
	Action string                    `json:"action"`
	Before *ProxmoxImportNodeViewDoc `json:"before"`
	After  ProxmoxImportNodeViewDoc  `json:"after"`
}

type ProxmoxImportInterfaceViewDoc struct {
	Name              string `json:"name"`
	Mac               string `json:"mac"`
	Bridge            string `json:"bridge"`
	VLANTag           string `json:"vlanTag"`
	ConfiguredAddress string `json:"configuredAddress"`
	ConfiguredNetwork string `json:"configuredNetwork"`
}

type ProxmoxImportWorkloadViewDoc struct {
	ID           uint                            `json:"id,omitempty"`
	NativeID     string                          `json:"nativeId"`
	WorkloadType string                          `json:"workloadType"`
	Name         string                          `json:"name"`
	Status       string                          `json:"status"`
	Source       string                          `json:"source"`
	RetiredAt    string                          `json:"retiredAt,omitempty"`
	Interfaces   []ProxmoxImportInterfaceViewDoc `json:"interfaces"`
}

type ProxmoxImportWorkloadDiffDoc struct {
	Action  string                        `json:"action"`
	Key     string                        `json:"key"`
	Changes []string                      `json:"changes"`
	Before  *ProxmoxImportWorkloadViewDoc `json:"before"`
	After   *ProxmoxImportWorkloadViewDoc `json:"after"`
}

type ProxmoxImportPreviewDoc struct {
	PreviewToken     string                           `json:"previewToken"`
	SnapshotDigest   string                           `json:"snapshotDigest"`
	Source           string                           `json:"source"`
	CollectedAt      string                           `json:"collectedAt"`
	Complete         bool                             `json:"complete"`
	ApplyAllowed     bool                             `json:"applyAllowed"`
	BlockedReasons   []string                         `json:"blockedReasons"`
	Warnings         []string                         `json:"warnings"`
	Summary          ProxmoxImportPreviewSummaryDoc   `json:"summary"`
	Node             ProxmoxImportNodeDiffDoc         `json:"node"`
	ManagedConflicts []ProxmoxImportFieldConflictDoc  `json:"managedConflicts"`
	Workloads        []ProxmoxImportWorkloadDiffDoc   `json:"workloads"`
}

type ProxmoxImportApplyDoc struct {
	PreviewToken string                   `json:"previewToken"`
	Confirmed    bool                     `json:"confirmed"`
	Snapshot     ProxmoxImportSnapshotDoc `json:"snapshot"`
}

type ProxmoxImportApplyResponseDoc struct {
	Applied    bool                           `json:"applied"`
	ImportedAt string                         `json:"importedAt"`
	Summary    ProxmoxImportPreviewSummaryDoc `json:"summary"`
}

type ProxmoxImportApplyRequest struct {
	PreviewToken string                   `json:"previewToken"`
	Confirmed    bool                     `json:"confirmed"`
	Snapshot     proxmoxsnapshot.Snapshot `json:"snapshot"`
}

type ProxmoxImportApplyResponse struct {
	Applied    bool                         `json:"applied"`
	ImportedAt string                       `json:"importedAt"`
	Summary    proxmoximport.PreviewSummary `json:"summary"`
}

// previewProxmoxScriptImport godoc
// @Summary      Preview Proxmox script import
// @Description  Strictly validate a read-only collector snapshot and return a deterministic diff. This endpoint never persists the snapshot.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                    true  "Proxmox Host ID"
// @Param        body  body      ProxmoxImportSnapshotDoc  true  "Collector snapshot"
// @Success      200   {object}  ProxmoxImportPreviewDoc
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/proxmox/import/preview [post]
func previewProxmoxScriptImport(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	var snapshot proxmoxsnapshot.Snapshot
	if !decodeStrictProxmoxImportJSON(c, &snapshot) {
		return
	}

	current, err := loadProxmoxImportCurrentState(host.Mac, models.InfrastructureWorkloadSourceScriptImport)
	if err != nil {
		slog.Error("Failed to load Proxmox import state", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox import state"})
		return
	}

	preview, err := proxmoximport.BuildPreview(host.Mac, snapshot, current)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, preview)
}

// applyProxmoxScriptImport godoc
// @Summary      Apply confirmed Proxmox script import
// @Description  Revalidate a previously previewed snapshot and atomically apply it only when the preview token still matches current persisted state.
// @Tags         hosts
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "Proxmox Host ID"
// @Param        body  body      ProxmoxImportApplyDoc  true  "Confirmed preview token and collector snapshot"
// @Success      200   {object}  ProxmoxImportApplyResponseDoc
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/proxmox/import/apply [post]
func applyProxmoxScriptImport(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	var payload ProxmoxImportApplyRequest
	if !decodeStrictProxmoxImportJSON(c, &payload) {
		return
	}
	payload.PreviewToken = strings.TrimSpace(payload.PreviewToken)
	if !payload.Confirmed {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "explicit confirmation is required"})
		return
	}
	if len(payload.PreviewToken) != 64 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "valid previewToken is required"})
		return
	}

	normalized, err := proxmoximport.ValidateAndNormalize(payload.Snapshot)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, err := loadProxmoxImportCurrentState(host.Mac, models.InfrastructureWorkloadSourceScriptImport)
	if err != nil {
		slog.Error("Failed to load current Proxmox import state", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox import state"})
		return
	}
	preview, err := proxmoximport.BuildPreview(host.Mac, normalized, current)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.PreviewToken != preview.PreviewToken {
		c.IndentedJSON(http.StatusConflict, gin.H{"error": "preview is stale; generate a new preview before applying"})
		return
	}
	if !preview.ApplyAllowed {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "preview cannot be applied until blocking issues are resolved"})
		return
	}

	inputs, err := proxmoximport.SnapshotWorkloadInputs(normalized)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	importedAt := time.Now().UTC().Format(time.RFC3339)
	state, err := proxmoximport.SourceStateFromSnapshot(host.Mac, normalized, preview.SnapshotDigest, importedAt)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := gdb.ApplyProxmoxScriptImport(host.Mac, state, inputs); err != nil {
		if errors.Is(err, gdb.ErrInfrastructureWorkloadSourceConflict) {
			c.IndentedJSON(http.StatusConflict, gin.H{"error": "workload state changed since preview; generate a new preview"})
			return
		}
		slog.Error("Failed to apply Proxmox script import", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to apply Proxmox import"})
		return
	}

	c.IndentedJSON(http.StatusOK, ProxmoxImportApplyResponse{
		Applied:    true,
		ImportedAt: importedAt,
		Summary:    preview.Summary,
	})
}

func proxmoxHypervisorHostFromRequest(c *gin.Context) (models.Host, bool) {
	host, ok := workloadHypervisorHostFromRequest(c)
	if !ok {
		return models.Host{}, false
	}
	profile, found, err := gdb.SelectHypervisorProfileByMAC(host.Mac)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load hypervisor profile"})
		return models.Host{}, false
	}
	if !found || profile.Platform != string(models.HypervisorPlatformProxmoxVE) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "Proxmox import requires a Proxmox VE hypervisor profile"})
		return models.Host{}, false
	}
	return host, true
}

func loadProxmoxImportCurrentState(mac, source string) (proxmoximport.CurrentState, error) {
	var current proxmoximport.CurrentState

	state, found, err := gdb.SelectProxmoxSourceState(mac, source)
	if err != nil {
		return current, err
	}
	if found {
		current.SourceState = &state
	}

	managed, found, err := gdb.SelectHypervisorProfileByMAC(mac)
	if err != nil {
		return current, err
	}
	if found {
		current.ManagedHypervisor = &managed
	}

	workloads, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(mac)
	if err != nil {
		return current, err
	}
	current.Workloads = workloads
	return current, nil
}

func decodeStrictProxmoxImportJSON(c *gin.Context, destination any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProxmoxImportBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid Proxmox import JSON"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid Proxmox import JSON"})
		return false
	}
	return true
}
