package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestHostServiceScanSettingsDefaultAndSave(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "server",
		IP:    "192.168.1.80",
		Mac:   "AA:BB:CC:DD:EE:B0",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})
	path := "/api/host/" + strconv.Itoa(host.ID) + "/service-scan-settings"

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d; body: %s", rec.Code, rec.Body.String())
	}

	var initial serviceScanSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("GET json.Unmarshal: %v", err)
	}
	if initial.Enabled || initial.IntervalMinutes != defaultServiceScanIntervalMinutes || len(initial.Ports) != 0 {
		t.Fatalf("initial settings = %+v", initial)
	}

	body := []byte(`{"enabled":true,"intervalMinutes":60,"ports":[443,22,443]}`)
	req = httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d; body: %s", rec.Code, rec.Body.String())
	}

	var saved serviceScanSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("PUT json.Unmarshal: %v", err)
	}
	if !saved.Enabled || saved.IntervalMinutes != 60 || len(saved.Ports) != 2 || saved.Ports[0] != 22 || saved.Ports[1] != 443 || saved.NextScanAt == "" {
		t.Fatalf("saved settings = %+v", saved)
	}

	stored, found, err := gdb.SelectServiceScanSettingsByMAC(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectServiceScanSettingsByMAC found=%v err=%v", found, err)
	}
	if stored.PortsJSON != "[22,443]" || !stored.Enabled || stored.NextScanAt == "" {
		t.Fatalf("stored settings = %+v", stored)
	}
}

func TestHostServiceScanSettingsRejectsUnsafeEnabledConfig(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "server",
		IP:    "192.168.1.81",
		Mac:   "AA:BB:CC:DD:EE:B1",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})
	path := "/api/host/" + strconv.Itoa(host.ID) + "/service-scan-settings"

	tests := []string{
		`{"enabled":true,"intervalMinutes":60,"ports":[]}`,
		`{"enabled":true,"intervalMinutes":0,"ports":[443]}`,
		`{"enabled":true,"intervalMinutes":60,"ports":[0,443]}`,
		`{"enabled":true,"intervalMinutes":60,"ports":[65536]}`,
	}
	for _, body := range tests {
		req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want 400; response: %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestHostServiceScanSettingsDisableClearsNextScanAndPreservesRuntime(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "server",
		IP:    "192.168.1.82",
		Mac:   "AA:BB:CC:DD:EE:B2",
		Iface: "eth0",
		Known: 1,
		Now:   1,
	})
	if _, err := gdb.UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:              host.Mac,
		Enabled:          true,
		IntervalMinutes:  30,
		PortsJSON:        "[22]",
		NextScanAt:       "2026-09-18 11:00:00",
		LastAttemptAt:    "2026-09-18 10:00:00",
		LastSuccessfulAt: "2026-09-18 10:00:00",
		LastError:        "old error",
	}); err != nil {
		t.Fatalf("UpsertServiceScanSettings: %v", err)
	}

	path := "/api/host/" + strconv.Itoa(host.ID) + "/service-scan-settings"
	body := []byte(`{"enabled":false,"intervalMinutes":120,"ports":[22,443]}`)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d; body: %s", rec.Code, rec.Body.String())
	}

	stored, found, err := gdb.SelectServiceScanSettingsByMAC(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectServiceScanSettingsByMAC found=%v err=%v", found, err)
	}
	if stored.Enabled || stored.NextScanAt != "" || stored.IntervalMinutes != 120 || stored.LastAttemptAt != "2026-09-18 10:00:00" || stored.LastSuccessfulAt != "2026-09-18 10:00:00" || stored.LastError != "" {
		t.Fatalf("disabled settings = %+v", stored)
	}
}
