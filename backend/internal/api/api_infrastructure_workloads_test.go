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

	typeReq := httptest.NewRequest(http.MethodPatch, "/api/host/"+itoa(server.ID), bytes.NewBufferString(`{"deviceType":"nas"}`))
	typeReq.Header.Set("Content-Type", "application/json")
	typeRec := httptest.NewRecorder()
	router.ServeHTTP(typeRec, typeReq)
	if typeRec.Code != http.StatusBadRequest {
		t.Fatalf("device type change while hypervisor enabled status = %d, want %d; body: %s", typeRec.Code, http.StatusBadRequest, typeRec.Body.String())
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

func TestInfrastructureWorkloadMembershipsExposeReverseParentRelation(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "proxmox", Mac: "AA:BB:CC:DD:F1:10", IP: "10.4.1.6", DeviceType: "server"})
	guest := seedHost(t, models.Host{Name: "actualbudget", Mac: "AA:BB:CC:DD:F1:11", IP: "10.4.1.67", DeviceType: "server"})
	enableTestHypervisor(t, router, hypervisor.ID)

	rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{
		"nativeId":"104",
		"workloadType":"container",
		"name":"actualbudget",
		"status":"running"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create workload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var workload InfrastructureWorkloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal workload: %v", err)
	}
	rec = workloadRequest(router, http.MethodPut, hypervisor.ID, "/"+itoa(int(workload.ID))+"/link", `{"hostId":`+itoa(guest.ID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("link workload status = %d; body: %s", rec.Code, rec.Body.String())
	}

	rec = getPath(router, "/api/infrastructure/workload-memberships")
	if rec.Code != http.StatusOK {
		t.Fatalf("memberships status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var memberships []InfrastructureWorkloadMembershipResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &memberships); err != nil {
		t.Fatalf("json.Unmarshal memberships: %v", err)
	}
	if len(memberships) != 1 {
		t.Fatalf("memberships = %+v", memberships)
	}
	item := memberships[0]
	if item.HostID != guest.ID || item.WorkloadID != workload.ID || item.NativeID != "104" ||
		item.WorkloadType != "container" || item.HypervisorHostID != hypervisor.ID ||
		item.HypervisorName != hypervisor.Name || item.HypervisorIP != hypervisor.IP {
		t.Fatalf("membership = %+v", item)
	}

	rec = getPath(router, "/api/infrastructure/workload-memberships?hostId="+itoa(guest.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("filtered memberships status = %d; body: %s", rec.Code, rec.Body.String())
	}
	memberships = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &memberships); err != nil || len(memberships) != 1 {
		t.Fatalf("filtered memberships=%+v err=%v", memberships, err)
	}

	other := seedHost(t, models.Host{Name: "other", Mac: "AA:BB:CC:DD:F1:12"})
	rec = getPath(router, "/api/infrastructure/workload-memberships?hostId="+itoa(other.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("other filtered memberships status = %d; body: %s", rec.Code, rec.Body.String())
	}
	memberships = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &memberships); err != nil || len(memberships) != 0 {
		t.Fatalf("other memberships=%+v err=%v", memberships, err)
	}
}

func TestInfrastructureWorkloadSummariesCountCurrentInventoryByHypervisor(t *testing.T) {
	router := setupTestRouter(t)
	// Keep the Host MAC lowercase to mirror real discovered Hosts while the
	// workload inventory normalizes hypervisor ownership to canonical uppercase.
	hypervisor := seedHost(t, models.Host{Name: "PROXMOX", Mac: "aa:bb:cc:dd:f2:10", IP: "10.4.1.6", DeviceType: "server"})
	enableTestHypervisor(t, router, hypervisor.ID)

	for _, workload := range []string{
		`{"nativeId":"100","workloadType":"vm","name":"windows","status":"stopped"}`,
		`{"nativeId":"112","workloadType":"vm","name":"haos","status":"running"}`,
		`{"nativeId":"102","workloadType":"container","name":"qbittorrent","status":"running"}`,
		`{"nativeId":"130","workloadType":"container","name":"arr-tools","status":"stopped"}`,
		`{"nativeId":"131","workloadType":"container","name":"zoraxy","status":"running"}`,
	} {
		rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", workload)
		if rec.Code != http.StatusOK {
			t.Fatalf("create workload status = %d; body: %s", rec.Code, rec.Body.String())
		}
	}

	rec := getPath(router, "/api/infrastructure/workload-summaries")
	if rec.Code != http.StatusOK {
		t.Fatalf("workload summaries status = %d; body: %s", rec.Code, rec.Body.String())
	}

	var summaries []InfrastructureWorkloadSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("json.Unmarshal summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summaries = %+v", summaries)
	}
	summary := summaries[0]
	if summary.HypervisorHostID != hypervisor.ID ||
		summary.HypervisorName != hypervisor.Name ||
		summary.HypervisorIP != hypervisor.IP ||
		summary.Platform != "proxmox-ve" ||
		summary.VMCount != 2 ||
		summary.ContainerCount != 3 ||
		summary.TotalCount != 5 {
		t.Fatalf("summary = %+v", summary)
	}
}
