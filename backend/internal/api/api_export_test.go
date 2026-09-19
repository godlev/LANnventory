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
	if err := gdb.RecordHostObservation("AA:BB:CC:DD:EE:20", "2026-09-05 09:00:00"); err != nil {
		t.Fatalf("RecordHostObservation NAS: %v", err)
	}
	if err := gdb.RecordHostObservation("AA:BB:CC:DD:EE:01", "2026-09-05 08:00:00"); err != nil {
		t.Fatalf("RecordHostObservation router: %v", err)
	}

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
	profileManufacturer := "TrueNAS"
	profileModel := "Storage Node"
	profileAddress := "nas.local"
	if _, found, err := gdb.UpdateDeviceProfile("AA:BB:CC:DD:EE:20", models.DeviceProfileUpdate{
		Manufacturer:      &profileManufacturer,
		Model:             &profileModel,
		ManagementAddress: &profileAddress,
	}); err != nil || !found {
		t.Fatalf("UpdateDeviceProfile nas found=%v err=%v", found, err)
	}
	networkMode := "managed"
	networkPorts := 5
	if _, found, err := gdb.UpdateNetworkDeviceProfile("AA:BB:CC:DD:EE:01", models.NetworkDeviceProfileUpdate{
		ManagementMode:    &networkMode,
		PhysicalPortCount: &networkPorts,
	}); err != nil || !found {
		t.Fatalf("UpdateNetworkDeviceProfile router found=%v err=%v", found, err)
	}
	systemRole := "Storage"
	systemOS := "TrueNAS SCALE"
	systemVersion := "25.04"
	if _, found, err := gdb.UpdateSystemDeviceProfile("AA:BB:CC:DD:EE:20", models.SystemDeviceProfileUpdate{
		Role:            &systemRole,
		OperatingSystem: &systemOS,
		Version:         &systemVersion,
	}); err != nil || !found {
		t.Fatalf("UpdateSystemDeviceProfile nas found=%v err=%v", found, err)
	}
	hypervisorPlatform := "proxmox-ve"
	hypervisorNode := "pve-test"
	if _, found, err := gdb.UpdateHypervisorProfile("AA:BB:CC:DD:EE:20", models.HypervisorProfileUpdate{
		Platform: &hypervisorPlatform,
		NodeName: &hypervisorNode,
	}); err != nil || !found {
		t.Fatalf("UpdateHypervisorProfile found=%v err=%v", found, err)
	}
	workloadRecord, err := gdb.UpsertInfrastructureWorkload("AA:BB:CC:DD:EE:20", models.InfrastructureWorkloadUpsert{
		NativeID:     "119",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "media-vm",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceManual,
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			Mac:               "AA:BB:CC:00:01:19",
			Bridge:            "vmbr0",
			ConfiguredAddress: "192.168.1.119",
		}},
	}, "2026-09-05T10:30:00Z")
	if err != nil {
		t.Fatalf("UpsertInfrastructureWorkload: %v", err)
	}
	routerHost, err := gdb.SelectHostWithMetadataByID(1)
	if err != nil {
		t.Fatalf("SelectHostWithMetadataByID router: %v", err)
	}
	if _, err := gdb.SetInfrastructureWorkloadHostLink(
		workloadRecord.Workload.ID,
		routerHost,
		models.InfrastructureWorkloadLinkSourceManual,
		"2026-09-05T10:35:00Z",
	); err != nil {
		t.Fatalf("SetInfrastructureWorkloadHostLink: %v", err)
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
	assertExportLifecycleMACs(t, document.Data.HostLifecycle, []string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:20"})
	assertExportDeviceProfileMACs(t, document.Data.DeviceProfiles, []string{"AA:BB:CC:DD:EE:20"})
	if len(document.Data.NetworkDeviceProfiles) != 1 || document.Data.NetworkDeviceProfiles[0].Mac != "AA:BB:CC:DD:EE:01" ||
		document.Data.NetworkDeviceProfiles[0].ManagementMode != networkMode || document.Data.NetworkDeviceProfiles[0].PhysicalPortCount != networkPorts {
		t.Fatalf("network device profiles = %+v", document.Data.NetworkDeviceProfiles)
	}
	if len(document.Data.SystemDeviceProfiles) != 1 || document.Data.SystemDeviceProfiles[0].Mac != "AA:BB:CC:DD:EE:20" ||
		document.Data.SystemDeviceProfiles[0].Role != systemRole || document.Data.SystemDeviceProfiles[0].OperatingSystem != systemOS {
		t.Fatalf("system device profiles = %+v", document.Data.SystemDeviceProfiles)
	}
	if len(document.Data.HypervisorProfiles) != 1 || document.Data.HypervisorProfiles[0].Platform != hypervisorPlatform ||
		document.Data.HypervisorProfiles[0].NodeName != hypervisorNode {
		t.Fatalf("hypervisor profiles = %+v", document.Data.HypervisorProfiles)
	}
	if len(document.Data.InfrastructureWorkloads) != 1 ||
		document.Data.InfrastructureWorkloads[0].NativeID != "119" ||
		document.Data.InfrastructureWorkloads[0].WorkloadType != "vm" ||
		document.Data.InfrastructureWorkloads[0].Source != "manual" {
		t.Fatalf("infrastructure workloads = %+v", document.Data.InfrastructureWorkloads)
	}
	if len(document.Data.InfrastructureWorkloadInterfaces) != 1 ||
		document.Data.InfrastructureWorkloadInterfaces[0].WorkloadID != workloadRecord.Workload.ID ||
		document.Data.InfrastructureWorkloadInterfaces[0].Mac != "AA:BB:CC:00:01:19" {
		t.Fatalf("infrastructure workload interfaces = %+v", document.Data.InfrastructureWorkloadInterfaces)
	}
	if len(document.Data.InfrastructureWorkloadHostLinks) != 1 ||
		document.Data.InfrastructureWorkloadHostLinks[0].WorkloadID != workloadRecord.Workload.ID ||
		document.Data.InfrastructureWorkloadHostLinks[0].HostID != routerHost.ID ||
		document.Data.InfrastructureWorkloadHostLinks[0].LinkSource != models.InfrastructureWorkloadLinkSourceManual {
		t.Fatalf("infrastructure workload host links = %+v", document.Data.InfrastructureWorkloadHostLinks)
	}
	if document.Data.DeviceProfiles[0].Manufacturer != profileManufacturer ||
		document.Data.DeviceProfiles[0].Model != profileModel ||
		document.Data.DeviceProfiles[0].ManagementAddress != profileAddress ||
		document.Data.DeviceProfiles[0].UpdatedAt == "" {
		t.Fatalf("device profile backup = %+v, want managed profile values", document.Data.DeviceProfiles[0])
	}
	assertStringSlice(t, document.Data.HostMetadata[0].Tags, routerTags, "router metadata tags")
	assertStringSlice(t, document.Data.HostMetadata[1].Tags, nasTags, "nas metadata tags")
	if !document.Data.HostMetadata[0].Pinned {
		t.Fatalf("router metadata pinned = false, want true")
	}
	if document.Data.Events[1].Date != "2026-09-05 10:00:00" {
		t.Fatalf("event Date = %q, want preserved stored Date", document.Data.Events[1].Date)
	}
	if document.Data.HostLifecycle[0].FirstSeen != "2026-09-05 08:00:00" || document.Data.HostLifecycle[0].LastSeen != "2026-09-05 08:00:00" || document.Data.HostLifecycle[0].FirstSeenEstimated {
		t.Fatalf("router lifecycle backup = %+v, want exact portable lifecycle", document.Data.HostLifecycle[0])
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
		!strings.Contains(rec.Body.String(), `"hostMetadata": []`) ||
		!strings.Contains(rec.Body.String(), `"hostLifecycle": []`) ||
		!strings.Contains(rec.Body.String(), `"deviceProfiles": []`) ||
		!strings.Contains(rec.Body.String(), `"networkDeviceProfiles": []`) ||
		!strings.Contains(rec.Body.String(), `"systemDeviceProfiles": []`) ||
		!strings.Contains(rec.Body.String(), `"hypervisorProfiles": []`) ||
		!strings.Contains(rec.Body.String(), `"infrastructureWorkloads": []`) ||
		!strings.Contains(rec.Body.String(), `"infrastructureWorkloadInterfaces": []`) ||
		!strings.Contains(rec.Body.String(), `"infrastructureWorkloadHostLinks": []`) {
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
	if err := gdb.RecordHostObservation("AA:BB:CC:DD:EE:20", "2026-09-04 08:00:00"); err != nil {
		t.Fatalf("RecordHostObservation create: %v", err)
	}
	if err := gdb.RecordHostObservation("AA:BB:CC:DD:EE:20", "2026-09-05 11:00:00"); err != nil {
		t.Fatalf("RecordHostObservation update: %v", err)
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
		"2026-09-04 08:00:00",
		"false",
		"2026-09-05 11:00:00",
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
	assertStringSlice(t, rows[1][11:], []string{"", "", "", "", "false", "", "false", ""}, "empty metadata and lifecycle CSV columns")
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

func assertExportDeviceProfileMACs(t *testing.T, profiles []backup.DeviceProfile, want []string) {
	t.Helper()

	if len(profiles) != len(want) {
		t.Fatalf("device profiles len = %d, want %d: %+v", len(profiles), len(want), profiles)
	}
	for i, mac := range want {
		if profiles[i].Mac != mac {
			t.Fatalf("deviceProfiles[%d].Mac = %q, want %q: %+v", i, profiles[i].Mac, mac, profiles)
		}
	}
}

func assertExportLifecycleMACs(t *testing.T, lifecycles []backup.HostLifecycle, want []string) {
	t.Helper()

	if len(lifecycles) != len(want) {
		t.Fatalf("lifecycle len = %d, want %d: %+v", len(lifecycles), len(want), lifecycles)
	}
	for i, mac := range want {
		if lifecycles[i].Mac != mac {
			t.Fatalf("lifecycle[%d].Mac = %q, want %q: %+v", i, lifecycles[i].Mac, mac, lifecycles)
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
