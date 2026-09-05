package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/backup"
	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/version"
)

func TestBackupExportEndpointIncludesStableDataAndMetadata(t *testing.T) {
	router := setupTestRouter(t)
	oldVersion := version.Version
	version.Version = "9.8.7-test"
	t.Cleanup(func() {
		version.Version = oldVersion
	})

	config := conf.GetAppConfig()
	config.ShoutURL = "discord://notification-secret@example"
	config.PGConnect = "postgres://wyl:pg-secret@localhost/wyl"
	config.InfluxToken = "influx-secret"
	conf.SetAppConfigForTest(config)

	seedExportHost(t, "now", models.Host{
		ID:         2,
		Name:       "NAS",
		DNS:        "nas.local",
		Iface:      "eth0",
		IP:         "192.168.1.20",
		Mac:        "AA:BB:CC:DD:EE:20",
		Hw:         "Storage Vendor",
		Date:       "2026-09-05 09:00:00",
		Known:      1,
		Now:        1,
		DeviceType: "nas",
	})
	seedExportHost(t, "now", models.Host{
		ID:    1,
		Name:  "router",
		Iface: "eth0",
		IP:    "192.168.1.1",
		Mac:   "AA:BB:CC:DD:EE:01",
		Date:  "2026-09-05 08:00:00",
		Known: 1,
		Now:   1,
	})
	seedExportHost(t, "history", models.Host{
		ID:    20,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Date:  "2026-09-05 07:30:00",
		Known: 1,
		Now:   0,
	})
	seedExportHost(t, "history", models.Host{
		ID:    10,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Date:  "2026-09-05 07:00:00",
		Known: 1,
		Now:   1,
	})
	seedExportEvent(t, models.HostEvent{
		ID:         2,
		HostID:     2,
		Mac:        "AA:BB:CC:DD:EE:20",
		Name:       "NAS",
		EventType:  string(models.EventDeviceTypeChanged),
		Date:       "2026-09-05 10:00:00",
		DateUTC:    "2030-01-01T00:00:00Z",
		IP:         "192.168.1.20",
		Iface:      "eth0",
		DeviceType: "nas",
		OldValue:   "",
		NewValue:   "nas",
	})
	seedExportEvent(t, models.HostEvent{
		ID:        1,
		HostID:    1,
		Mac:       "AA:BB:CC:DD:EE:01",
		Name:      "router",
		EventType: string(models.EventDiscovered),
		Date:      "2026-09-05 09:30:00",
	})

	rec := getPath(router, "/api/export/backup")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertHeaderContains(t, rec.Header().Get("Content-Type"), "application/json", "backup content type")
	assertDownloadFilename(t, rec.Header().Get("Content-Disposition"), `^attachment; filename="lannventory-backup-\d{8}T\d{6}Z\.json"$`)

	body := rec.Body.String()
	for _, secret := range []string{"notification-secret", "pg-secret", "influx-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("backup leaked secret %q: %s", secret, body)
		}
	}
	if strings.Contains(body, "DateUTC") || strings.Contains(body, "2030-01-01T00:00:00Z") {
		t.Fatalf("backup included DateUTC display data: %s", body)
	}
	if strings.Contains(body, "TagsJSON") || strings.Contains(body, "TAGS_JSON") {
		t.Fatalf("backup exposed internal tag storage: %s", body)
	}

	routerTags := []string{"gateway", "critical"}
	nasTags := []string{"storage", "important"}
	routerPinned := true
	if _, err := gdb.UpsertHostMetadata("AA:BB:CC:DD:EE:20", models.HostMetadataUpdate{
		Tags: &nasTags,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata nas: %v", err)
	}
	if _, err := gdb.UpsertHostMetadata("AA:BB:CC:DD:EE:01", models.HostMetadataUpdate{
		Tags:   &routerTags,
		Pinned: &routerPinned,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata router: %v", err)
	}

	rec = getPath(router, "/api/export/backup")
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata backup status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	body = rec.Body.String()
	if strings.Contains(body, "DateUTC") || strings.Contains(body, "2030-01-01T00:00:00Z") {
		t.Fatalf("metadata backup included DateUTC display data: %s", body)
	}
	if strings.Contains(body, "TagsJSON") || strings.Contains(body, "TAGS_JSON") {
		t.Fatalf("metadata backup exposed internal tag storage: %s", body)
	}

	var document backup.Document
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if document.Format != backup.Format {
		t.Fatalf("Format = %q, want %q", document.Format, backup.Format)
	}
	if document.FormatVersion != backup.FormatVersion {
		t.Fatalf("FormatVersion = %d, want %d", document.FormatVersion, backup.FormatVersion)
	}
	createdAt, err := time.Parse(time.RFC3339, document.CreatedAt)
	if err != nil {
		t.Fatalf("CreatedAt is not RFC3339 UTC: %q", document.CreatedAt)
	}
	if createdAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC", createdAt.Location())
	}
	if document.AppVersion != "9.8.7-test" {
		t.Fatalf("AppVersion = %q, want test version", document.AppVersion)
	}

	assertExportHostIDs(t, document.Data.CurrentHosts, []int{1, 2}, "current hosts")
	assertExportHostIDs(t, document.Data.History, []int{10, 20}, "history")
	assertExportEventIDs(t, document.Data.Events, []int{1, 2}, "events")
	assertExportMetadataMACs(t, document.Data.HostMetadata, []string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:20"})
	assertStringSlice(t, document.Data.HostMetadata[0].Tags, routerTags, "router metadata tags")
	assertStringSlice(t, document.Data.HostMetadata[1].Tags, nasTags, "nas metadata tags")
	if !document.Data.HostMetadata[0].Pinned {
		t.Fatalf("router metadata pinned = false, want true")
	}
	if document.Data.Events[1].Date != "2026-09-05 10:00:00" {
		t.Fatalf("event Date = %q, want preserved stored Date", document.Data.Events[1].Date)
	}
}

func TestBackupExportEndpointEmptyTablesUsesArrays(t *testing.T) {
	router := setupTestRouter(t)

	rec := getPath(router, "/api/export/backup")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"currentHosts": []`) ||
		!strings.Contains(rec.Body.String(), `"history": []`) ||
		!strings.Contains(rec.Body.String(), `"events": []`) ||
		!strings.Contains(rec.Body.String(), `"hostMetadata": []`) {
		t.Fatalf("empty backup did not encode empty arrays: %s", rec.Body.String())
	}
}

func TestInventoryCSVExportEndpointEscapesCurrentInventory(t *testing.T) {
	router := setupTestRouter(t)
	seedExportHost(t, "now", models.Host{
		ID:         7,
		Name:       "NAS, \"primary\"\nline",
		DNS:        "nas.local",
		Iface:      "eth0",
		IP:         "192.168.1.20",
		Mac:        "AA:BB:CC:DD:EE:20",
		Hw:         "Vendor, Inc.",
		Date:       "2026-09-05 11:00:00",
		Known:      1,
		Now:        1,
		DeviceType: "nas",
	})
	owner := "Storage Team"
	location := "Rack 1"
	notes := "Primary backup target"
	tags := []string{"storage", "important"}
	pinned := true
	if _, err := gdb.UpsertHostMetadata("AA:BB:CC:DD:EE:20", models.HostMetadataUpdate{
		Owner:    &owner,
		Location: &location,
		Notes:    &notes,
		Tags:     &tags,
		Pinned:   &pinned,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}

	rec := getPath(router, "/api/export/inventory.csv")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertHeaderContains(t, rec.Header().Get("Content-Type"), "text/csv", "inventory CSV content type")
	assertDownloadFilename(t, rec.Header().Get("Content-Disposition"), `^attachment; filename="lannventory-inventory-\d{8}T\d{6}Z\.csv"$`)

	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v\n%s", err, rec.Body.String())
	}
	if len(rows) != 2 {
		t.Fatalf("CSV rows len = %d, want 2: %#v", len(rows), rows)
	}
	assertStringSlice(t, rows[0], backup.InventoryCSVHeader, "CSV header")
	assertStringSlice(t, rows[1], []string{
		"7",
		"NAS, \"primary\"\nline",
		"nas.local",
		"eth0",
		"192.168.1.20",
		"AA:BB:CC:DD:EE:20",
		"Vendor, Inc.",
		"2026-09-05 11:00:00",
		"1",
		"1",
		"nas",
		"Storage Team",
		"Rack 1",
		"Primary backup target",
		"storage; important",
		"true",
	}, "CSV row")
}

func TestInventoryCSVExportWithoutMetadataReturnsEmptyMetadataColumns(t *testing.T) {
	router := setupTestRouter(t)
	seedExportHost(t, "now", models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Known: 1,
		Now:   1,
	})

	rec := getPath(router, "/api/export/inventory.csv")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("CSV rows len = %d, want 2: %#v", len(rows), rows)
	}
	assertStringSlice(t, rows[1][11:], []string{"", "", "", "", "false"}, "empty metadata CSV columns")
}

func TestInventoryCSVExportEndpointEmptyInventoryReturnsHeader(t *testing.T) {
	router := setupTestRouter(t)

	rec := getPath(router, "/api/export/inventory.csv")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("CSV rows len = %d, want header only: %#v", len(rows), rows)
	}
	assertStringSlice(t, rows[0], backup.InventoryCSVHeader, "empty CSV header")
}

func TestExportEndpointsReportDatabaseFailure(t *testing.T) {
	router := setupTestRouter(t)
	if err := gdb.Close(); err != nil {
		t.Fatalf("gdb.Close: %v", err)
	}

	for _, path := range []string{"/api/export/backup", "/api/export/inventory.csv"} {
		t.Run(path, func(t *testing.T) {
			rec := getPath(router, path)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
			}
			if rec.Header().Get("Content-Disposition") != "" {
				t.Fatalf("failed export unexpectedly set attachment header: %s", rec.Header().Get("Content-Disposition"))
			}
		})
	}
}

func seedExportHost(t *testing.T, table string, host models.Host) {
	t.Helper()

	if err := gdb.UpdateWithError(table, host); err != nil {
		t.Fatalf("UpdateWithError %s: %v", table, err)
	}
}

func seedExportEvent(t *testing.T, event models.HostEvent) {
	t.Helper()

	if err := gdb.AddEvent(event); err != nil {
		t.Fatalf("AddEvent: %v", err)
	}
}

func assertHeaderContains(t *testing.T, got, want, label string) {
	t.Helper()

	if !strings.Contains(got, want) {
		t.Fatalf("%s = %q, want to contain %q", label, got, want)
	}
}

func assertDownloadFilename(t *testing.T, got, pattern string) {
	t.Helper()

	if !regexp.MustCompile(pattern).MatchString(got) {
		t.Fatalf("Content-Disposition = %q, want pattern %s", got, pattern)
	}
}

func assertExportHostIDs(t *testing.T, hosts []backup.Host, want []int, label string) {
	t.Helper()

	if len(hosts) != len(want) {
		t.Fatalf("%s len = %d, want %d: %+v", label, len(hosts), len(want), hosts)
	}
	for i, id := range want {
		if hosts[i].ID != id {
			t.Fatalf("%s[%d].ID = %d, want %d: %+v", label, i, hosts[i].ID, id, hosts)
		}
	}
}

func assertExportEventIDs(t *testing.T, events []backup.Event, want []int, label string) {
	t.Helper()

	if len(events) != len(want) {
		t.Fatalf("%s len = %d, want %d: %+v", label, len(events), len(want), events)
	}
	for i, id := range want {
		if events[i].ID != id {
			t.Fatalf("%s[%d].ID = %d, want %d: %+v", label, i, events[i].ID, id, events)
		}
	}
}

func assertExportMetadataMACs(t *testing.T, metadata []backup.HostMetadata, want []string) {
	t.Helper()

	if len(metadata) != len(want) {
		t.Fatalf("metadata len = %d, want %d: %+v", len(metadata), len(want), metadata)
	}
	for i, mac := range want {
		if metadata[i].Mac != mac {
			t.Fatalf("metadata[%d].Mac = %q, want %q: %+v", i, metadata[i].Mac, mac, metadata)
		}
	}
}

func assertStringSlice(t *testing.T, got, want []string, label string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s len = %d, want %d: got %#v want %#v", label, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q; got %#v", label, i, got[i], want[i], got)
		}
	}
}
