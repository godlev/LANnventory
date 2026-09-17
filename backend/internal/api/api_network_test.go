package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/portscan"
)

func TestPortEndpointRejectsInvalidPort(t *testing.T) {
	router := setupTestRouter(t)

	tests := []string{
		"/api/port/127.0.0.1/not-a-port",
		"/api/port/127.0.0.1/0",
		"/api/port/127.0.0.1/-1",
		"/api/port/127.0.0.1/65536",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHostPortScanPersistsOpenServiceAndLegacyTransitionEvent(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "router",
		IP:         "192.168.1.1",
		Mac:        "AA:BB:CC:DD:EE:90",
		Iface:      "eth0",
		DeviceType: "router",
		Known:      1,
		Now:        1,
	})

	originalPortProbe := portProbe
	portProbe = func(_ context.Context, addr, port string) portscan.Result {
		if addr == host.IP && port == "443" {
			return portscan.Result{State: portscan.ProbeOpen}
		}
		return portscan.Result{State: portscan.ProbeClosed}
	}
	t.Cleanup(func() {
		portProbe = originalPortProbe
	})

	path := "/api/host/" + strconv.Itoa(host.ID) + "/port/443/scan"
	for attempt := 0; attempt < 2; attempt++ {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want %d; body: %s", attempt+1, rec.Code, http.StatusOK, rec.Body.String())
		}

		var result hostPortScanResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if !result.Open || result.Port != 443 {
			t.Fatalf("result = %+v, want open port 443", result)
		}
	}

	service, ok, err := gdb.SelectServiceByIdentity(host.Mac, host.IP, "tcp", 443)
	if err != nil || !ok {
		t.Fatalf("SelectServiceByIdentity ok=%v err=%v", ok, err)
	}
	if service.State != "open" || service.Address != host.IP || service.LastScanSource != "manual" || service.FirstDetected == "" || service.LastDetected == "" || service.LastChecked == "" {
		t.Fatalf("persisted service = %+v", service)
	}

	events, ok := gdb.SelectEventsByHostID(host.ID, 10)
	if !ok {
		t.Fatal("SelectEventsByHostID failed")
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want one transition event after repeated open scans: %+v", len(events), events)
	}
	if events[0].EventType != string(models.EventPortOpen) || events[0].NewValue != "443" {
		t.Fatalf("event = %+v, want port-open NewValue=443", events[0])
	}
}

func TestHostPortScanDoesNotPersistNeverSeenClosedPort(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "desktop",
		IP:    "192.168.1.20",
		Mac:   "AA:BB:CC:DD:EE:91",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})

	originalPortProbe := portProbe
	portProbe = func(_ context.Context, addr, port string) portscan.Result {
		return portscan.Result{State: portscan.ProbeClosed}
	}
	t.Cleanup(func() {
		portProbe = originalPortProbe
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/port/22/scan", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	services, err := gdb.SelectServicesByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectServicesByMAC: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("never-seen closed port unexpectedly persisted services: %+v", services)
	}

	events, ok := gdb.SelectEventsByHostID(host.ID, 10)
	if !ok {
		t.Fatal("SelectEventsByHostID failed")
	}
	if len(events) != 0 {
		t.Fatalf("closed port unexpectedly recorded events: %+v", events)
	}
}

func TestHostPortScanDoesNotChangeStateOnIndeterminateFailure(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "nas",
		IP:    "192.168.1.30",
		Mac:   "AA:BB:CC:DD:EE:92",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})

	if _, _, _, err := gdb.RecordServiceObservation(models.Service{
		Mac:            host.Mac,
		Address:        host.IP,
		Protocol:       "tcp",
		Port:           445,
		State:          "open",
		LastScanSource: "manual",
	}, "2026-09-18 10:00:00"); err != nil {
		t.Fatalf("seed service observation: %v", err)
	}

	originalPortProbe := portProbe
	portProbe = func(_ context.Context, addr, port string) portscan.Result {
		return portscan.Result{State: portscan.ProbeIndeterminate, Err: errors.New("network unreachable")}
	}
	t.Cleanup(func() {
		portProbe = originalPortProbe
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/port/445/scan", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	service, ok, err := gdb.SelectServiceByIdentity(host.Mac, host.IP, "tcp", 445)
	if err != nil || !ok {
		t.Fatalf("SelectServiceByIdentity ok=%v err=%v", ok, err)
	}
	if service.State != "open" || service.LastChecked != "2026-09-18 10:00:00" || service.LastDetected != "2026-09-18 10:00:00" {
		t.Fatalf("indeterminate scan changed persisted state: %+v", service)
	}
}
