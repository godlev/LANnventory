package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestHostPortRangeScanPersistsOnlyStateTransitions(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "server",
		IP:         "192.168.1.90",
		Mac:        "AA:BB:CC:DD:EE:92",
		Iface:      "eth0",
		DeviceType: "server",
		Known:      1,
		Now:        1,
	})

	originalScanPortRange := scanPortRange
	originalJobs := portScanJobs
	portScanJobs = newPortScanJobManager()
	t.Cleanup(func() {
		scanPortRange = originalScanPortRange
		portScanJobs = originalJobs
	})

	scanPortRange = func(
		ctx context.Context,
		target string,
		start int,
		end int,
		workers int,
		timeout time.Duration,
		onResult func(portscan.Result),
	) error {
		if target != host.IP || start != 22 || end != 80 {
			t.Fatalf("scan args target=%q start=%d end=%d", target, start, end)
		}
		onResult(portscan.Result{Port: 22, Open: true})
		onResult(portscan.Result{Port: 23, Open: false})
		onResult(portscan.Result{Port: 80, Open: true})
		return nil
	}

	first := startRangeScanRequest(t, router, host.ID, 22, 80)
	first = waitForRangeScan(t, router, host.ID, first.ID)
	if !first.Completed || first.Running || first.Cancelled {
		t.Fatalf("first scan status = %+v", first)
	}
	if len(first.OpenPorts) != 2 || first.OpenPorts[0] != 22 || first.OpenPorts[1] != 80 {
		t.Fatalf("first open ports = %+v", first.OpenPorts)
	}

	states, err := gdb.SelectHostPorts(host.ID)
	if err != nil {
		t.Fatalf("SelectHostPorts: %v", err)
	}
	if len(states) != 2 || states[0].Port != 22 || states[1].Port != 80 {
		t.Fatalf("states = %+v, want open 22 and 80", states)
	}

	events, ok := gdb.SelectEventsByHostID(host.ID, 20)
	if !ok || len(events) != 2 {
		t.Fatalf("first events = %+v, ok=%v", events, ok)
	}

	second := startRangeScanRequest(t, router, host.ID, 22, 80)
	_ = waitForRangeScan(t, router, host.ID, second.ID)
	events, ok = gdb.SelectEventsByHostID(host.ID, 20)
	if !ok || len(events) != 2 {
		t.Fatalf("same-state rescan created duplicate events: %+v", events)
	}

	scanPortRange = func(
		ctx context.Context,
		target string,
		start int,
		end int,
		workers int,
		timeout time.Duration,
		onResult func(portscan.Result),
	) error {
		onResult(portscan.Result{Port: 22, Open: false})
		onResult(portscan.Result{Port: 80, Open: true})
		return nil
	}

	third := startRangeScanRequest(t, router, host.ID, 22, 80)
	_ = waitForRangeScan(t, router, host.ID, third.ID)
	events, ok = gdb.SelectEventsByHostID(host.ID, 20)
	if !ok || len(events) != 3 {
		t.Fatalf("close transition events = %+v, ok=%v", events, ok)
	}
	if events[0].EventType != string(models.EventPortClosed) || events[0].NewValue != "22" {
		t.Fatalf("latest event = %+v, want port-closed 22", events[0])
	}
}

func TestHostPortRangeScanCanBeCancelled(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "desktop",
		IP:    "192.168.1.40",
		Mac:   "AA:BB:CC:DD:EE:93",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})

	originalScanPortRange := scanPortRange
	originalJobs := portScanJobs
	portScanJobs = newPortScanJobManager()
	t.Cleanup(func() {
		scanPortRange = originalScanPortRange
		portScanJobs = originalJobs
	})

	started := make(chan struct{})
	scanPortRange = func(
		ctx context.Context,
		target string,
		start int,
		end int,
		workers int,
		timeout time.Duration,
		onResult func(portscan.Result),
	) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	status := startRangeScanRequest(t, router, host.ID, 1, 100)
	<-started

	activeReq := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(host.ID)+"/ports/scan", nil)
	activeRec := httptest.NewRecorder()
	router.ServeHTTP(activeRec, activeReq)
	if activeRec.Code != http.StatusOK {
		t.Fatalf("active scan status = %d; body: %s", activeRec.Code, activeRec.Body.String())
	}
	var active portScanJobStatus
	if err := json.Unmarshal(activeRec.Body.Bytes(), &active); err != nil {
		t.Fatalf("json.Unmarshal active scan: %v", err)
	}
	if active.ID != status.ID || !active.Running {
		t.Fatalf("active scan = %+v, want running job %s", active, status.ID)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/host/"+strconv.Itoa(host.ID)+"/ports/scan/"+status.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d; body: %s", rec.Code, rec.Body.String())
	}

	status = waitForRangeScan(t, router, host.ID, status.ID)
	if !status.Completed || !status.Cancelled || status.Running {
		t.Fatalf("cancelled scan status = %+v", status)
	}

	activeReq = httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(host.ID)+"/ports/scan", nil)
	activeRec = httptest.NewRecorder()
	router.ServeHTTP(activeRec, activeReq)
	if activeRec.Code != http.StatusNotFound {
		t.Fatalf("active scan after completion status = %d, want %d; body: %s", activeRec.Code, http.StatusNotFound, activeRec.Body.String())
	}
}

func startRangeScanRequest(t *testing.T, router http.Handler, hostID, startPort, endPort int) portScanJobStatus {
	t.Helper()

	body := strings.NewReader(`{"startPort":` + strconv.Itoa(startPort) + `,"endPort":` + strconv.Itoa(endPort) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(hostID)+"/ports/scan", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start scan status = %d; body: %s", rec.Code, rec.Body.String())
	}

	var status portScanJobStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal start scan: %v", err)
	}
	if status.ID == "" || !status.Running {
		t.Fatalf("start scan status = %+v", status)
	}
	return status
}

func waitForRangeScan(t *testing.T, router http.Handler, hostID int, scanID string) portScanJobStatus {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(hostID)+"/ports/scan/"+scanID, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("scan status = %d; body: %s", rec.Code, rec.Body.String())
		}

		var status portScanJobStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("json.Unmarshal scan status: %v", err)
		}
		if status.Completed {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("scan %s did not complete before timeout", scanID)
	return portScanJobStatus{}
}
