package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func setupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	oldConfig := conf.AppConfig
	conf.AppConfig.UseDB = "sqlite"
	conf.AppConfig.DBPath = filepath.Join(t.TempDir(), "watchyourlan-test.db")
	gdb.Start()

	t.Cleanup(func() {
		if err := gdb.Close(); err != nil {
			t.Errorf("gdb.Close: %v", err)
		}
		conf.AppConfig = oldConfig
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	Routes(router)

	return router
}

func TestHostEndpointsRejectInvalidID(t *testing.T) {
	router := setupTestRouter(t)

	tests := []string{
		"/api/host/not-a-number",
		"/api/host/del/not-a-number",
		"/api/edit/not-a-number/name/toggle",
		"/api/host/0",
		"/api/host/-1",
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

func TestHostEndpointsRejectMissingHostID(t *testing.T) {
	router := setupTestRouter(t)

	tests := []string{
		"/api/host/999",
		"/api/host/del/999",
		"/api/edit/999/name/toggle",
		"/api/host/999/activity",
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

	hosts, ok := gdb.Select("now")
	if !ok {
		t.Fatal("Select now failed")
	}
	if len(hosts) != 0 {
		t.Fatalf("missing-host operations changed hosts: %+v", hosts)
	}
}

func TestSetHostDeviceTypeAcceptsValidTypeAndPreservesFields(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "gateway",
		Iface: "eth0",
		IP:    "192.168.1.1",
		Mac:   "AA:BB:CC:DD:EE:01",
		Hw:    "Gateway Vendor",
		Date:  "2026-08-24 09:00:00",
		Known: 1,
		Now:   1,
	})

	rec := patchHostDeviceType(router, host.ID, `{"deviceType":"router"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var updated models.Host
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if updated.DeviceType != "router" {
		t.Fatalf("DeviceType = %q, want router", updated.DeviceType)
	}
	if updated.Name != host.Name || updated.IP != host.IP || updated.Mac != host.Mac || updated.Known != host.Known || updated.Now != host.Now {
		t.Fatalf("unrelated fields changed: got %+v, original %+v", updated, host)
	}

	reread := gdb.SelectByID(host.ID)
	if reread.DeviceType != "router" {
		t.Fatalf("reread DeviceType = %q, want router", reread.DeviceType)
	}
	if reread.Hw != host.Hw || reread.Iface != host.Iface || reread.Date != host.Date {
		t.Fatalf("reread unrelated fields changed: got %+v, original %+v", reread, host)
	}
}

func TestSetHostDeviceTypeAcceptsNASAndClearing(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "storage",
		IP:         "192.168.1.20",
		Mac:        "AA:BB:CC:DD:EE:20",
		Hw:         "Storage Vendor",
		Known:      1,
		Now:        1,
		DeviceType: "router",
	})

	rec := patchHostDeviceType(router, host.ID, `{"deviceType":"nas"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := gdb.SelectByID(host.ID).DeviceType; got != "nas" {
		t.Fatalf("DeviceType after NAS update = %q, want nas", got)
	}

	rec = patchHostDeviceType(router, host.ID, `{"deviceType":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := gdb.SelectByID(host.ID).DeviceType; got != "" {
		t.Fatalf("DeviceType after clearing = %q, want empty string", got)
	}
}

func TestSetHostDeviceTypeRejectsInvalidInput(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "desktop",
		IP:         "192.168.1.42",
		Mac:        "AA:BB:CC:DD:EE:42",
		DeviceType: "desktop",
	})

	tests := []struct {
		name string
		body string
	}{
		{name: "unknown type", body: `{"deviceType":"spaceship"}`},
		{name: "missing field", body: `{}`},
		{name: "null field", body: `{"deviceType":null}`},
		{name: "non-string field", body: `{"deviceType":42}`},
		{name: "malformed json", body: `not-json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := patchHostDeviceType(router, host.ID, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if got := gdb.SelectByID(host.ID).DeviceType; got != "desktop" {
				t.Fatalf("DeviceType changed after rejected input: got %q, want desktop", got)
			}
		})
	}
}

func TestSetHostDeviceTypeRejectsInvalidID(t *testing.T) {
	router := setupTestRouter(t)
	body := bytes.NewBufferString(`{"deviceType":"router"}`)
	tests := []string{
		"/api/host/not-a-number/type",
		"/api/host/0/type",
		"/api/host/-1/type",
		"/api/host/999/type",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body.Bytes()))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func TestHostJSONIncludesDeviceType(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "console",
		IP:         "127.0.0.1",
		Mac:        "AA:BB:CC:DD:EE:99",
		DeviceType: "game-console",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/all", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/all status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var allHosts []models.Host
	if err := json.Unmarshal(rec.Body.Bytes(), &allHosts); err != nil {
		t.Fatalf("json.Unmarshal /api/all: %v", err)
	}
	if len(allHosts) != 1 || allHosts[0].DeviceType != "game-console" {
		t.Fatalf("/api/all DeviceType = %+v, want game-console", allHosts)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/host/"+itoa(host.ID), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/host status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got models.Host
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal /api/host: %v", err)
	}
	if got.DeviceType != "game-console" {
		t.Fatalf("/api/host DeviceType = %q, want game-console", got.DeviceType)
	}
}

func TestHostEndpointsIncludeMetadata(t *testing.T) {
	router := setupTestRouter(t)
	routerHost := seedHost(t, models.Host{
		Name:       "router",
		IP:         "192.168.1.1",
		Mac:        "AA:BB:CC:DD:EE:01",
		DeviceType: "router",
	})
	nasHost := seedHost(t, models.Host{
		Name:       "NAS",
		IP:         "192.168.1.20",
		Mac:        "AA:BB:CC:DD:EE:20",
		DeviceType: "nas",
	})
	owner := "Network Team"
	location := "Closet"
	notes := "Main gateway"
	tags := []string{"critical", "edge"}
	pinned := true
	if _, err := gdb.UpsertHostMetadata(routerHost.Mac, models.HostMetadataUpdate{
		Owner:    &owner,
		Location: &location,
		Notes:    &notes,
		Tags:     &tags,
		Pinned:   &pinned,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}

	rec := getPath(router, "/api/all")
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/all status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var hosts []models.Host
	if err := json.Unmarshal(rec.Body.Bytes(), &hosts); err != nil {
		t.Fatalf("json.Unmarshal /api/all: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("/api/all hosts len = %d, want 2: %+v", len(hosts), hosts)
	}
	for _, host := range hosts {
		switch host.ID {
		case routerHost.ID:
			if host.Owner != owner || host.Location != location || host.Notes != notes || !host.Pinned {
				t.Fatalf("router metadata = %+v, want saved metadata", host)
			}
			assertStringSlice(t, host.Tags, tags, "router tags")
		case nasHost.ID:
			if host.Owner != "" || host.Location != "" || host.Notes != "" || host.Pinned {
				t.Fatalf("default host metadata = %+v, want empty and unpinned", host)
			}
			if host.Tags == nil || len(host.Tags) != 0 {
				t.Fatalf("default host Tags = %#v, want empty array", host.Tags)
			}
		}
	}

	rec = getPath(router, "/api/host/"+itoa(routerHost.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/host status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var host models.Host
	if err := json.Unmarshal(rec.Body.Bytes(), &host); err != nil {
		t.Fatalf("json.Unmarshal /api/host: %v", err)
	}
	if host.Owner != owner || !host.Pinned {
		t.Fatalf("/api/host metadata = %+v, want saved metadata", host)
	}
}

func TestSetHostMetadataCanonicalizesAndPartiallyUpdates(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name: "proxmox",
		IP:   "192.168.1.30",
		Mac:  "AA:BB:CC:DD:EE:30",
	})

	rec := patchHostMetadata(router, host.ID, `{
		"owner": "  Miroslav  ",
		"location": "  Office  ",
		"notes": "Main Proxmox host\nUPS attached",
		"tags": [" server ", "Critical", "SERVER", "", "critical"],
		"pinned": true
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata patch status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var updated models.Host
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal metadata response: %v", err)
	}
	if updated.Owner != "Miroslav" || updated.Location != "Office" || updated.Notes != "Main Proxmox host\nUPS attached" || !updated.Pinned {
		t.Fatalf("metadata response = %+v, want trimmed text and pinned", updated)
	}
	assertStringSlice(t, updated.Tags, []string{"server", "Critical"}, "canonical tags")

	rec = patchHostMetadata(router, host.ID, `{"pinned": false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("partial metadata patch status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal partial metadata response: %v", err)
	}
	if updated.Owner != "Miroslav" || updated.Location != "Office" || updated.Notes != "Main Proxmox host\nUPS attached" || updated.Pinned {
		t.Fatalf("partial patch metadata = %+v, want original text and unpinned", updated)
	}
	assertStringSlice(t, updated.Tags, []string{"server", "Critical"}, "partial patch tags")

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 0 {
		t.Fatalf("metadata patch created activity events: %+v", events)
	}
}

func TestSetHostMetadataRejectsInvalidInput(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name: "desktop",
		IP:   "192.168.1.42",
		Mac:  "AA:BB:CC:DD:EE:42",
	})

	tests := []struct {
		name string
		body string
	}{
		{name: "owner too long", body: metadataJSON(t, map[string]any{"owner": strings.Repeat("a", 121)})},
		{name: "location too long", body: metadataJSON(t, map[string]any{"location": strings.Repeat("b", 121)})},
		{name: "notes too long", body: metadataJSON(t, map[string]any{"notes": strings.Repeat("c", 4001)})},
		{name: "too many tags", body: metadataJSON(t, map[string]any{"tags": numberedTags(21, 3)})},
		{name: "tag too long", body: metadataJSON(t, map[string]any{"tags": []string{strings.Repeat("d", 49)}})},
		{name: "control character", body: metadataJSON(t, map[string]any{"owner": "bad\u0001value"})},
		{name: "malformed json", body: `not-json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := patchHostMetadata(router, host.ID, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/host/not-a-number/metadata", bytes.NewBufferString(`{"pinned":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHostDeleteRemovesMetadataButOrdinaryHostUpdatesPreserveIt(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "camera",
		IP:         "192.168.1.60",
		Mac:        "AA:BB:CC:DD:EE:60",
		Known:      1,
		Now:        1,
		DeviceType: "camera",
	})
	owner := "Facilities"
	tags := []string{"security"}
	pinned := true
	if _, err := gdb.UpsertHostMetadata(host.Mac, models.HostMetadataUpdate{
		Owner:  &owner,
		Tags:   &tags,
		Pinned: &pinned,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}

	rec := getPath(router, "/api/edit/"+itoa(host.ID)+"/camera-renamed/toggle")
	if rec.Code != http.StatusOK {
		t.Fatalf("edit status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertMetadataStillExists(t, host.Mac, owner)

	rec = patchHostDeviceType(router, host.ID, `{"deviceType":"iot"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("device type status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertMetadataStillExists(t, host.Mac, owner)

	rec = getPath(router, "/api/host/del/"+itoa(host.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	_, ok, err := gdb.SelectHostMetadataByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC after delete: %v", err)
	}
	if ok {
		t.Fatal("host metadata still exists after explicit host delete")
	}
}

func TestEditHostKnownToggleCreatesEventsOnlyOnKnownChange(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "camera",
		IP:    "192.168.1.60",
		Mac:   "AA:BB:CC:DD:EE:60",
		Known: 0,
		Now:   1,
	})

	rec := getPath(router, "/api/edit/"+itoa(host.ID)+"/camera/toggle")
	if rec.Code != http.StatusOK {
		t.Fatalf("known toggle status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertActivityEvents(t, []models.HostEventType{models.EventKnown})

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if events[0].HostID != host.ID || events[0].Mac != host.Mac || events[0].Name != host.Name {
		t.Fatalf("event snapshot = %+v, want HostID/Mac/Name from host %+v", events[0], host)
	}

	rec = getPath(router, "/api/edit/"+itoa(host.ID)+"/camera-renamed/")
	if rec.Code != http.StatusOK {
		t.Fatalf("name edit status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertActivityEvents(t, []models.HostEventType{models.EventKnown})

	rec = getPath(router, "/api/edit/"+itoa(host.ID)+"/camera-renamed/toggle")
	if rec.Code != http.StatusOK {
		t.Fatalf("unknown toggle status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertActivityEvents(t, []models.HostEventType{models.EventUnknown, models.EventKnown})
}

func TestSetHostDeviceTypeCreatesEventsOnlyOnRealChange(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "router",
		IP:         "192.168.1.1",
		Mac:        "AA:BB:CC:DD:EE:01",
		Known:      1,
		Now:        1,
		DeviceType: "",
	})

	rec := patchHostDeviceType(router, host.ID, `{"deviceType":"router"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("router update status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertActivityEvents(t, []models.HostEventType{models.EventDeviceTypeChanged})

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if events[0].OldValue != "" || events[0].NewValue != "router" || events[0].DeviceType != "router" {
		t.Fatalf("device type event = %+v, want old empty/new router/current router", events[0])
	}

	rec = patchHostDeviceType(router, host.ID, `{"deviceType":"router"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-value update status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertActivityEvents(t, []models.HostEventType{models.EventDeviceTypeChanged})

	rec = patchHostDeviceType(router, host.ID, `{"deviceType":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear update status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertActivityEvents(t, []models.HostEventType{models.EventDeviceTypeChanged, models.EventDeviceTypeChanged})

	events, ok = gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if events[0].OldValue != "router" || events[0].NewValue != "" || events[0].DeviceType != "" {
		t.Fatalf("clear event = %+v, want old router/new empty/current empty", events[0])
	}
}

func TestDeleteHostRemovesDeviceChangeEventsButKeepsConnectivityEvents(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:       "printer",
		IP:         "192.168.1.55",
		Mac:        "AA:BB:CC:DD:EE:55",
		Known:      1,
		Now:        0,
		DeviceType: "printer",
	})

	seededEvents := []models.HostEventType{
		models.EventDiscovered,
		models.EventKnown,
		models.EventUnknown,
		models.EventDeviceTypeChanged,
		models.EventOnline,
		models.EventOffline,
	}
	for i, eventType := range seededEvents {
		event := models.NewHostEvent(host, eventType, "", "")
		event.Date = "2026-08-24 10:0" + strconv.Itoa(i) + ":00"
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent %s: %v", eventType, err)
		}
	}

	rec := getPath(router, "/api/host/del/"+itoa(host.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 2 {
		t.Fatalf("remaining events len = %d, want 2 connectivity events: %+v", len(events), events)
	}
	for _, event := range events {
		if event.EventType != string(models.EventOnline) && event.EventType != string(models.EventOffline) {
			t.Fatalf("remaining non-connectivity event after host delete: %+v", event)
		}
		if event.HostID != host.ID || event.Mac != host.Mac {
			t.Fatalf("remaining connectivity event lost host snapshot: %+v, want host %+v", event, host)
		}
	}
}

func seedHost(t *testing.T, host models.Host) models.Host {
	t.Helper()

	gdb.Update("now", host)
	hosts := gdb.SelectByMAC("now", host.Mac)
	if len(hosts) != 1 {
		t.Fatalf("seeded hosts len = %d, want 1", len(hosts))
	}
	return hosts[0]
}

func patchHostDeviceType(router *gin.Engine, id int, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/host/"+itoa(id)+"/type", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func patchHostMetadata(router *gin.Engine, id int, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, "/api/host/"+itoa(id)+"/metadata", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func getPath(router *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertActivityEvents(t *testing.T, want []models.HostEventType) {
	t.Helper()

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != len(want) {
		t.Fatalf("events len = %d, want %d: %+v", len(events), len(want), events)
	}
	for i, eventType := range want {
		if events[i].EventType != string(eventType) {
			t.Fatalf("events[%d].EventType = %q, want %q; events: %+v", i, events[i].EventType, eventType, events)
		}
	}
}

func itoa(id int) string {
	return strconv.Itoa(id)
}

func metadataJSON(t *testing.T, value map[string]any) string {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal metadata payload: %v", err)
	}

	return string(payload)
}

func numberedTags(count int, length int) []string {
	tags := make([]string, 0, count)
	for i := 0; i < count; i++ {
		tags = append(tags, strings.Repeat(string(rune('a'+i%26)), length)+itoa(i))
	}

	return tags
}

func assertMetadataStillExists(t *testing.T, mac string, owner string) {
	t.Helper()

	metadata, ok, err := gdb.SelectHostMetadataByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC: %v", err)
	}
	if !ok || metadata.Owner != owner {
		t.Fatalf("metadata for %s = %+v, ok=%v, want owner %q", mac, metadata, ok, owner)
	}
}
