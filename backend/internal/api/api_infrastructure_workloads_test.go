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

func TestInfrastructureWorkloadAPICreatesInventoryWithoutFakeHost(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:60", DeviceType: "server"})
	enableTestHypervisor(t, router, hypervisor.ID)

	rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{"nativeId":"119","workloadType":"vm","name":"ubuntu-plex-immich","status":"running","interfaces":[{"name":"net0","mac":"bc:24:11:a2:40:12","bridge":"vmbr0","configuredAddress":"10.4.1.27"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create workload status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var workload InfrastructureWorkloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal workload: %v", err)
	}
	if workload.NativeID != "119" || workload.WorkloadType != "vm" || workload.Source != "manual" {
		t.Fatalf("workload = %+v", workload.InfrastructureWorkload)
	}
	if len(workload.Interfaces) != 1 || workload.Interfaces[0].Mac != "BC:24:11:A2:40:12" {
		t.Fatalf("interfaces = %+v", workload.Interfaces)
	}

	all := getPath(router, "/api/all")
	if all.Code != http.StatusOK {
		t.Fatalf("all hosts status = %d; body: %s", all.Code, all.Body.String())
	}
	var hosts []models.Host
	if err := json.Unmarshal(all.Body.Bytes(), &hosts); err != nil {
		t.Fatalf("json.Unmarshal hosts: %v", err)
	}
	if len(hosts) != 1 || hosts[0].ID != hypervisor.ID {
		t.Fatalf("hosts = %+v, workload must not create fake Host", hosts)
	}
}

func TestInfrastructureWorkloadAPILinksAndUnlinksExistingHost(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:61", DeviceType: "server"})
	guest := seedHost(t, models.Host{Name: "guest", Mac: "AA:BB:CC:DD:EE:62", IP: "10.4.1.62", DeviceType: "server"})
	enableTestHypervisor(t, router, hypervisor.ID)

	rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{"nativeId":"200","workloadType":"vm","name":"guest","status":"running"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create workload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var workload InfrastructureWorkloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal workload: %v", err)
	}

	rec = workloadRequest(router, http.MethodPut, hypervisor.ID, "/"+itoa(int(workload.ID))+"/link", `{"hostId":`+itoa(guest.ID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("link status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal linked workload: %v", err)
	}
	if workload.Link == nil || workload.Link.HostID != guest.ID || workload.Link.LinkSource != "manual" {
		t.Fatalf("link = %+v", workload.Link)
	}
	if workload.MatchedHost == nil || workload.MatchedHost.HostID != guest.ID || workload.MatchedHost.Mac != guest.Mac {
		t.Fatalf("matched host = %+v", workload.MatchedHost)
	}

	rec = workloadRequest(router, http.MethodDelete, hypervisor.ID, "/"+itoa(int(workload.ID))+"/link", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("unlink status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal unlinked workload: %v", err)
	}
	if workload.Link != nil || workload.MatchedHost != nil {
		t.Fatalf("workload remained linked: link=%+v host=%+v", workload.Link, workload.MatchedHost)
	}
}

func TestInfrastructureWorkloadAPIRejectsInvalidOwnershipAndHypervisorState(t *testing.T) {
	router := setupTestRouter(t)
	server := seedHost(t, models.Host{Name: "plain-server", Mac: "AA:BB:CC:DD:EE:63", DeviceType: "server"})
	desktop := seedHost(t, models.Host{Name: "desktop", Mac: "AA:BB:CC:DD:EE:64", DeviceType: "desktop"})

	rec := workloadRequest(router, http.MethodPost, server.ID, "", `{"nativeId":"1","workloadType":"vm"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-hypervisor create status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = patchProfilePath(router, desktop.ID, "/hypervisor", `{"platform":"proxmox-ve"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("desktop hypervisor status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	enableTestHypervisor(t, router, server.ID)

	rec = patchProfilePath(router, server.ID, "", `{"deviceType":"nas"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("device type change while hypervisor enabled status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = workloadRequest(router, http.MethodPost, server.ID, "", `{"nativeId":"","workloadType":"vm"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty native id status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	rec = workloadRequest(router, http.MethodPost, server.ID, "", `{"nativeId":"2","workloadType":"fake"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid workload type status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	rec = workloadRequest(router, http.MethodPost, server.ID, "", `{"nativeId":"3","workloadType":"vm","interfaces":[{"name":"net0"},{"name":"net0"}]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate interface status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestInfrastructureWorkloadAPIManualPatchAndDelete(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EE:65", DeviceType: "server"})
	enableTestHypervisor(t, router, hypervisor.ID)

	rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{"nativeId":"300","workloadType":"container","name":"old","status":"stopped","interfaces":[{"name":"net0","mac":"AA:BB:CC:00:00:30"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create workload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var workload InfrastructureWorkloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal workload: %v", err)
	}

	rec = workloadRequest(router, http.MethodPatch, hypervisor.ID, "/"+itoa(int(workload.ID)), `{"name":"new","status":"running","interfaces":[]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch workload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal patched workload: %v", err)
	}
	if workload.Name != "new" || workload.Status != "running" || len(workload.Interfaces) != 0 {
		t.Fatalf("patched workload = %+v interfaces=%+v", workload.InfrastructureWorkload, workload.Interfaces)
	}

	rec = workloadRequest(router, http.MethodDelete, hypervisor.ID, "/"+itoa(int(workload.ID)), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete workload status = %d; body: %s", rec.Code, rec.Body.String())
	}

	list := getPath(router, "/api/host/"+itoa(hypervisor.ID)+"/workloads")
	if list.Code != http.StatusOK {
		t.Fatalf("list workloads status = %d; body: %s", list.Code, list.Body.String())
	}
	var workloads []InfrastructureWorkloadResponse
	if err := json.Unmarshal(list.Body.Bytes(), &workloads); err != nil {
		t.Fatalf("json.Unmarshal workloads: %v", err)
	}
	if len(workloads) != 0 {
		t.Fatalf("workloads after delete = %+v, want empty", workloads)
	}

	rec = patchProfilePath(router, hypervisor.ID, "/hypervisor", `{"platform":"proxmox-ve"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-enable hypervisor status = %d; body: %s", rec.Code, rec.Body.String())
	}
	rec = workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{"nativeId":"301","workloadType":"vm"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create workload before profile removal guard status = %d; body: %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/host/"+itoa(hypervisor.ID)+"/profile/hypervisor", nil)
	blocked := httptest.NewRecorder()
	router.ServeHTTP(blocked, req)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("remove hypervisor with workloads status = %d, want %d; body: %s", blocked.Code, http.StatusBadRequest, blocked.Body.String())
	}
}

func enableTestHypervisor(t *testing.T, router *gin.Engine, hostID int) {
	t.Helper()
	rec := patchProfilePath(router, hostID, "/hypervisor", `{"platform":"proxmox-ve"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable hypervisor status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func workloadRequest(router *gin.Engine, method string, hostID int, suffix, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/host/"+itoa(hostID)+"/workloads"+suffix, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
