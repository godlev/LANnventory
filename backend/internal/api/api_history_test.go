package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestHistoryByMACRejectsInvalidLimit(t *testing.T) {
	router := setupTestRouter(t)

	tests := []string{
		"/api/history/AA:BB:CC:DD:EE:FF",
		"/api/history/AA:BB:CC:DD:EE:FF?num=abc",
		"/api/history/AA:BB:CC:DD:EE:FF?num=0",
		"/api/history/AA:BB:CC:DD:EE:FF?num=-1",
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

func TestHistoryRowsDoNotIncludeInventoryMetadata(t *testing.T) {
	router := setupTestRouter(t)
	historyHost := models.Host{
		Name:       "NAS",
		DNS:        "nas.lan",
		Iface:      "eth0",
		IP:         "192.168.1.20",
		Mac:        "AA:BB:CC:DD:EE:20",
		Hw:         "Storage Vendor",
		Date:       "2026-09-05 10:00:00",
		Known:      1,
		Now:        0,
		DeviceType: "nas",
	}
	gdb.Update("history", historyHost)

	owner := "Storage Team"
	tags := []string{"backup"}
	pinned := true
	if _, err := gdb.UpsertHostMetadata(historyHost.Mac, models.HostMetadataUpdate{
		Owner:  &owner,
		Tags:   &tags,
		Pinned: &pinned,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}

	for _, path := range []string{
		"/api/history",
		"/api/history/" + historyHost.Mac + "?num=1",
		"/api/history/" + historyHost.Mac + "/2026-09-05",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			body := rec.Body.String()
			for _, field := range []string{`"Owner"`, `"Location"`, `"Notes"`, `"Tags"`, `"Pinned"`} {
				if strings.Contains(body, field) {
					t.Fatalf("history response contains metadata field %s: %s", field, body)
				}
			}
		})
	}
}
