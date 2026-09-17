package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestActivityEndpointReturnsNewestEventsFirst(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "router",
		IP:         "192.168.1.1",
		Mac:        "AA:BB:CC:DD:EE:01",
		Iface:      "eth0",
		DeviceType: "router",
	})

	seedActivityEvent(t, host, models.EventOnline, "2026-08-24 10:00:00")
	seedActivityEvent(t, host, models.EventOffline, "2026-08-24 10:05:00")

	rec := getPath(router, "/api/activity?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	events := decodeActivityEvents(t, rec)
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2: %+v", len(events), events)
	}
	if events[0].EventType != string(models.EventOffline) || events[1].EventType != string(models.EventOnline) {
		t.Fatalf("events order = %+v, want offline then online", events)
	}
}

func TestActivityEndpointReturnsDeterministicOrderForEqualDates(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})

	first := seedActivityEvent(t, host, models.EventOnline, "2026-08-24 10:00:00")
	second := seedActivityEvent(t, host, models.EventOffline, "2026-08-24 10:00:00")

	rec := getPath(router, "/api/activity?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2: %+v", len(events), events)
	}
	if events[0].ID != second.ID || events[1].ID != first.ID {
		t.Fatalf("equal-date order IDs = [%d %d], want [%d %d]", events[0].ID, events[1].ID, second.ID, first.ID)
	}
}

func TestActivityEndpointRejectsInvalidLimit(t *testing.T) {
	router := setupTestRouter(t)

	for _, limit := range []string{"0", "101", "abc"} {
		rec := getPath(router, "/api/activity?limit="+limit)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("limit %q status = %d, want %d", limit, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestActivityEndpointSupportsLegacyOffsetPagination(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	for i := 0; i < 5; i++ {
		seedActivityEvent(t, host, models.EventOnline, "2026-08-24 10:0"+strconv.Itoa(i)+":00")
	}

	rec := getPath(router, "/api/activity?limit=2&offset=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 2 || events[0].Date != "2026-08-24 10:02:00" || events[1].Date != "2026-08-24 10:01:00" {
		t.Fatalf("offset events = %+v, want 10:02 then 10:01", events)
	}
}

func TestActivityEndpointSupportsCursorPagination(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})

	seedActivityEvent(t, host, models.EventOnline, "2026-08-24 10:00:00")
	middleFirst := seedActivityEvent(t, host, models.EventKnown, "2026-08-24 10:05:00")
	middleSecond := seedActivityEvent(t, host, models.EventUnknown, "2026-08-24 10:05:00")
	seedActivityEvent(t, host, models.EventOffline, "2026-08-24 10:10:00")

	firstRec := getPath(router, "/api/activity?limit=2")
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first page status = %d, want %d; body: %s", firstRec.Code, http.StatusOK, firstRec.Body.String())
	}
	firstPage := decodeActivityEvents(t, firstRec)
	if len(firstPage) != 2 || firstPage[0].EventType != string(models.EventOffline) || firstPage[1].ID != middleSecond.ID {
		t.Fatalf("first page = %+v, want offline then newest equal-date event", firstPage)
	}

	cursor := firstPage[len(firstPage)-1]
	secondPath := "/api/activity?limit=2&beforeDate=" + url.QueryEscape(cursor.Date) + "&beforeId=" + strconv.Itoa(cursor.ID)
	secondRec := getPath(router, secondPath)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second page status = %d, want %d; body: %s", secondRec.Code, http.StatusOK, secondRec.Body.String())
	}
	secondPage := decodeActivityEvents(t, secondRec)
	if len(secondPage) != 2 || secondPage[0].ID != middleFirst.ID || secondPage[1].EventType != string(models.EventOnline) {
		t.Fatalf("second page = %+v, want older equal-date event then online", secondPage)
	}
}

func TestActivityEndpointRejectsInvalidCursor(t *testing.T) {
	router := setupTestRouter(t)

	paths := []string{
		"/api/activity?beforeDate=2026-08-24+10%3A00%3A00",
		"/api/activity?beforeId=1",
		"/api/activity?beforeDate=bad&beforeId=1",
		"/api/activity?beforeDate=2026-08-24+10%3A00%3A00&beforeId=0",
		"/api/activity?beforeDate=2026-08-24+10%3A00%3A00&beforeId=1&offset=1",
	}

	for _, path := range paths {
		rec := getPath(router, path)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path %q status = %d, want %d; body: %s", path, rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	}
}

func TestActivityEndpointAcceptsExplicitZeroOffsetWithCursor(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	event := seedActivityEvent(t, host, models.EventOnline, "2026-08-24 10:00:00")

	path := "/api/activity?limit=10&offset=0&beforeDate=" + url.QueryEscape(event.Date) + "&beforeId=" + strconv.Itoa(event.ID)
	rec := getPath(router, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestActivityEndpointFiltersByCategory(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	seedActivityEventTypes(t, host)

	tests := []struct {
		category string
		want     map[string]bool
	}{
		{
			category: "connectivity",
			want: map[string]bool{
				string(models.EventOnline):  true,
				string(models.EventOffline): true,
			},
		},
		{
			category: "changes",
			want: eventTypeSet(models.DeviceChangeEventTypes),
		},
	}

	for _, test := range tests {
		t.Run(test.category, func(t *testing.T) {
			rec := getPath(router, "/api/activity?category="+test.category+"&limit=100")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			events := decodeActivityEvents(t, rec)
			if len(events) != len(test.want) {
				t.Fatalf("category %s events len = %d, want %d: %+v", test.category, len(events), len(test.want), events)
			}
			for _, event := range events {
				if !test.want[event.EventType] {
					t.Fatalf("category %s returned unexpected type %q", test.category, event.EventType)
				}
			}
		})
	}
}

func TestActivityEndpointRejectsInvalidCategory(t *testing.T) {
	router := setupTestRouter(t)
	rec := getPath(router, "/api/activity?category=network")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestActivityEndpointFiltersByEventType(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	seedActivityEventTypes(t, host)

	rec := getPath(router, "/api/activity?eventType=online&eventType=pinned-changed&limit=100")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2: %+v", len(events), events)
	}
	if events[0].EventType != string(models.EventPinnedChanged) || events[1].EventType != string(models.EventOnline) {
		t.Fatalf("events order/types = %+v", events)
	}
}

func TestActivityEndpointRejectsInvalidEventType(t *testing.T) {
	router := setupTestRouter(t)
	rec := getPath(router, "/api/activity?eventType=bad")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestActivityEndpointCombinesCategoryAndEventTypes(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	seedActivityEventTypes(t, host)

	rec := getPath(router, "/api/activity?category=connectivity&eventType=online&eventType=pinned-changed&limit=100")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 1 || events[0].EventType != string(models.EventOnline) {
		t.Fatalf("events = %+v, want only online", events)
	}
}

func TestActivityEndpointReturnsEmptyForDisjointCategoryAndEventTypes(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	seedActivityEventTypes(t, host)

	rec := getPath(router, "/api/activity?category=connectivity&eventType=pinned-changed&limit=100")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 0 {
		t.Fatalf("events = %+v, want empty result", events)
	}
}

func TestActivityEndpointFiltersByDevice(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	nasHost := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})

	seedActivityEvent(t, routerHost, models.EventOnline, "2026-08-24 10:00:00")
	seedActivityEvent(t, nasHost, models.EventOffline, "2026-08-24 10:05:00")

	rec := getPath(router, "/api/activity?mac="+url.QueryEscape(nasHost.Mac)+"&limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 1 || events[0].Mac != nasHost.Mac {
		t.Fatalf("events = %+v, want only NAS event", events)
	}
}

func TestActivityEndpointFiltersByMultipleDevices(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	nasHost := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})
	phoneHost := seedHost(t, models.Host{Name: "phone", Mac: "AA:BB:CC:DD:EE:83"})

	seedActivityEvent(t, routerHost, models.EventOnline, "2026-08-24 10:00:00")
	seedActivityEvent(t, nasHost, models.EventOffline, "2026-08-24 10:05:00")
	seedActivityEvent(t, phoneHost, models.EventKnown, "2026-08-24 10:10:00")

	query := "/api/activity?limit=10&mac=" + url.QueryEscape(routerHost.Mac) + "&mac=" + url.QueryEscape(phoneHost.Mac)
	rec := getPath(router, query)
	if rec.Code != http.StatusOK {
		t.Fatalf("multi-mac status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 2 {
		t.Fatalf("multi-mac events len = %d, want 2: %+v", len(events), events)
	}
	for _, event := range events {
		if event.Mac != routerHost.Mac && event.Mac != phoneHost.Mac {
			t.Fatalf("unexpected mac in multi-mac results: %+v", events)
		}
	}
}

func TestActivityEndpointSupportsRepeatedMacFilters(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	nasHost := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})
	phoneHost := seedHost(t, models.Host{Name: "phone", Mac: "AA:BB:CC:DD:EE:83"})

	seedActivityEvent(t, routerHost, models.EventOnline, "2026-08-24 10:00:00")
	seedActivityEvent(t, nasHost, models.EventOffline, "2026-08-24 10:05:00")
	seedActivityEvent(t, phoneHost, models.EventKnown, "2026-08-24 10:10:00")

	query := "/api/activity?limit=10&mac=" + url.QueryEscape(routerHost.Mac) + "&mac=" + url.QueryEscape(nasHost.Mac)
	rec := getPath(router, query)
	if rec.Code != http.StatusOK {
		t.Fatalf("multi-mac status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 2 {
		t.Fatalf("multi-mac events len = %d, want 2: %+v", len(events), events)
	}
	for _, event := range events {
		if event.Mac != routerHost.Mac && event.Mac != nasHost.Mac {
			t.Fatalf("unexpected mac in multi-mac results: %+v", events)
		}
	}
}

func TestActivityEndpointCombinesCategoryAndMultipleMacs(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	nasHost := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})
	phoneHost := seedHost(t, models.Host{Name: "phone", Mac: "AA:BB:CC:DD:EE:83"})

	seedActivityEvent(t, routerHost, models.EventOnline, "2026-08-24 10:00:00")
	seedActivityEvent(t, routerHost, models.EventKnown, "2026-08-24 10:01:00")
	seedActivityEvent(t, nasHost, models.EventOffline, "2026-08-24 10:05:00")
	seedActivityEvent(t, phoneHost, models.EventOnline, "2026-08-24 10:10:00")

	query := "/api/activity?category=connectivity&limit=10&mac=" + url.QueryEscape(routerHost.Mac) + "&mac=" + url.QueryEscape(nasHost.Mac)
	rec := getPath(router, query)
	if rec.Code != http.StatusOK {
		t.Fatalf("category multi-mac status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 2 {
		t.Fatalf("category multi-mac events len = %d, want 2: %+v", len(events), events)
	}
	for _, event := range events {
		if event.Mac != routerHost.Mac && event.Mac != nasHost.Mac {
			t.Fatalf("unexpected mac in category multi-mac results: %+v", events)
		}
		if event.EventType != string(models.EventOnline) && event.EventType != string(models.EventOffline) {
			t.Fatalf("unexpected event type in category multi-mac results: %+v", events)
		}
	}
}

func TestActivityStatsEndpointReturnsTotals(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	nasHost := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})

	seedActivityEventTypes(t, routerHost)
	seedActivityEvent(t, nasHost, models.EventOnline, "2026-08-24 11:00:00")
	seedActivityEvent(t, nasHost, models.EventOffline, "2026-08-24 11:01:00")

	rec := getPath(router, "/api/activity/stats")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	stats := decodeActivityStats(t, rec)
	wantTotal := int64(len(models.HostEventTypeValues) + 2)
	if stats.Total != wantTotal || stats.Online != 2 || stats.Offline != 2 || stats.Discovered != 1 || stats.Known != 1 || stats.Unknown != 1 || stats.DeviceTypeChanged != 1 || stats.MetadataChanged != 5 {
		t.Fatalf("stats = %+v, want totals for all %d seeded event types plus two NAS connectivity events", stats, len(models.HostEventTypeValues))
	}
}

func TestActivityStatsEndpointAppliesDeviceFilterOnly(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	nasHost := seedHost(t, models.Host{Name: "NAS", Mac: "AA:BB:CC:DD:EE:20"})

	seedActivityEventTypes(t, routerHost)
	seedActivityEvent(t, nasHost, models.EventOnline, "2026-08-24 11:00:00")
	seedActivityEvent(t, nasHost, models.EventOffline, "2026-08-24 11:01:00")

	rec := getPath(router, "/api/activity/stats?mac="+url.QueryEscape(nasHost.Mac))
	if rec.Code != http.StatusOK {
		t.Fatalf("filtered stats status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	stats := decodeActivityStats(t, rec)
	if stats.Total != 2 || stats.Online != 1 || stats.Offline != 1 || stats.Discovered != 0 || stats.Known != 0 || stats.Unknown != 0 || stats.DeviceTypeChanged != 0 || stats.MetadataChanged != 0 {
		t.Fatalf("filtered stats = %+v, want only NAS counts", stats)
	}
}

func TestActivityDevicesEndpointIncludesCurrentAndDeletedEventDevices(t *testing.T) {
	router := setupTestRouter(t)
	currentHost := seedHost(t, models.Host{
		Name:       "router",
		IP:         "192.168.1.1",
		Mac:        "AA:BB:CC:DD:EE:01",
		DeviceType: "router",
	})
	quietHost := seedHost(t, models.Host{
		Name:       "quiet",
		IP:         "192.168.1.2",
		Mac:        "AA:BB:CC:DD:EE:02",
		DeviceType: "server",
	})
	deletedHost := seedHost(t, models.Host{
		Name:       "old-camera",
		IP:         "192.168.1.90",
		Mac:        "AA:BB:CC:DD:EE:90",
		DeviceType: "camera",
	})
	seedActivityEvent(t, currentHost, models.EventOnline, "2026-08-24 10:00:00")
	seedActivityEvent(t, deletedHost, models.EventDiscovered, "2026-08-24 09:00:00")
	if err := gdb.DeleteCurrentHostWithMetadata(deletedHost); err != nil {
		t.Fatalf("DeleteCurrentHostWithMetadata: %v", err)
	}

	rec := getPath(router, "/api/activity/devices")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	devices := decodeActivityDevices(t, rec)
	if len(devices) != 3 {
		t.Fatalf("devices len = %d, want 3: %+v", len(devices), devices)
	}

	byMac := make(map[string]models.ActivityDeviceOption, len(devices))
	for _, device := range devices {
		byMac[device.Mac] = device
	}
	if got := byMac[currentHost.Mac]; !got.Exists || got.HostID != currentHost.ID || got.Name != currentHost.Name {
		t.Fatalf("current device = %+v, want existing current host", got)
	}
	if got := byMac[quietHost.Mac]; !got.Exists || got.HostID != quietHost.ID || got.Name != quietHost.Name {
		t.Fatalf("quiet current device = %+v, want current host without events", got)
	}
	if got := byMac[deletedHost.Mac]; got.Exists || got.HostID != deletedHost.ID || got.Name != deletedHost.Name || got.DeviceType != deletedHost.DeviceType {
		t.Fatalf("deleted device = %+v, want retained event snapshot", got)
	}
}

func TestHostActivityEndpointUsesHostSnapshot(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "camera",
		IP:         "192.168.1.50",
		Mac:        "AA:BB:CC:DD:EE:50",
		Iface:      "eth0",
		DeviceType: "camera",
	})
	seedActivityEvent(t, host, models.EventOnline, "2026-08-24 10:00:00")

	rec := getPath(router, "/api/host/"+strconv.Itoa(host.ID)+"/activity?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 1 || events[0].HostID != host.ID || events[0].Mac != host.Mac || events[0].Name != host.Name || events[0].IP != host.IP || events[0].DeviceType != host.DeviceType {
		t.Fatalf("host event = %+v", events)
	}
}

func TestActivityDisplayTimePreservesStoredDate(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{Name: "router", Mac: "AA:BB:CC:DD:EE:01"})
	storedDate := "2026-09-03 18:18:43"
	seedActivityEvent(t, host, models.EventOnline, storedDate)

	rec := getPath(router, "/api/activity?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	events := decodeActivityEvents(t, rec)
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1: %+v", len(events), events)
	}
	if events[0].Date != storedDate {
		t.Fatalf("stored Date = %q, want %q", events[0].Date, storedDate)
	}
	if events[0].DateUTC == "" {
		t.Fatal("DateUTC should be populated for display")
	}
}

func TestActivityStatsEndpointRejectsDatabaseFailure(t *testing.T) {
	router := setupTestRouter(t)
	if err := gdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rec := getPath(router, "/api/activity/stats")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestActivityDevicesEndpointRejectsDatabaseFailure(t *testing.T) {
	router := setupTestRouter(t)
	if err := gdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rec := getPath(router, "/api/activity/devices")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func seedActivityEventTypes(t *testing.T, host models.Host) {
	t.Helper()

	for index, eventType := range models.HostEventTypeValues {
		seedActivityEvent(t, host, eventType, "2026-08-24 10:"+twoDigit(index)+":00")
	}
}

func seedActivityEvent(t *testing.T, host models.Host, eventType models.HostEventType, date string) models.HostEvent {
	t.Helper()

	event := models.NewHostEvent(host, eventType, "", "")
	event.Date = date
	if err := gdb.AddEvent(event); err != nil {
		t.Fatalf("AddEvent(%s): %v", eventType, err)
	}

	events, ok := gdb.SelectEventsByHostID(host.ID, 100)
	if !ok {
		t.Fatalf("SelectEventsByHostID failed after AddEvent(%s)", eventType)
	}
	for _, stored := range events {
		if stored.EventType == string(eventType) && stored.Date == date {
			return stored
		}
	}
	t.Fatalf("stored event %s at %s not found", eventType, date)
	return models.HostEvent{}
}

func decodeActivityEvents(t *testing.T, rec *httptest.ResponseRecorder) []models.HostEvent {
	t.Helper()

	var events []models.HostEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("json.Unmarshal events: %v; body: %s", err, rec.Body.String())
	}
	return events
}

func decodeActivityStats(t *testing.T, rec *httptest.ResponseRecorder) models.ActivityStats {
	t.Helper()

	var stats models.ActivityStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("json.Unmarshal stats: %v; body: %s", err, rec.Body.String())
	}
	return stats
}

func decodeActivityDevices(t *testing.T, rec *httptest.ResponseRecorder) []models.ActivityDeviceOption {
	t.Helper()

	var devices []models.ActivityDeviceOption
	if err := json.Unmarshal(rec.Body.Bytes(), &devices); err != nil {
		t.Fatalf("json.Unmarshal devices: %v; body: %s", err, rec.Body.String())
	}
	return devices
}

func getPath(router *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func eventTypeSet(eventTypes []models.HostEventType) map[string]bool {
	result := make(map[string]bool, len(eventTypes))
	for _, eventType := range eventTypes {
		result[string(eventType)] = true
	}
	return result
}

func twoDigit(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func TestParseActivityLimitDefaultsAndBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		query   string
		want    int
		wantErr bool
	}{
		{name: "default", want: defaultActivityLimit},
		{name: "lower bound", query: "?limit=1", want: 1},
		{name: "upper bound", query: "?limit=100", want: 100},
		{name: "zero", query: "?limit=0", wantErr: true},
		{name: "above max", query: "?limit=101", wantErr: true},
		{name: "invalid", query: "?limit=nope", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/activity"+test.query, nil)
			got, err := parseActivityLimit(ctx)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseActivityLimit(%q) returned nil error", test.query)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseActivityLimit(%q): %v", test.query, err)
			}
			if got != test.want {
				t.Fatalf("parseActivityLimit(%q) = %d, want %d", test.query, got, test.want)
			}
		})
	}
}

func TestParseActivityOffsetDefaultsAndBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		query   string
		want    int
		wantErr bool
	}{
		{name: "default", want: defaultActivityOffset},
		{name: "zero", query: "?offset=0", want: 0},
		{name: "positive", query: "?offset=25", want: 25},
		{name: "negative", query: "?offset=-1", wantErr: true},
		{name: "invalid", query: "?offset=nope", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/activity"+test.query, nil)
			got, err := parseActivityOffset(ctx)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseActivityOffset(%q) returned nil error", test.query)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseActivityOffset(%q): %v", test.query, err)
			}
			if got != test.want {
				t.Fatalf("parseActivityOffset(%q) = %d, want %d", test.query, got, test.want)
			}
		})
	}
}

func TestParseActivityCursorValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		query   string
		offset  int
		want    activityCursor
		wantErr bool
	}{
		{name: "missing cursor", want: activityCursor{}},
		{name: "complete cursor", query: "?beforeDate=2026-08-24+10%3A00%3A00&beforeId=7", want: activityCursor{BeforeDate: "2026-08-24 10:00:00", BeforeID: 7}},
		{name: "date only", query: "?beforeDate=2026-08-24+10%3A00%3A00", wantErr: true},
		{name: "id only", query: "?beforeId=7", wantErr: true},
		{name: "invalid date", query: "?beforeDate=not-a-date&beforeId=7", wantErr: true},
		{name: "zero id", query: "?beforeDate=2026-08-24+10%3A00%3A00&beforeId=0", wantErr: true},
		{name: "negative id", query: "?beforeDate=2026-08-24+10%3A00%3A00&beforeId=-1", wantErr: true},
		{name: "offset conflict", query: "?beforeDate=2026-08-24+10%3A00%3A00&beforeId=7", offset: 1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/activity"+test.query, nil)
			got, err := parseActivityCursor(ctx, test.offset)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseActivityCursor(%q) returned nil error", test.query)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseActivityCursor(%q): %v", test.query, err)
			}
			if got != test.want {
				t.Fatalf("parseActivityCursor(%q) = %+v, want %+v", test.query, got, test.want)
			}
		})
	}
}

func TestCombineActivityEventTypes(t *testing.T) {
	all := combineActivityEventTypes

	tests := []struct {
		name          string
		categoryTypes []models.HostEventType
		requestTypes  []models.HostEventType
		want          []models.HostEventType
		wantEmpty     bool
	}{
		{name: "all category no request", want: nil},
		{name: "all category explicit request", requestTypes: []models.HostEventType{models.EventOnline, models.EventKnown}, want: []models.HostEventType{models.EventOnline, models.EventKnown}},
		{name: "specific category no request", categoryTypes: models.ConnectivityEventTypes, want: models.ConnectivityEventTypes},
		{name: "intersection preserves request order", categoryTypes: models.ConnectivityEventTypes, requestTypes: []models.HostEventType{models.EventOffline, models.EventPinnedChanged, models.EventOnline}, want: []models.HostEventType{models.EventOffline, models.EventOnline}},
		{name: "disjoint", categoryTypes: models.ConnectivityEventTypes, requestTypes: []models.HostEventType{models.EventPinnedChanged}, wantEmpty: true},
		{name: "deduplicates", categoryTypes: models.ConnectivityEventTypes, requestTypes: []models.HostEventType{models.EventOnline, models.EventOnline}, want: []models.HostEventType{models.EventOnline}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, empty := all(test.categoryTypes, test.requestTypes)
			if empty != test.wantEmpty {
				t.Fatalf("empty = %v, want %v", empty, test.wantEmpty)
			}
			if strings.Join(hostEventTypesToStrings(got), ",") != strings.Join(hostEventTypesToStrings(test.want), ",") {
				t.Fatalf("types = %+v, want %+v", got, test.want)
			}
		})
	}
}

func hostEventTypesToStrings(types []models.HostEventType) []string {
	values := make([]string, 0, len(types))
	for _, eventType := range types {
		values = append(values, string(eventType))
	}
	return values
}
