package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/workloadmatch"
)

func TestWorkloadMatchesEndpointReportsExactAutoLinkAfterImport(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EF:10", DeviceType: "server", Now: 1})
	guest := seedHost(t, models.Host{Name: "media", Mac: "AA:BB:CC:DD:EF:11", IP: "10.4.1.27", DeviceType: "server", Now: 1})
	enableTestHypervisor(t, router, hypervisor.ID)

	snapshot := apiValidProxmoxSnapshot()
	snapshot.Workloads[0].Name = "media"
	snapshot.Workloads[0].Interfaces[0].Mac = guest.Mac
	snapshot.Workloads[0].Interfaces[0].ConfiguredAddress = "10.4.1.27/24"
	snapshot.Workloads[0].Interfaces[0].ConfiguredNetwork = "10.4.1.0/24"

	previewRec := proxmoxPreviewRequest(t, router, hypervisor.ID, snapshot)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("preview status = %d; body: %s", previewRec.Code, previewRec.Body.String())
	}
	var preview proxmoximport.Preview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}

	applyRec := proxmoxApplyRequest(t, router, hypervisor.ID, ProxmoxImportApplyRequest{
		PreviewToken: preview.PreviewToken,
		Confirmed:    true,
		Snapshot:     snapshot,
	})
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d; body: %s", applyRec.Code, applyRec.Body.String())
	}

	rec := getPath(router, "/api/host/"+itoa(hypervisor.ID)+"/workload-matches")
	if rec.Code != http.StatusOK {
		t.Fatalf("matches status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var matches []InfrastructureWorkloadMatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &matches); err != nil {
		t.Fatalf("json.Unmarshal matches: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %+v", matches)
	}
	match := matches[0]
	if match.CurrentLink == nil ||
		match.CurrentLink.HostID != guest.ID ||
		match.CurrentLink.LinkSource != models.InfrastructureWorkloadLinkSourceExactMAC {
		t.Fatalf("current exact link = %+v", match.CurrentLink)
	}
	if match.DeterministicExactHostID != guest.ID || match.ExactAmbiguous {
		t.Fatalf("exact match result = %+v", match)
	}
	if len(match.Candidates) != 1 ||
		match.Candidates[0].HostID != guest.ID ||
		match.Candidates[0].Strength != workloadmatch.StrengthExactMAC {
		t.Fatalf("candidates = %+v", match.Candidates)
	}
}

func TestWorkloadMatchesEndpointKeepsAddressAndNameAsSuggestions(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EF:20", DeviceType: "server", Now: 1})
	guest := seedHost(t, models.Host{Name: "guest-vm", DNS: "guest-vm.local", Mac: "AA:BB:CC:DD:EF:21", IP: "10.4.1.61", DeviceType: "server", Now: 1})
	enableTestHypervisor(t, router, hypervisor.ID)

	rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{
		"nativeId":"301",
		"workloadType":"vm",
		"name":"guest-vm",
		"status":"running",
		"interfaces":[{"name":"net0","mac":"AA:BB:CC:DD:EF:2F","configuredAddress":"10.4.1.61/24"}]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create manual workload status = %d; body: %s", rec.Code, rec.Body.String())
	}

	rec = getPath(router, "/api/host/"+itoa(hypervisor.ID)+"/workload-matches")
	if rec.Code != http.StatusOK {
		t.Fatalf("matches status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var matches []InfrastructureWorkloadMatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &matches); err != nil {
		t.Fatalf("json.Unmarshal matches: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %+v", matches)
	}
	match := matches[0]
	if match.CurrentLink != nil || match.DeterministicExactHostID != 0 || match.ExactAmbiguous {
		t.Fatalf("weak evidence produced automatic link: %+v", match)
	}
	if len(match.Candidates) != 1 || match.Candidates[0].HostID != guest.ID {
		t.Fatalf("suggestions = %+v", match.Candidates)
	}
	if match.Candidates[0].Strength != workloadmatch.StrengthAddress {
		t.Fatalf("candidate strength = %q, want address; candidate=%+v", match.Candidates[0].Strength, match.Candidates[0])
	}
	foundName := false
	for _, evidence := range match.Candidates[0].Evidence {
		if evidence.Strength == workloadmatch.StrengthName {
			foundName = true
		}
	}
	if !foundName {
		t.Fatalf("address candidate did not retain weak name evidence: %+v", match.Candidates[0].Evidence)
	}
}

func TestWorkloadMatchesEndpointReportsAmbiguousExactMACWithoutAutoLink(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:EF:30", DeviceType: "server", Now: 1})
	guestA := seedHost(t, models.Host{Name: "guest-a", Mac: "AA:BB:CC:DD:EF:31", DeviceType: "server", Now: 1})
	enableTestHypervisor(t, router, hypervisor.ID)

	guestB := models.Host{ID: guestA.ID + 100, Name: "guest-b", Mac: guestA.Mac, DeviceType: "server", Now: 1}
	if err := gdb.UpdateWithError("now", guestB); err != nil {
		t.Fatalf("seed duplicate-MAC host: %v", err)
	}

	snapshot := apiValidProxmoxSnapshot()
	snapshot.Workloads[0].Interfaces[0].Mac = guestA.Mac
	snapshot.Workloads[0].Interfaces[0].ConfiguredAddress = ""
	snapshot.Workloads[0].Interfaces[0].ConfiguredNetwork = ""

	previewRec := proxmoxPreviewRequest(t, router, hypervisor.ID, snapshot)
	var preview proxmoximport.Preview
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal preview: %v", err)
	}
	applyRec := proxmoxApplyRequest(t, router, hypervisor.ID, ProxmoxImportApplyRequest{
		PreviewToken: preview.PreviewToken,
		Confirmed:    true,
		Snapshot:     snapshot,
	})
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d; body: %s", applyRec.Code, applyRec.Body.String())
	}

	rec := getPath(router, "/api/host/"+itoa(hypervisor.ID)+"/workload-matches")
	if rec.Code != http.StatusOK {
		t.Fatalf("matches status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var matches []InfrastructureWorkloadMatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &matches); err != nil {
		t.Fatalf("json.Unmarshal matches: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %+v", matches)
	}
	if !matches[0].ExactAmbiguous ||
		matches[0].DeterministicExactHostID != 0 ||
		matches[0].CurrentLink != nil ||
		len(matches[0].Candidates) != 2 {
		t.Fatalf("ambiguous exact result = %+v", matches[0])
	}
}
