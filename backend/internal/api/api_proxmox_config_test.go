package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestProxmoxAPIConfigDefaultsTLSVerificationOn(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-default", Mac: "AA:BB:CC:DD:EE:A1", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	rec := getPath(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api-config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got ProxmoxAPIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.VerifyTLS != true || got.TimeoutSeconds != defaultProxmoxAPITimeoutSeconds || got.TokenSecretSet {
		t.Fatalf("default Proxmox API config = %+v", got)
	}
}

func TestProxmoxAPIConfigSecretIsWriteOnlyKeepReplaceClear(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-secret", Mac: "AA:BB:CC:DD:EE:A2", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	rec := patchProxmoxAPIConfig(t, router, host.ID, `{
		"enabled": false,
		"baseUrl": "https://10.4.1.6:8006/",
		"tokenId": "lannventory@pve!inventory",
		"tokenSecret": "first-proxmox-secret",
		"verifyTls": true,
		"timeoutSeconds": 12
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("initial save status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "first-proxmox-secret") {
		t.Fatalf("save response leaked token secret: %s", rec.Body.String())
	}

	stored, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found || stored.TokenSecret != "first-proxmox-secret" {
		t.Fatalf("stored config found=%v err=%v config=%+v", found, err, stored)
	}

	rec = patchProxmoxAPIConfig(t, router, host.ID, `{"tokenSecret":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("keep save status = %d; body: %s", rec.Code, rec.Body.String())
	}
	stored, _, _ = gdb.SelectProxmoxAPIConfig(host.Mac)
	if stored.TokenSecret != "first-proxmox-secret" {
		t.Fatalf("blank tokenSecret did not keep stored secret")
	}

	rec = patchProxmoxAPIConfig(t, router, host.ID, `{"tokenSecret":"replacement-proxmox-secret"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace save status = %d; body: %s", rec.Code, rec.Body.String())
	}
	stored, _, _ = gdb.SelectProxmoxAPIConfig(host.Mac)
	if stored.TokenSecret != "replacement-proxmox-secret" {
		t.Fatalf("replacement token secret = %q", stored.TokenSecret)
	}

	rec = patchProxmoxAPIConfig(t, router, host.ID, `{"clearTokenSecret":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear save status = %d; body: %s", rec.Code, rec.Body.String())
	}
	stored, _, _ = gdb.SelectProxmoxAPIConfig(host.Mac)
	if stored.TokenSecret != "" {
		t.Fatalf("clear left token secret configured")
	}

	rec = getPath(router, "/api/host/"+itoaHostID(host.ID)+"/proxmox/api-config")
	if strings.Contains(rec.Body.String(), "replacement-proxmox-secret") || strings.Contains(rec.Body.String(), "first-proxmox-secret") {
		t.Fatalf("GET leaked token secret: %s", rec.Body.String())
	}
}

func TestProxmoxAPIConfigEnabledRequiresSecureCompleteCredentials(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-validation", Mac: "AA:BB:CC:DD:EE:A3", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	for name, body := range map[string]string{
		"missing credentials": `{"enabled":true}`,
		"http URL": `{"enabled":false,"baseUrl":"http://10.4.1.6:8006"}`,
		"path URL": `{"enabled":false,"baseUrl":"https://10.4.1.6:8006/api2/json"}`,
		"invalid timeout": `{"enabled":false,"timeoutSeconds":61}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := patchProxmoxAPIConfig(t, router, host.ID, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}

	rec := patchProxmoxAPIConfig(t, router, host.ID, `{
		"enabled": true,
		"baseUrl": "https://10.4.1.6:8006",
		"tokenId": "lannventory@pve!inventory",
		"tokenSecret": "valid-proxmox-secret",
		"verifyTls": false,
		"timeoutSeconds": 15
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unreviewed enable status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = patchProxmoxAPIConfig(t, router, host.ID, `{
		"baseUrl": "https://10.4.1.6:8006",
		"tokenId": "lannventory@pve!inventory",
		"tokenSecret": "valid-proxmox-secret",
		"verifyTls": false,
		"timeoutSeconds": 15
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete disabled config status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var got ProxmoxAPIConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.Enabled || got.VerifyTLS || !got.TokenSecretSet {
		t.Fatalf("saved pre-sync config = %+v", got)
	}
}

func TestProxmoxAPIConfigClearingSecretDisablesEnabledSource(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-clear-enabled", Mac: "AA:BB:CC:DD:EE:A5", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	if err := gdb.UpsertProxmoxAPIConfig(models.ProxmoxAPIConfig{
		HypervisorMac: host.Mac,
		Enabled:       true,
		BaseURL:       "https://10.4.1.6:8006",
		TokenID:       "lannventory@pve!inventory",
		TokenSecret:   "enabled-secret",
		VerifyTLS:     true,
		TimeoutSeconds: 10,
		Status:        "connected",
	}); err != nil {
		t.Fatalf("UpsertProxmoxAPIConfig: %v", err)
	}

	rec := patchProxmoxAPIConfig(t, router, host.ID, `{"clearTokenSecret":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear enabled secret status = %d; body: %s", rec.Code, rec.Body.String())
	}
	stored, found, err := gdb.SelectProxmoxAPIConfig(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectProxmoxAPIConfig found=%v err=%v", found, err)
	}
	if stored.Enabled || stored.TokenSecret != "" {
		t.Fatalf("cleared config must be disabled with no secret: %+v", stored)
	}
}

func TestProxmoxAPIConfigSecretIsExcludedFromBackup(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "pve-api-backup", Mac: "AA:BB:CC:DD:EE:A4", DeviceType: "server"})
	enableTestHypervisor(t, router, host.ID)

	rec := patchProxmoxAPIConfig(t, router, host.ID, `{
		"enabled": false,
		"baseUrl": "https://10.4.1.6:8006",
		"tokenId": "lannventory@pve!inventory",
		"tokenSecret": "backup-must-not-contain-this-secret"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d; body: %s", rec.Code, rec.Body.String())
	}

	rec = getPath(router, "/api/export/backup")
	if rec.Code != http.StatusOK {
		t.Fatalf("backup status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "backup-must-not-contain-this-secret") {
		t.Fatalf("backup leaked Proxmox API token secret")
	}
}

func patchProxmoxAPIConfig(t *testing.T, router *gin.Engine, hostID int, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/host/"+itoaHostID(hostID)+"/proxmox/api-config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func itoaHostID(id int) string {
	const digits = "0123456789"
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = digits[id%10]
		id /= 10
	}
	return string(buf[i:])
}
