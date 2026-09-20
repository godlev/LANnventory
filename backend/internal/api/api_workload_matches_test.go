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


func TestWorkloadMatchRejectionHidesOnlyUnchangedWeakEvidence(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:F0:10", DeviceType: "server", Now: 1})
	guest := seedHost(t, models.Host{Name: "IR + RF - Tuya", Mac: "FC:67:1F:26:20:A5", IP: "10.4.1.67", DeviceType: "iot", Now: 1})
	enableTestHypervisor(t, router, hypervisor.ID)

	rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{
		"nativeId":"104",
		"workloadType":"container",
		"name":"actualbudget",
		"status":"running",
		"interfaces":[{"name":"net0","mac":"BC:24:11:14:02:A5","configuredAddress":"10.4.1.67/24"}]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create workload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var workload InfrastructureWorkloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal workload: %v", err)
	}

	loadMatches := func() []InfrastructureWorkloadMatchResponse {
		t.Helper()
		matchesRec := getPath(router, "/api/host/"+itoa(hypervisor.ID)+"/workload-matches")
		if matchesRec.Code != http.StatusOK {
			t.Fatalf("matches status = %d; body: %s", matchesRec.Code, matchesRec.Body.String())
		}
		var matches []InfrastructureWorkloadMatchResponse
		if err := json.Unmarshal(matchesRec.Body.Bytes(), &matches); err != nil {
			t.Fatalf("json.Unmarshal matches: %v", err)
		}
		return matches
	}

	matches := loadMatches()
	if len(matches) != 1 || len(matches[0].Candidates) != 1 {
		t.Fatalf("initial matches = %+v", matches)
	}
	candidate := matches[0].Candidates[0]
	if candidate.HostID != guest.ID || !candidate.PossibleIPConflict || candidate.Assessment != "possible-ip-conflict" || candidate.Rejected {
		t.Fatalf("initial candidate = %+v", candidate)
	}
	if candidate.EvidenceFingerprint == "" {
		t.Fatal("candidate evidence fingerprint is empty")
	}

	rec = workloadRequest(
		router,
		http.MethodPut,
		hypervisor.ID,
		"/"+itoa(int(workload.ID))+"/match-rejections/"+itoa(guest.ID),
		`{"evidenceFingerprint":"`+candidate.EvidenceFingerprint+`"}`,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject candidate status = %d; body: %s", rec.Code, rec.Body.String())
	}
	matches = loadMatches()
	if len(matches[0].Candidates) != 1 || !matches[0].Candidates[0].Rejected {
		t.Fatalf("unchanged rejected candidate was not marked rejected: %+v", matches[0].Candidates)
	}
	originalFingerprint := matches[0].Candidates[0].EvidenceFingerprint

	rec = workloadRequest(
		router,
		http.MethodPatch,
		hypervisor.ID,
		"/"+itoa(int(workload.ID)),
		`{"interfaces":[{"name":"net0","mac":"BC:24:11:14:02:A5","configuredAddress":"10.4.1.68/24"}]}`,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch workload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	guest.IP = "10.4.1.68"
	if err := gdb.UpdateWithError("now", guest); err != nil {
		t.Fatalf("move candidate Host address: %v", err)
	}

	matches = loadMatches()
	if len(matches[0].Candidates) != 1 {
		t.Fatalf("changed evidence candidates = %+v", matches[0].Candidates)
	}
	changed := matches[0].Candidates[0]
	if changed.EvidenceFingerprint == originalFingerprint {
		t.Fatalf("material address change did not change evidence fingerprint: %+v", changed)
	}
	if changed.Rejected {
		t.Fatalf("stale rejection suppressed changed evidence: %+v", changed)
	}

	rec = workloadRequest(
		router,
		http.MethodPatch,
		hypervisor.ID,
		"/"+itoa(int(workload.ID)),
		`{"interfaces":[{"name":"net0","mac":"FC:67:1F:26:20:A5","configuredAddress":"10.4.1.68/24"}]}`,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch exact workload MAC status = %d; body: %s", rec.Code, rec.Body.String())
	}
	matches = loadMatches()
	if matches[0].DeterministicExactHostID != guest.ID || len(matches[0].Candidates) != 1 ||
		matches[0].Candidates[0].Strength != workloadmatch.StrengthExactMAC || matches[0].Candidates[0].Rejected {
		t.Fatalf("stronger exact evidence did not supersede rejection: %+v", matches[0])
	}

	rec = workloadRequest(router, http.MethodDelete, hypervisor.ID, "/"+itoa(int(workload.ID))+"/match-rejections/"+itoa(guest.ID), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clear rejection status = %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkloadMatchRejectEndpointRefusesExactMACCandidate(t *testing.T) {
	router := setupTestRouter(t)
	hypervisor := seedHost(t, models.Host{Name: "pve", Mac: "AA:BB:CC:DD:F0:20", DeviceType: "server", Now: 1})
	guest := seedHost(t, models.Host{Name: "guest", Mac: "AA:BB:CC:DD:F0:21", DeviceType: "server", Now: 1})
	enableTestHypervisor(t, router, hypervisor.ID)

	rec := workloadRequest(router, http.MethodPost, hypervisor.ID, "", `{
		"nativeId":"220",
		"workloadType":"vm",
		"name":"guest",
		"interfaces":[{"name":"net0","mac":"AA:BB:CC:DD:F0:21"}]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create workload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var workload InfrastructureWorkloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &workload); err != nil {
		t.Fatalf("json.Unmarshal workload: %v", err)
	}
	matchesRec := getPath(router, "/api/host/"+itoa(hypervisor.ID)+"/workload-matches")
	var matches []InfrastructureWorkloadMatchResponse
	if err := json.Unmarshal(matchesRec.Body.Bytes(), &matches); err != nil || len(matches) != 1 || len(matches[0].Candidates) != 1 {
		t.Fatalf("exact matches=%+v err=%v", matches, err)
	}
	candidate := matches[0].Candidates[0]

	rec = workloadRequest(
		router,
		http.MethodPut,
		hypervisor.ID,
		"/"+itoa(int(workload.ID))+"/match-rejections/"+itoa(guest.ID),
		`{"evidenceFingerprint":"`+candidate.EvidenceFingerprint+`"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("exact candidate rejection status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
