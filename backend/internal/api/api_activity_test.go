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

	rec = getPath(router, "/api/activity?limit=20")
	if rec.Code != http.StatusOK {
		t.Fatalf("compatible status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if events := decodeActivityEvents(t, rec); len(events) != 20 {
		t.Fatalf("compatible events len = %d, want 20", len(events))
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

func TestActivityEndpointCategoryOmittedAndAllReturnEveryEventType(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	seedActivityEventTypes(t, host)

	for _, path := range []string{"/api/activity?limit=10", "/api/activity?category=all&limit=10"} {
		t.Run(path, func(t *testing.T) {
			rec := getPath(router, path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			events := decodeActivityEvents(t, rec)
			if len(events) != len(models.HostEventTypeValues) {
				t.Fatalf("events len = %d, want %d: %+v", len(events), len(models.HostEventTypeValues), events)
			}
		})
	}
}

func TestActivityEndpointFiltersByCategory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})
	seedActivityEventTypes(t, host)

	tests := []struct {
		path string
		want map[models.HostEventType]bool
	}{
		{
			path: "/api/activity?category=connectivity&limit=10",
			want: map[models.HostEventType]bool{
				models.EventOnline:  true,
				models.EventOffline: true,
			},
		},
		{
			path: "/api/activity?category=changes&limit=10",
			want: map[models.HostEventType]bool{
				models.EventDiscovered:        true,
				models.EventKnown:             true,
				models.EventUnknown:           true,
				models.EventDeviceTypeChanged: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := getPath(router, tt.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			events := decodeActivityEvents(t, rec)
			if len(events) != len(tt.want) {
				t.Fatalf("events len = %d, want %d: %+v", len(events), len(tt.want), events)
			}
			for _, event := range events {
				if !tt.want[models.HostEventType(event.EventType)] {
					t.Fatalf("unexpected event type %q in %s: %+v", event.EventType, tt.path, events)
				}
			}
		})
	}
}

func TestActivityEndpointRejectsInvalidCategory(t *testing.T) {
	router := setupTestRouter(t)

	for _, path := range []string{
		"/api/activity?category=",
		"/api/activity?category=online,offline",
		"/api/activity?category=ONLINE",
		"/api/activity?category=other",
	} {
		t.Run(path, func(t *testing.T) {
			rec := getPath(router, path)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestActivityEndpointOffsetDefaultAndExplicit(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "phone", Mac: "AA:BB:CC:DD:EE:83"})

	for i := 0; i < 5; i++ {
		event := models.NewHostEvent(host, models.EventOnline, "", "")
		event.Date = fmt.Sprintf("2026-08-24 10:%02d:00", i)
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent %d: %v", i, err)
		}
	}

	rec := getPath(router, "/api/activity?limit=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("default offset status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	defaultEvents := decodeActivityEvents(t, rec)

	rec = getPath(router, "/api/activity?limit=2&offset=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("explicit offset status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	offsetZeroEvents := decodeActivityEvents(t, rec)

	if len(defaultEvents) != len(offsetZeroEvents) || defaultEvents[0].Date != offsetZeroEvents[0].Date || defaultEvents[1].Date != offsetZeroEvents[1].Date {
		t.Fatalf("offset default events = %+v, want same as offset 0 %+v", defaultEvents, offsetZeroEvents)
	}

	rec = getPath(router, "/api/activity?limit=2&offset=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("offset status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	offsetEvents := decodeActivityEvents(t, rec)
	if len(offsetEvents) != 2 || offsetEvents[0].Date != "2026-08-24 10:02:00" || offsetEvents[1].Date != "2026-08-24 10:01:00" {
		t.Fatalf("offset events = %+v, want third and fourth newest", offsetEvents)
	}
}

func TestActivityEndpointRejectsInvalidOffset(t *testing.T) {
	router := setupTestRouter(t)

	for _, path := range []string{
		"/api/activity?offset=abc",
		"/api/activity?offset=-1",
	} {
		t.Run(path, func(t *testing.T) {
			rec := getPath(router, path)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestActivityEndpointCombinesCategoryOffsetAndLimit(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})

	events := []struct {
		eventType models.HostEventType
		date      string
	}{
		{models.EventKnown, "2026-08-24 10:00:00"},
		{models.EventOnline, "2026-08-24 10:01:00"},
		{models.EventDeviceTypeChanged, "2026-08-24 10:02:00"},
		{models.EventOffline, "2026-08-24 10:03:00"},
		{models.EventUnknown, "2026-08-24 10:04:00"},
		{models.EventDiscovered, "2026-08-24 10:05:00"},
	}
	for _, item := range events {
		event := models.NewHostEvent(host, item.eventType, "", "")
		event.Date = item.date
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent: %v", err)
		}
	}

	rec := getPath(router, "/api/activity?category=changes&limit=2&offset=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := decodeActivityEvents(t, rec)
	want := []models.HostEventType{models.EventUnknown, models.EventDeviceTypeChanged}
	if len(got) != len(want) {
		t.Fatalf("events len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, eventType := range want {
		if got[i].EventType != string(eventType) {
			t.Fatalf("events[%d].EventType = %q, want %q; events: %+v", i, got[i].EventType, eventType, got)
		}
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

func seedActivityEventTypes(t *testing.T, host models.Host) {
	t.Helper()

	for i, eventType := range models.HostEventTypeValues {
		event := models.NewHostEvent(host, eventType, "", "")
		event.Date = fmt.Sprintf("2026-08-24 10:%02d:00", i)
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent %s: %v", eventType, err)
		}
	}
}
