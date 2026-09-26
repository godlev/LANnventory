package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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
			for _, field := range []string{`"Owner"`, `"Location"`, `"Notes"`, `"Tags"`, `"Pinned"`, `"FirstSeen"`, `"LastSeen"`, `"FirstSeenEstimated"`} {
				if strings.Contains(body, field) {
					t.Fatalf("history response contains current-host enriched field %s: %s", field, body)
				}
			}
		})
	}
}


func TestHistoryDateRangeConvertsBrowserDayToServerClock(t *testing.T) {
	serverLocation := time.FixedZone("server", 0)
	from, to, err := historyDateRange(
		"2026-09-25T21:00:00Z",
		"2026-09-26T21:00:00Z",
		serverLocation,
	)
	if err != nil {
		t.Fatalf("historyDateRange: %v", err)
	}
	if from != "2026-09-25 21:00:00" || to != "2026-09-26 21:00:00" {
		t.Fatalf("range = %q -> %q, want Sofia local-day UTC bounds in server clock", from, to)
	}
}

func TestConvertHistoryDatesUsesRequestedBrowserTimeZone(t *testing.T) {
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	rows := []models.Host{{Date: "2026-09-25 22:18:00"}}
	convertHistoryDates(rows, time.UTC, sofia)

	if rows[0].Date != "2026-09-26 01:18:00" {
		t.Fatalf("converted date = %q, want browser-local 2026-09-26 01:18:00", rows[0].Date)
	}
}

func TestHistoryByDateRejectsPartialBrowserRange(t *testing.T) {
	router := setupTestRouter(t)
	path := "/api/history/AA:BB:CC:DD:EE:FF/2026-09-26?from=" +
		url.QueryEscape("2026-09-25T21:00:00Z")
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHistoryByDateUsesBrowserLocalDayRange(t *testing.T) {
	router := setupTestRouter(t)
	mac := "AA:BB:CC:DD:EE:66"
	fromInstant := time.Date(2026, 9, 25, 21, 0, 0, 0, time.UTC)
	toInstant := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	fromServer := fromInstant.In(time.Local).Format(historyDateLayout)
	toServer := toInstant.In(time.Local).Format(historyDateLayout)

	fromServerTime, err := time.ParseInLocation(historyDateLayout, fromServer, time.Local)
	if err != nil {
		t.Fatalf("ParseInLocation from: %v", err)
	}
	toServerTime, err := time.ParseInLocation(historyDateLayout, toServer, time.Local)
	if err != nil {
		t.Fatalf("ParseInLocation to: %v", err)
	}

	for _, date := range []string{
		fromServerTime.Add(-time.Second).Format(historyDateLayout),
		fromServerTime.Format(historyDateLayout),
		toServerTime.Add(-time.Second).Format(historyDateLayout),
		toServerTime.Format(historyDateLayout),
	} {
		gdb.Update("history", models.Host{Name: "presence-range", Mac: mac, Date: date})
	}

	path := "/api/history/" + mac + "/2026-09-26?from=" +
		url.QueryEscape(fromInstant.Format(time.RFC3339)) +
		"&to=" + url.QueryEscape(toInstant.Format(time.RFC3339)) +
		"&timeZone=" + url.QueryEscape("Europe/Sofia")
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Count(body, "\"Name\": \"presence-range\"") != 2 {
		t.Fatalf("browser-local range returned unexpected rows: %s", body)
	}
	if !strings.Contains(body, "\"Date\": \"2026-09-26 00:00:00\"") ||
		!strings.Contains(body, "\"Date\": \"2026-09-26 23:59:59\"") {
		t.Fatalf("browser-local dates not rendered in Europe/Sofia: %s", body)
	}
}
