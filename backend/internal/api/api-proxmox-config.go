package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	defaultProxmoxAPITimeoutSeconds = 10
	maxProxmoxAPITimeoutSeconds     = 60
	maxProxmoxAPIConfigBodyBytes    = 64 * 1024
	maxProxmoxAPIURLRunes           = 2048
	maxProxmoxAPITokenIDRunes       = 255
	maxProxmoxAPITokenSecretRunes   = 4096
)

type ProxmoxAPIConfigResponse struct {
	HypervisorMac      string `json:"hypervisorMac"`
	Enabled            bool   `json:"enabled"`
	BaseURL            string `json:"baseUrl"`
	TokenID            string `json:"tokenId"`
	TokenSecretSet     bool   `json:"tokenSecretConfigured"`
	VerifyTLS          bool   `json:"verifyTls"`
	TimeoutSeconds     int    `json:"timeoutSeconds"`
	LastAttemptAt      string `json:"lastAttemptAt,omitempty"`
	LastSuccessfulSync string `json:"lastSuccessfulSync,omitempty"`
	LastError          string `json:"lastError,omitempty"`
	Status             string `json:"status"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

type ProxmoxAPIConfigPatchRequest struct {
	Enabled          *bool   `json:"enabled,omitempty"`
	BaseURL          *string `json:"baseUrl,omitempty"`
	TokenID          *string `json:"tokenId,omitempty"`
	TokenSecret      *string `json:"tokenSecret,omitempty"`
	ClearTokenSecret bool    `json:"clearTokenSecret,omitempty"`
	VerifyTLS        *bool   `json:"verifyTls,omitempty"`
	TimeoutSeconds   *int    `json:"timeoutSeconds,omitempty"`
}

// getHostProxmoxAPIConfig godoc
// @Summary      Get Proxmox API configuration
// @Description  Return the non-secret API connection configuration for one Proxmox Host. The stored token secret is never returned.
// @Tags         proxmox
// @Produce      json
// @Param        id   path      string                    true  "Proxmox Host ID"
// @Success      200  {object}  ProxmoxAPIConfigResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/proxmox/api-config [get]
func getHostProxmoxAPIConfig(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	config, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil {
		slog.Error("Failed to load Proxmox API config", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox API configuration"})
		return
	}
	if !found {
		config = defaultProxmoxAPIConfig(host.Mac)
	}

	c.IndentedJSON(http.StatusOK, publicProxmoxAPIConfig(config))
}

// patchHostProxmoxAPIConfig godoc
// @Summary      Save Proxmox API configuration
// @Description  Update one Proxmox Host API connection. tokenSecret is write-only: omit or send blank to keep it, send a non-empty value to replace it, or clearTokenSecret=true to remove it.
// @Tags         proxmox
// @Accept       json
// @Produce      json
// @Param        id    path      string                        true  "Proxmox Host ID"
// @Param        body  body      ProxmoxAPIConfigPatchRequest true  "API connection settings"
// @Success      200   {object}  ProxmoxAPIConfigResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /host/{id}/proxmox/api-config [patch]
func patchHostProxmoxAPIConfig(c *gin.Context) {
	host, ok := proxmoxHypervisorHostFromRequest(c)
	if !ok {
		return
	}

	var request ProxmoxAPIConfigPatchRequest
	if !decodeStrictProxmoxAPIConfigJSON(c, &request) {
		return
	}

	current, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil {
		slog.Error("Failed to load Proxmox API config before update", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load Proxmox API configuration"})
		return
	}
	if !found {
		current = defaultProxmoxAPIConfig(host.Mac)
	}
	next := current

	if request.Enabled != nil {
		if *request.Enabled && !current.Enabled {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "API integration is enabled only after applying a reviewed sync preview"})
			return
		}
		next.Enabled = *request.Enabled
	}
	if request.BaseURL != nil {
		next.BaseURL = strings.TrimSpace(*request.BaseURL)
	}
	if request.TokenID != nil {
		next.TokenID = strings.TrimSpace(*request.TokenID)
	}
	if request.ClearTokenSecret {
		next.TokenSecret = ""
	} else if request.TokenSecret != nil && *request.TokenSecret != "" {
		next.TokenSecret = *request.TokenSecret
	}
	if request.VerifyTLS != nil {
		next.VerifyTLS = *request.VerifyTLS
	}
	if request.TimeoutSeconds != nil {
		next.TimeoutSeconds = *request.TimeoutSeconds
	}

	if err := normalizeAndValidateProxmoxAPIConfig(&next); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	next.HypervisorMac = host.Mac
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if next.Enabled {
		next.Status = "configured"
	} else {
		next.Status = "disabled"
	}
	next.LastError = ""

	if err := gdb.UpsertProxmoxAPIConfig(next); err != nil {
		slog.Error("Failed to persist Proxmox API config", "hostID", host.ID, "mac", host.Mac, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to persist Proxmox API configuration"})
		return
	}

	c.IndentedJSON(http.StatusOK, publicProxmoxAPIConfig(next))
}

func defaultProxmoxAPIConfig(hypervisorMac string) models.ProxmoxAPIConfig {
	return models.ProxmoxAPIConfig{
		HypervisorMac:  hypervisorMac,
		VerifyTLS:      true,
		TimeoutSeconds: defaultProxmoxAPITimeoutSeconds,
		Status:         "not-configured",
	}
}

func publicProxmoxAPIConfig(config models.ProxmoxAPIConfig) ProxmoxAPIConfigResponse {
	status := strings.TrimSpace(config.Status)
	if status == "" {
		status = "not-configured"
	}
	return ProxmoxAPIConfigResponse{
		HypervisorMac:      config.HypervisorMac,
		Enabled:            config.Enabled,
		BaseURL:            config.BaseURL,
		TokenID:            config.TokenID,
		TokenSecretSet:     config.TokenSecret != "",
		VerifyTLS:          config.VerifyTLS,
		TimeoutSeconds:     config.TimeoutSeconds,
		LastAttemptAt:      config.LastAttemptAt,
		LastSuccessfulSync: config.LastSuccessfulSync,
		LastError:          config.LastError,
		Status:             status,
		UpdatedAt:          config.UpdatedAt,
	}
}

func normalizeAndValidateProxmoxAPIConfig(config *models.ProxmoxAPIConfig) error {
	if config == nil {
		return errors.New("invalid Proxmox API configuration")
	}

	config.BaseURL = strings.TrimSpace(config.BaseURL)
	config.TokenID = strings.TrimSpace(config.TokenID)
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = defaultProxmoxAPITimeoutSeconds
	}
	if config.TimeoutSeconds < 1 || config.TimeoutSeconds > maxProxmoxAPITimeoutSeconds {
		return errors.New("timeoutSeconds must be between 1 and 60")
	}
	if utf8.RuneCountInString(config.BaseURL) > maxProxmoxAPIURLRunes {
		return errors.New("baseUrl is too long")
	}
	if utf8.RuneCountInString(config.TokenID) > maxProxmoxAPITokenIDRunes || hasUnsafeConfigControl(config.TokenID) {
		return errors.New("invalid tokenId")
	}
	if utf8.RuneCountInString(config.TokenSecret) > maxProxmoxAPITokenSecretRunes || hasUnsafeConfigControl(config.TokenSecret) {
		return errors.New("invalid tokenSecret")
	}

	if config.BaseURL != "" {
		parsed, err := url.Parse(config.BaseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return errors.New("invalid baseUrl")
		}
		if parsed.Scheme != "https" {
			return errors.New("baseUrl must use https")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("baseUrl must not contain credentials, query, or fragment")
		}
		if path := strings.Trim(parsed.EscapedPath(), "/"); path != "" {
			return errors.New("baseUrl must not contain an API path")
		}
		config.BaseURL = strings.TrimRight(config.BaseURL, "/")
	}

	if config.Enabled {
		if config.BaseURL == "" {
			return errors.New("baseUrl is required when Proxmox API is enabled")
		}
		if config.TokenID == "" {
			return errors.New("tokenId is required when Proxmox API is enabled")
		}
		if config.TokenSecret == "" {
			return errors.New("tokenSecret is required when Proxmox API is enabled")
		}
	}

	return nil
}

func hasUnsafeConfigControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func decodeStrictProxmoxAPIConfigJSON(c *gin.Context, target any) bool {
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxProxmoxAPIConfigBodyBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid Proxmox API configuration"})
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "invalid Proxmox API configuration"})
		return false
	}
	return true
}
