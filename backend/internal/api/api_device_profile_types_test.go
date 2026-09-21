package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/models"
)

func TestTypedDeviceProfileAPIUsesOneAggregateReadModel(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "proxmox", Mac: "AA:BB:CC:DD:EE:45", DeviceType: "server"})

	assertProfilePatchOK(t, router, host.ID, "/network", `{"managementMode":"managed","physicalPortCount":4,"portCapabilityNotes":"2x 2.5GbE"}`)
	assertProfilePatchOK(t, router, host.ID, "/system", `{"role":"Virtualization host","operatingSystem":"Debian GNU/Linux","version":"13"}`)
	assertProfilePatchOK(t, router, host.ID, "/hypervisor", `{"platform":"proxmox-ve","version":"9.2","nodeName":"proxmox","clusterName":"home"}`)

	rec := getPath(router, "/api/host/"+itoa(host.ID)+"/profile")
	if rec.Code != http.StatusOK {
		t.Fatalf("profile status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response DeviceProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal profile: %v", err)
	}
	if response.Network == nil || response.Network.ManagementMode != "managed" || response.Network.PhysicalPortCount != 4 {
		t.Fatalf("network profile = %+v", response.Network)
	}
	if response.System == nil || response.System.Role != "Virtualization host" || response.System.OperatingSystem != "Debian GNU/Linux" {
		t.Fatalf("system profile = %+v", response.System)
	}
	if response.Hypervisor == nil || response.Hypervisor.Platform != "proxmox-ve" || response.Hypervisor.NodeName != "proxmox" {
		t.Fatalf("hypervisor profile = %+v", response.Hypervisor)
	}
	if response.Managed != nil {
		t.Fatalf("generic managed profile unexpectedly created: %+v", response.Managed)
	}
}

func TestTypedDeviceProfileAPIValidatesSpecializedFields(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "switch", Mac: "AA:BB:CC:DD:EE:46", DeviceType: "switch"})

	tests := []struct {
		path string
		body string
	}{
		{"/network", `{"managementMode":"sometimes"}`},
		{"/network", `{"physicalPortCount":-1}`},
		{"/network", `{"physicalPortCount":65536}`},
		{"/system", `{"role":"bad\u0001role"}`},
		{"/hypervisor", `{"platform":"not-a-hypervisor"}`},
		{"/hypervisor", `{"nodeName":"node-without-platform"}`},
		{"/system", `{"unexpected":"field"}`},
	}

	for _, tt := range tests {
		rec := patchProfilePath(router, host.ID, tt.path, tt.body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s body %s status = %d, want %d; response: %s", tt.path, tt.body, rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	}
}

func TestHypervisorProfileCanBeRemovedWithoutChangingHostOrSystemProfile(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "hypervisor", Mac: "AA:BB:CC:DD:EE:47", DeviceType: "server"})
	assertProfilePatchOK(t, router, host.ID, "/system", `{"role":"Compute","operatingSystem":"Debian"}`)
	assertProfilePatchOK(t, router, host.ID, "/hypervisor", `{"platform":"proxmox-ve","nodeName":"pve-1"}`)

	req := httptest.NewRequest(http.MethodDelete, "/api/host/"+itoa(host.ID)+"/profile/hypervisor", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete hypervisor status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response DeviceProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal response: %v", err)
	}
	if response.Hypervisor != nil {
		t.Fatalf("hypervisor profile after delete = %+v, want nil", response.Hypervisor)
	}
	if response.System == nil || response.System.Role != "Compute" || response.System.OperatingSystem != "Debian" {
		t.Fatalf("system profile changed by hypervisor removal: %+v", response.System)
	}

	current := getPath(router, "/api/host/"+itoa(host.ID))
	if current.Code != http.StatusOK {
		t.Fatalf("host status = %d; body: %s", current.Code, current.Body.String())
	}
	var currentHost models.Host
	if err := json.Unmarshal(current.Body.Bytes(), &currentHost); err != nil {
		t.Fatalf("json.Unmarshal host: %v", err)
	}
	if currentHost.DeviceType != "server" {
		t.Fatalf("DeviceType = %q, want server", currentHost.DeviceType)
	}
}

func assertProfilePatchOK(t *testing.T, router *gin.Engine, id int, suffix, body string) {
	t.Helper()
	rec := patchProfilePath(router, id, suffix, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %s status = %d, want %d; body: %s", suffix, rec.Code, http.StatusOK, rec.Body.String())
	}
}

func patchProfilePath(router *gin.Engine, id int, suffix, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/host/"+itoa(id)+"/profile"+suffix, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
