package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestDeviceProfileAPISeparatesManagedProfileAndSupportsPartialUpdates(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "server",
		Mac:        "AA:BB:CC:DD:EE:35",
		IP:         "192.168.1.35",
		DeviceType: "server",
	})

	rec := getPath(router, "/api/host/"+itoa(host.ID)+"/profile")
	if rec.Code != http.StatusOK {
		t.Fatalf("initial profile status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var initial DeviceProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("json.Unmarshal initial profile: %v", err)
	}
	if initial.Managed != nil {
		t.Fatalf("initial managed profile = %+v, want nil", initial.Managed)
	}

	rec = patchDeviceProfile(router, host.ID, `{"manufacturer":"  Fujitsu  ","model":"CELSIUS W550P","managementAddress":"  proxmox.home  "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile patch status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var updated DeviceProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal updated profile: %v", err)
	}
	if updated.Managed == nil {
		t.Fatal("updated managed profile is nil")
	}
	if updated.Managed.Manufacturer != "Fujitsu" || updated.Managed.Model != "CELSIUS W550P" || updated.Managed.ManagementAddress != "proxmox.home" {
		t.Fatalf("updated managed profile = %+v", updated.Managed)
	}

	rec = patchDeviceProfile(router, host.ID, `{"model":"CELSIUS W550P Gen2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("partial patch status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal partial profile: %v", err)
	}
	if updated.Managed == nil || updated.Managed.Manufacturer != "Fujitsu" || updated.Managed.Model != "CELSIUS W550P Gen2" || updated.Managed.ManagementAddress != "proxmox.home" {
		t.Fatalf("partial patch lost unrelated values: %+v", updated.Managed)
	}

	rec = patchDeviceProfile(router, host.ID, `{"manufacturer":"","model":"","managementAddress":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear profile status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal cleared profile: %v", err)
	}
	if updated.Managed != nil {
		t.Fatalf("cleared managed profile = %+v, want nil", updated.Managed)
	}
}

func TestDeviceProfileAPIValidatesInputStrictly(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:36"})

	tests := []struct {
		name string
		body string
	}{
		{name: "manufacturer too long", body: profileJSON(t, map[string]any{"manufacturer": strings.Repeat("a", 256)})},
		{name: "model control character", body: profileJSON(t, map[string]any{"model": "bad\u0001model"})},
		{name: "management address too long", body: profileJSON(t, map[string]any{"managementAddress": strings.Repeat("b", 256)})},
		{name: "unknown field", body: `{"manufacturer":"Fujitsu","secret":"unexpected"}`},
		{name: "malformed json", body: `not-json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := patchDeviceProfile(router, host.ID, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHostDeleteRemovesManagedDeviceProfile(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "nas",
		Mac:        "AA:BB:CC:DD:EE:37",
		DeviceType: "nas",
	})

	manufacturer := "TrueNAS"
	if _, found, err := gdb.UpdateDeviceProfile(host.Mac, models.DeviceProfileUpdate{Manufacturer: &manufacturer}); err != nil || !found {
		t.Fatalf("UpdateDeviceProfile found=%v err=%v", found, err)
	}
	role := "Storage"
	if _, found, err := gdb.UpdateSystemDeviceProfile(host.Mac, models.SystemDeviceProfileUpdate{Role: &role}); err != nil || !found {
		t.Fatalf("UpdateSystemDeviceProfile found=%v err=%v", found, err)
	}
	platform := "other"
	if _, found, err := gdb.UpdateHypervisorProfile(host.Mac, models.HypervisorProfileUpdate{Platform: &platform}); err != nil || !found {
		t.Fatalf("UpdateHypervisorProfile found=%v err=%v", found, err)
	}

	rec := getPath(router, "/api/host/del/"+itoa(host.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete host status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if _, found, err := gdb.SelectDeviceProfileByMAC(host.Mac); err != nil || found {
		t.Fatalf("profile after host delete found=%v err=%v, want absent", found, err)
	}
	if _, found, err := gdb.SelectSystemDeviceProfileByMAC(host.Mac); err != nil || found {
		t.Fatalf("system profile after host delete found=%v err=%v, want absent", found, err)
	}
	if _, found, err := gdb.SelectHypervisorProfileByMAC(host.Mac); err != nil || found {
		t.Fatalf("hypervisor profile after host delete found=%v err=%v, want absent", found, err)
	}
}

func patchDeviceProfile(router *gin.Engine, id int, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/host/"+itoa(id)+"/profile", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func profileJSON(t *testing.T, value map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal profile payload: %v", err)
	}
	return string(payload)
}
