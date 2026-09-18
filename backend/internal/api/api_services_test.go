package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestHostServicesEndpointReturnsRetainedServicesAcrossAddresses(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "nas",
		IP:    "192.168.1.50",
		Mac:   "AA:BB:CC:DD:EE:A0",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})
	otherHost := seedHost(t, models.Host{
		Name:  "other",
		IP:    "192.168.1.60",
		Mac:   "AA:BB:CC:DD:EE:A1",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})

	observations := []models.Service{
		{
			Mac:            host.Mac,
			Address:        host.IP,
			Protocol:       "tcp",
			Port:           443,
			State:          "open",
			FirstDetected:  "2026-09-18 09:00:00",
			LastDetected:   "2026-09-18 10:00:00",
			LastChecked:    "2026-09-18 10:00:00",
			StateChangedAt: "2026-09-18 09:00:00",
			LastScanSource: "manual",
		},
		{
			Mac:            host.Mac,
			Address:        "fd00::50",
			Protocol:       "tcp",
			Port:           22,
			State:          "closed",
			FirstDetected:  "2026-09-17 09:00:00",
			LastDetected:   "2026-09-17 09:00:00",
			LastChecked:    "2026-09-18 10:05:00",
			StateChangedAt: "2026-09-18 10:05:00",
			LastScanSource: "manual",
		},
		{
			Mac:            otherHost.Mac,
			Address:        otherHost.IP,
			Protocol:       "tcp",
			Port:           8080,
			State:          "open",
			FirstDetected:  "2026-09-18 08:00:00",
			LastDetected:   "2026-09-18 08:00:00",
			LastChecked:    "2026-09-18 08:00:00",
			StateChangedAt: "2026-09-18 08:00:00",
			LastScanSource: "manual",
		},
	}
	for _, service := range observations {
		if _, err := gdb.UpsertService(service); err != nil {
			t.Fatalf("UpsertService(%+v): %v", service, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(host.ID)+"/services", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var services []models.Service
	if err := json.Unmarshal(rec.Body.Bytes(), &services); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("services len = %d, want 2: %+v", len(services), services)
	}

	byPort := make(map[int]models.Service, len(services))
	for _, service := range services {
		byPort[service.Port] = service
		if service.Mac != host.Mac {
			t.Fatalf("unexpected service for another MAC: %+v", service)
		}
	}

	httpsService, ok := byPort[443]
	if !ok || httpsService.State != "open" || httpsService.Address != host.IP || httpsService.AddressFamily != "ipv4" {
		t.Fatalf("HTTPS service = %+v", httpsService)
	}
	sshService, ok := byPort[22]
	if !ok || sshService.State != "closed" || sshService.Address != "fd00::50" || sshService.AddressFamily != "ipv6" {
		t.Fatalf("SSH service = %+v", sshService)
	}
}

func TestHostServicesEndpointReturnsEmptyArray(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "empty",
		IP:    "192.168.1.70",
		Mac:   "AA:BB:CC:DD:EE:A2",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(host.ID)+"/services", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("body = %q, want []", rec.Body.String())
	}
}
