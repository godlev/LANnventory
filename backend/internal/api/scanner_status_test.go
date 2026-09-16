package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/arp"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/routines"
)

func TestScannerStatusEndpointUsesBackendScannerAndDatabaseState(t *testing.T) {
	oldScanner := scannerStateSnapshot
	oldDB := databaseHealthSnapshot
	oldNow := scannerStatusNow
	t.Cleanup(func() {
		scannerStateSnapshot = oldScanner
		databaseHealthSnapshot = oldDB
		scannerStatusNow = oldNow
	})

	started := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)
	completed := started.Add(2800 * time.Millisecond)
	next := completed.Add(37 * time.Second)
	serverNow := completed.Add(5 * time.Second)

	scannerStateSnapshot = func() routines.ScannerState {
		return routines.ScannerState{
			Status:               routines.ScannerStatusProblem,
			LastScanStartedAt:    started,
			LastScanAt:           completed,
			LastSuccessfulScanAt: started.Add(-10 * time.Minute),
			Duration:             2800 * time.Millisecond,
			DevicesFound:         34,
			Interfaces:           []string{"eth0"},
			LastErrors: []arp.ScanError{{
				Source:  "eth0",
				Command: "arp-scan -glNx -I eth0",
				Kind:    arp.ScanErrorTimeout,
				Message: "command timed out after 2m0s",
			}},
			NextScanAt: next,
		}
	}
	databaseHealthSnapshot = func() gdb.DatabaseHealth {
		return gdb.DatabaseHealth{Connected: true, Backend: "sqlite"}
	}
	scannerStatusNow = func() time.Time { return serverNow }

	gin.SetMode(gin.TestMode)
	router := gin.New()
	Routes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/scanner/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var response scannerStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if response.Status != "problem" || response.Scanning {
		t.Fatalf("scanner status = %+v", response)
	}
	if response.LastScanAt == nil || !response.LastScanAt.Equal(completed) {
		t.Fatalf("LastScanAt = %v, want %v", response.LastScanAt, completed)
	}
	if response.DurationMs != 2800 || response.DevicesFound != 34 {
		t.Fatalf("duration/devices = %d/%d", response.DurationMs, response.DevicesFound)
	}
	if len(response.Interfaces) != 1 || response.Interfaces[0] != "eth0" {
		t.Fatalf("Interfaces = %v, want [eth0]", response.Interfaces)
	}
	if response.LastError == nil || response.LastError.Kind != "timeout" || response.LastError.Source != "eth0" {
		t.Fatalf("LastError = %+v, want eth0 timeout", response.LastError)
	}
	if response.NextScanAt == nil || !response.NextScanAt.Equal(next) {
		t.Fatalf("NextScanAt = %v, want %v", response.NextScanAt, next)
	}
	if !response.ServerTime.Equal(serverNow) {
		t.Fatalf("ServerTime = %v, want %v", response.ServerTime, serverNow)
	}
	if response.Database.Status != "connected" || response.Database.Backend != "sqlite" {
		t.Fatalf("Database = %+v", response.Database)
	}
}

func TestScannerStatusEndpointUsesNullForUnknownTimes(t *testing.T) {
	oldScanner := scannerStateSnapshot
	oldDB := databaseHealthSnapshot
	oldNow := scannerStatusNow
	t.Cleanup(func() {
		scannerStateSnapshot = oldScanner
		databaseHealthSnapshot = oldDB
		scannerStatusNow = oldNow
	})

	scannerStateSnapshot = func() routines.ScannerState {
		return routines.ScannerState{}
	}
	databaseHealthSnapshot = func() gdb.DatabaseHealth {
		return gdb.DatabaseHealth{Connected: false, Error: "database is not started"}
	}
	scannerStatusNow = func() time.Time {
		return time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	Routes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/scanner/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var response scannerStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if response.Status != "problem" {
		t.Fatalf("Status = %q, want problem", response.Status)
	}
	if response.LastScanAt != nil || response.LastSuccessfulScanAt != nil || response.NextScanAt != nil {
		t.Fatalf("unknown timestamps should be null: %+v", response)
	}
	if response.Database.Status != "problem" {
		t.Fatalf("Database status = %q, want problem", response.Database.Status)
	}
}
