package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
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

func TestHostPortScanRecordsOpenPortEvent(t *testing.T) {
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

	originalPortIsOpen := portIsOpen
	portIsOpen = func(addr, port string) bool {
		return addr == host.IP && port == "443"
	}
	t.Cleanup(func() {
		portIsOpen = originalPortIsOpen
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/port/443/scan", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result hostPortScanResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !result.Open || result.Port != 443 {
		t.Fatalf("result = %+v, want open port 443", result)
	}

	events, ok := gdb.SelectEventsByHostID(host.ID, 10)
	if !ok {
		t.Fatal("SelectEventsByHostID failed")
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1: %+v", len(events), events)
	}
	if events[0].EventType != string(models.EventPortOpen) || events[0].NewValue != "443" {
		t.Fatalf("event = %+v, want port-open NewValue=443", events[0])
	}
}

func TestHostPortScanDoesNotRecordClosedPort(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "desktop",
		IP:    "192.168.1.20",
		Mac:   "AA:BB:CC:DD:EE:91",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})

	originalPortIsOpen := portIsOpen
	portIsOpen = func(addr, port string) bool { return false }
	t.Cleanup(func() {
		portIsOpen = originalPortIsOpen
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/port/22/scan", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	events, ok := gdb.SelectEventsByHostID(host.ID, 10)
	if !ok {
		t.Fatal("SelectEventsByHostID failed")
	}
	if len(events) != 0 {
		t.Fatalf("closed port unexpectedly recorded events: %+v", events)
	}
}

func TestHostPortScanInvalidPortDoesNotRecordEvent(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "desktop",
		IP:    "192.168.1.20",
		Mac:   "AA:BB:CC:DD:EE:92",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})

	originalPortIsOpen := portIsOpen
	portIsOpen = func(addr, port string) bool {
		t.Fatal("portIsOpen called for invalid port")
		return false
	}
	t.Cleanup(func() {
		portIsOpen = originalPortIsOpen
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/port/65536/scan", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	events, ok := gdb.SelectEventsByHostID(host.ID, 10)
	if !ok {
		t.Fatal("SelectEventsByHostID failed")
	}
	if len(events) != 0 {
		t.Fatalf("invalid port unexpectedly recorded events: %+v", events)
	}
}

func TestHostPortScanInvalidHostDoesNotRecordEvent(t *testing.T) {
	router := setupTestRouter(t)

	originalPortIsOpen := portIsOpen
	portIsOpen = func(addr, port string) bool {
		t.Fatal("portIsOpen called for invalid host")
		return false
	}
	t.Cleanup(func() {
		portIsOpen = originalPortIsOpen
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/999/port/443/scan", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 0 {
		t.Fatalf("invalid host unexpectedly recorded events: %+v", events)
	}
}
