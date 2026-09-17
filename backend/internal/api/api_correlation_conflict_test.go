package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestCorrelationDecisionAPIRejectsContradictoryRelationshipGraph(t *testing.T) {
	router := setupTestRouter(t)
	macA := "02:AA:BB:CC:EF:01"
	macB := "06:AA:BB:CC:EF:02"
	macC := "0A:AA:BB:CC:EF:03"

	if err := gdb.UpdateWithError("now", models.Host{
		Mac: macA, IP: "10.4.1.81", Iface: "eth0", Date: "2026-09-17 19:00:00", Now: 1,
	}); err != nil {
		t.Fatalf("seed target host: %v", err)
	}
	hosts := gdb.SelectByMAC("now", macA)
	if len(hosts) != 1 {
		t.Fatalf("target host count = %d, want 1", len(hosts))
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: macC, IP: "10.4.1.83", Iface: "eth0", Date: "2026-09-17 18:30:00", Now: 1},
	}); err != nil {
		t.Fatalf("record observed MAC C: %v", err)
	}

	if _, err := gdb.SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationConfirmed, "2026-09-17 18:40:00"); err != nil {
		t.Fatalf("confirm A/B: %v", err)
	}
	if _, err := gdb.SetIdentityCorrelationDecision(macB, macC, models.IdentityCorrelationConfirmed, "2026-09-17 18:41:00"); err != nil {
		t.Fatalf("confirm B/C: %v", err)
	}

	path := "/api/host/" + strconv.Itoa(hosts[0].ID) + "/identity/decisions/" + macC
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(`{"decision":"rejected"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("conflicting decision status = %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
	if _, found, err := gdb.SelectIdentityCorrelationDecision(macA, macC); err != nil || found {
		t.Fatalf("conflicting decision must not persist, found=%v err=%v", found, err)
	}
}
