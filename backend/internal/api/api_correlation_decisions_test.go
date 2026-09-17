package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestCorrelationDecisionAPIConfirmListAndClearHistoricalMAC(t *testing.T) {
	router := setupTestRouter(t)
	targetMAC := "02:AA:BB:CC:EE:10"
	historicalMAC := "06:AA:BB:CC:EE:20"

	if err := gdb.UpdateWithError("now", models.Host{
		Name: "My Phone", IP: "10.4.1.51", Mac: targetMAC, Iface: "eth0",
		Date: "2026-09-17 18:00:00", Now: 1, DeviceType: "phone",
	}); err != nil {
		t.Fatalf("seed target host: %v", err)
	}
	hosts := gdb.SelectByMAC("now", targetMAC)
	if len(hosts) != 1 {
		t.Fatalf("target host count = %d, want 1", len(hosts))
	}

	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: historicalMAC, IP: "10.4.1.50", Iface: "eth0", Date: "2026-09-17 17:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record historical identity: %v", err)
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: targetMAC, IP: "10.4.1.51", Iface: "eth0", Date: "2026-09-17 18:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record target identity: %v", err)
	}

	oldNow := correlationDecisionNow
	correlationDecisionNow = func() time.Time {
		return time.Date(2026, 9, 17, 18, 30, 0, 0, time.UTC)
	}
	t.Cleanup(func() { correlationDecisionNow = oldNow })

	base := "/api/host/" + strconv.Itoa(hosts[0].ID) + "/identity/decisions/" + historicalMAC
	req := httptest.NewRequest(http.MethodPut, base, strings.NewReader(`{"decision":"confirmed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var decision IdentityCorrelationDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.Mac != historicalMAC || decision.Decision != models.IdentityCorrelationConfirmed {
		t.Fatalf("decision response = %+v", decision)
	}
	if decision.CreatedAt != "2026-09-17 18:30:00" || decision.UpdatedAt != decision.CreatedAt {
		t.Fatalf("decision timestamps = %+v", decision)
	}
	if decision.Exists || decision.HostID != 0 {
		t.Fatalf("historical identity must not require current Host row: %+v", decision)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(hosts[0].ID)+"/identity/decisions", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRec.Code, listRec.Body.String())
	}
	var listed HostIdentityDecisionsResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode decisions list: %v", err)
	}
	if listed.Mac != targetMAC || len(listed.Decisions) != 1 || listed.Decisions[0].Mac != historicalMAC || listed.Decisions[0].Decision != models.IdentityCorrelationConfirmed {
		t.Fatalf("listed decisions = %+v", listed)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, base, nil)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204; body: %s", deleteRec.Code, deleteRec.Body.String())
	}

	listRec = httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list-after-delete status = %d, want 200", listRec.Code)
	}
	listed = HostIdentityDecisionsResponse{}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list after delete: %v", err)
	}
	if len(listed.Decisions) != 0 {
		t.Fatalf("decision should be cleared, got %+v", listed.Decisions)
	}
}

func TestCorrelationDecisionAPIRejectsInvalidOrUnobservedPair(t *testing.T) {
	router := setupTestRouter(t)
	targetMAC := "02:AA:BB:CC:EE:31"
	if err := gdb.UpdateWithError("now", models.Host{
		IP: "10.4.1.61", Mac: targetMAC, Iface: "eth0", Date: "2026-09-17 18:00:00", Now: 1,
	}); err != nil {
		t.Fatalf("seed target host: %v", err)
	}
	hosts := gdb.SelectByMAC("now", targetMAC)
	if len(hosts) != 1 {
		t.Fatalf("target host count = %d, want 1", len(hosts))
	}
	prefix := "/api/host/" + strconv.Itoa(hosts[0].ID) + "/identity/decisions/"

	cases := []struct {
		name   string
		mac    string
		body   string
		status int
	}{
		{name: "same mac", mac: targetMAC, body: `{"decision":"confirmed"}`, status: http.StatusBadRequest},
		{name: "invalid mac", mac: "not-a-mac", body: `{"decision":"confirmed"}`, status: http.StatusBadRequest},
		{name: "unobserved mac", mac: "06:AA:BB:CC:EE:32", body: `{"decision":"confirmed"}`, status: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, prefix+tc.mac, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestCorrelationDecisionAPIUpdatesExistingDecision(t *testing.T) {
	router := setupTestRouter(t)
	targetMAC := "02:AA:BB:CC:EE:41"
	otherMAC := "06:AA:BB:CC:EE:42"

	if err := gdb.UpdateWithError("now", models.Host{Mac: targetMAC, IP: "10.4.1.70", Iface: "eth0", Date: "2026-09-17 18:00:00", Now: 1}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	hosts := gdb.SelectByMAC("now", targetMAC)
	if len(hosts) != 1 {
		t.Fatalf("target host count = %d", len(hosts))
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{{Mac: otherMAC, IP: "10.4.1.71", Iface: "eth0", Date: "2026-09-17 17:00:00", Now: 1}}); err != nil {
		t.Fatalf("record other: %v", err)
	}

	times := []time.Time{
		time.Date(2026, 9, 17, 18, 40, 0, 0, time.UTC),
		time.Date(2026, 9, 17, 18, 45, 0, 0, time.UTC),
	}
	oldNow := correlationDecisionNow
	idx := 0
	correlationDecisionNow = func() time.Time {
		value := times[idx]
		idx++
		return value
	}
	t.Cleanup(func() { correlationDecisionNow = oldNow })

	path := "/api/host/" + strconv.Itoa(hosts[0].ID) + "/identity/decisions/" + otherMAC
	for _, decision := range []string{models.IdentityCorrelationConfirmed, models.IdentityCorrelationRejected} {
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(`{"decision":"`+decision+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("set %s status = %d; body: %s", decision, rec.Code, rec.Body.String())
		}
	}

	row, found, err := gdb.SelectIdentityCorrelationDecision(targetMAC, otherMAC)
	if err != nil || !found {
		t.Fatalf("select updated decision found=%v err=%v", found, err)
	}
	if row.Decision != models.IdentityCorrelationRejected || row.CreatedAt != "2026-09-17 18:40:00" || row.UpdatedAt != "2026-09-17 18:45:00" {
		t.Fatalf("updated decision = %+v", row)
	}
}
