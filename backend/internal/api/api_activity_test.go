package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aceberg/WatchYourLAN/internal/gdb"
	"github.com/aceberg/WatchYourLAN/internal/models"
)

func TestActivityEndpointReturnsEmptyTable(t *testing.T) {
	router := setupTestRouter(t)

	rec := getPath(router, "/api/activity")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var events []models.HostEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events len = %d, want 0", len(events))
	}
}

func TestActivityEndpointDefaultAndExplicitLimit(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})

	for i := 0; i < 25; i++ {
		event := models.NewHostEvent(host, models.EventOnline, "", "")
		event.Date = fmt.Sprintf("2026-08-24 10:%02d:00", i)
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent %d: %v", i, err)
		}
	}

	rec := getPath(router, "/api/activity")
	if rec.Code != http.StatusOK {
		t.Fatalf("default status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if events := decodeActivityEvents(t, rec); len(events) != 20 {
		t.Fatalf("default events len = %d, want 20", len(events))
	}

	rec = getPath(router, "/api/activity?limit=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("explicit status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if events := decodeActivityEvents(t, rec); len(events) != 2 {
		t.Fatalf("explicit events len = %d, want 2", len(events))
	}
}

func TestActivityEndpointRejectsInvalidLimit(t *testing.T) {
	router := setupTestRouter(t)

	tests := []string{
		"/api/activity?limit=abc",
		"/api/activity?limit=0",
		"/api/activity?limit=-1",
		"/api/activity?limit=101",
		"/api/host/1/activity?limit=abc",
		"/api/host/1/activity?limit=0",
		"/api/host/1/activity?limit=-1",
		"/api/host/1/activity?limit=101",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			rec := getPath(router, path)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestActivityEndpointNewestFirstByDateAndID(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})

	first := models.NewHostEvent(host, models.EventDiscovered, "", "")
	first.Date = "2026-08-24 10:00:00"
	second := models.NewHostEvent(host, models.EventKnown, "", "")
	second.Date = "2026-08-24 10:00:00"
	third := models.NewHostEvent(host, models.EventOffline, "", "")
	third.Date = "2026-08-24 10:05:00"

	for _, event := range []models.HostEvent{first, second, third} {
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent: %v", err)
		}
	}

	rec := getPath(router, "/api/activity")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	events := decodeActivityEvents(t, rec)
	if len(events) != 3 {
		t.Fatalf("events len = %d, want 3", len(events))
	}
	want := []models.HostEventType{models.EventOffline, models.EventKnown, models.EventDiscovered}
	for i, eventType := range want {
		if events[i].EventType != string(eventType) {
			t.Fatalf("events[%d].EventType = %q, want %q; events: %+v", i, events[i].EventType, eventType, events)
		}
	}
}

func TestActivityEndpointFiltersByMacAndHostID(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	nasHost := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})

	routerEvent := models.NewHostEvent(routerHost, models.EventOnline, "", "")
	routerEvent.Date = "2026-08-24 10:00:00"
	nasEvent := models.NewHostEvent(nasHost, models.EventOffline, "", "")
	nasEvent.Date = "2026-08-24 10:05:00"

	for _, event := range []models.HostEvent{routerEvent, nasEvent} {
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent: %v", err)
		}
	}

	rec := getPath(router, "/api/activity?mac="+routerHost.Mac)
	if rec.Code != http.StatusOK {
		t.Fatalf("mac filter status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 1 || events[0].Mac != routerHost.Mac {
		t.Fatalf("mac filter events = %+v, want only router event", events)
	}

	rec = getPath(router, "/api/host/"+itoa(nasHost.ID)+"/activity")
	if rec.Code != http.StatusOK {
		t.Fatalf("host filter status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events = decodeActivityEvents(t, rec)
	if len(events) != 1 || events[0].HostID != nasHost.ID {
		t.Fatalf("host filter events = %+v, want only NAS event", events)
	}
}

func decodeActivityEvents(t *testing.T, rec *httptest.ResponseRecorder) []models.HostEvent {
	t.Helper()

	var events []models.HostEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return events
}
