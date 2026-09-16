package routines

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/arp"
	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func setupScanRoutineTest(t *testing.T) {
	t.Helper()

	oldConfig := conf.AppConfig
	conf.AppConfig.UseDB = "sqlite"
	conf.AppConfig.DBPath = filepath.Join(t.TempDir(), "scan-routine-test.db")
	conf.AppConfig.InfluxEnable = false
	conf.AppConfig.PrometheusEnable = false
	conf.AppConfig.ShoutURL = ""
	gdb.Start()

	t.Cleanup(func() {
		if err := gdb.Close(); err != nil {
			t.Errorf("gdb.Close: %v", err)
		}
		conf.AppConfig = oldConfig
	})
}

func TestStartScanPublishesHealthyStateAndExactNextSchedule(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	oldNow := scannerNow
	oldScanNetwork := scanNetwork
	oldProcess := processScanResultFunc
	oldWait := waitForNextScan
	oldState := GetScannerState()

	started := time.Date(2026, 9, 16, 9, 30, 0, 0, time.UTC)
	completed := started.Add(2800 * time.Millisecond)
	nowCalls := 0
	var scheduled time.Time

	conf.SetAppConfigForTest(models.Conf{
		Ifaces:   "eth0",
		Timeout:  37,
		LogLevel: "info",
	})
	setScannerStateForTest(ScannerState{})
	scannerNow = func() time.Time {
		nowCalls++
		if nowCalls == 1 {
			return started
		}
		return completed
	}
	scanNetwork = func(context.Context, string, string, []string) arp.ScanResult {
		state := GetScannerState()
		if state.Status != ScannerStatusScanning {
			t.Fatalf("Status during scan = %q, want %q", state.Status, ScannerStatusScanning)
		}
		return arp.ScanResult{
			Success:    true,
			Interfaces: []string{"eth0"},
			Hosts: []models.Host{
				{Mac: "AA:BB:CC:DD:EE:01"},
				{Mac: "AA:BB:CC:DD:EE:02"},
			},
		}
	}
	processScanResultFunc = func([]models.Host, bool) bool {
		return true
	}
	waitForNextScan = func(_ context.Context, next time.Time) bool {
		scheduled = next
		return false
	}

	t.Cleanup(func() {
		conf.SetAppConfigForTest(oldConfig)
		scannerNow = oldNow
		scanNetwork = oldScanNetwork
		processScanResultFunc = oldProcess
		waitForNextScan = oldWait
		setScannerStateForTest(oldState)
	})

	startScan(context.Background())

	state := GetScannerState()
	wantNext := completed.Add(37 * time.Second)
	if state.Status != ScannerStatusHealthy {
		t.Fatalf("Status = %q, want %q", state.Status, ScannerStatusHealthy)
	}
	if !state.LastScanStartedAt.Equal(started) {
		t.Fatalf("LastScanStartedAt = %v, want %v", state.LastScanStartedAt, started)
	}
	if !state.LastScanAt.Equal(completed) || !state.LastSuccessfulScanAt.Equal(completed) {
		t.Fatalf("successful scan timestamps = %+v", state)
	}
	if state.Duration != 2800*time.Millisecond {
		t.Fatalf("Duration = %s, want 2.8s", state.Duration)
	}
	if state.DevicesFound != 2 {
		t.Fatalf("DevicesFound = %d, want 2", state.DevicesFound)
	}
	if len(state.Interfaces) != 1 || state.Interfaces[0] != "eth0" {
		t.Fatalf("Interfaces = %v, want [eth0]", state.Interfaces)
	}
	if !state.NextScanAt.Equal(wantNext) || !scheduled.Equal(wantNext) {
		t.Fatalf("next scan state=%v scheduled=%v want=%v", state.NextScanAt, scheduled, wantNext)
	}
}

func TestStartScanFailureStillSchedulesNextScan(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	oldNow := scannerNow
	oldScanNetwork := scanNetwork
	oldProcess := processScanResultFunc
	oldWait := waitForNextScan
	oldState := GetScannerState()

	lastSuccess := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	started := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)
	completed := started.Add(4 * time.Second)
	nowCalls := 0

	conf.SetAppConfigForTest(models.Conf{
		Ifaces:   "eth0",
		Timeout:  120,
		LogLevel: "info",
	})
	setScannerStateForTest(ScannerState{
		Status:               ScannerStatusHealthy,
		LastSuccessfulScanAt: lastSuccess,
	})
	scannerNow = func() time.Time {
		nowCalls++
		if nowCalls == 1 {
			return started
		}
		return completed
	}
	scanNetwork = func(context.Context, string, string, []string) arp.ScanResult {
		return arp.ScanResult{
			Success:    false,
			Interfaces: []string{"eth0"},
			Errors: []arp.ScanError{{
				Source:  "eth0",
				Kind:    arp.ScanErrorTimeout,
				Message: "command timed out",
			}},
		}
	}
	processScanResultFunc = func([]models.Host, bool) bool {
		return false
	}
	waitForNextScan = func(context.Context, time.Time) bool {
		return false
	}

	t.Cleanup(func() {
		conf.SetAppConfigForTest(oldConfig)
		scannerNow = oldNow
		scanNetwork = oldScanNetwork
		processScanResultFunc = oldProcess
		waitForNextScan = oldWait
		setScannerStateForTest(oldState)
	})

	startScan(context.Background())

	state := GetScannerState()
	if state.Status != ScannerStatusProblem {
		t.Fatalf("Status = %q, want %q", state.Status, ScannerStatusProblem)
	}
	if !state.LastSuccessfulScanAt.Equal(lastSuccess) {
		t.Fatalf("LastSuccessfulScanAt = %v, want preserved %v", state.LastSuccessfulScanAt, lastSuccess)
	}
	if !state.NextScanAt.Equal(completed.Add(120 * time.Second)) {
		t.Fatalf("NextScanAt = %v, want %v", state.NextScanAt, completed.Add(120*time.Second))
	}
	if len(state.LastErrors) != 1 || state.LastErrors[0].Kind != arp.ScanErrorTimeout {
		t.Fatalf("LastErrors = %+v, want timeout", state.LastErrors)
	}
}

func TestNoScanSourceDoesNotMarkOnlineHostsOffline(t *testing.T) {
	setupScanRoutineTest(t)

	oldNow := scannerNow
	oldScanNetwork := scanNetwork
	oldProcess := processScanResultFunc
	oldWait := waitForNextScan
	oldState := GetScannerState()

	started := time.Date(2026, 9, 16, 10, 30, 0, 0, time.UTC)
	completed := started.Add(20 * time.Millisecond)
	nowCalls := 0

	conf.AppConfig.Ifaces = ""
	conf.AppConfig.ArpArgs = "-r 1"
	conf.AppConfig.ArpStrs = nil
	conf.AppConfig.Timeout = 120

	gdb.Update("now", models.Host{
		Name:  "router",
		Iface: "eth0",
		IP:    "192.168.1.1",
		Mac:   "AA:BB:CC:DD:EE:99",
		Hw:    "Gateway Vendor",
		Date:  "2026-09-16 10:00:00",
		Known: 1,
		Now:   1,
	})
	hosts := gdb.SelectByMAC("now", "AA:BB:CC:DD:EE:99")
	if len(hosts) != 1 {
		t.Fatalf("seeded hosts len = %d, want 1", len(hosts))
	}

	setScannerStateForTest(ScannerState{})
	scannerNow = func() time.Time {
		nowCalls++
		if nowCalls == 1 {
			return started
		}
		return completed
	}
	scanNetwork = arp.ScanDetailedContext
	processScanResultFunc = processScanResult
	waitForNextScan = func(context.Context, time.Time) bool {
		return false
	}

	t.Cleanup(func() {
		scannerNow = oldNow
		scanNetwork = oldScanNetwork
		processScanResultFunc = oldProcess
		waitForNextScan = oldWait
		setScannerStateForTest(oldState)
	})

	startScan(context.Background())

	updated := gdb.SelectByID(hosts[0].ID)
	if updated.Now != 1 {
		t.Fatalf("Now after no-source scanner cycle = %d, want 1", updated.Now)
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 0 {
		t.Fatalf("events len after no-source scanner cycle = %d, want 0: %+v", len(events), events)
	}

	state := GetScannerState()
	if state.Status != ScannerStatusProblem {
		t.Fatalf("scanner Status = %q, want %q", state.Status, ScannerStatusProblem)
	}
	if state.DevicesFound != 0 {
		t.Fatalf("DevicesFound = %d, want 0", state.DevicesFound)
	}
	if len(state.LastErrors) != 1 {
		t.Fatalf("LastErrors len = %d, want 1", len(state.LastErrors))
	}
	if state.LastErrors[0].Kind != arp.ScanErrorConfiguration {
		t.Fatalf("LastErrors[0].Kind = %q, want %q", state.LastErrors[0].Kind, arp.ScanErrorConfiguration)
	}
	if state.LastErrors[0].Source != "configuration" {
		t.Fatalf("LastErrors[0].Source = %q, want configuration", state.LastErrors[0].Source)
	}
	if !state.NextScanAt.Equal(completed.Add(120 * time.Second)) {
		t.Fatalf("NextScanAt = %v, want %v", state.NextScanAt, completed.Add(120*time.Second))
	}
}

func TestWaitUntilNextScanStopsImmediatelyOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if waitUntilNextScan(ctx, time.Now().Add(time.Hour)) {
		t.Fatal("waitUntilNextScan returned true after context cancellation")
	}
}

func TestCompareHostsPreservesDeviceType(t *testing.T) {
	setupScanRoutineTest(t)

	gdb.Update("now", models.Host{
		Name:       "router",
		Iface:      "eth0",
		IP:         "192.168.1.1",
		Mac:        "AA:BB:CC:DD:EE:01",
		Hw:         "Gateway Vendor",
		Date:       "2026-08-24 08:00:00",
		Known:      1,
		Now:        1,
		DeviceType: "router",
	})
	hosts := gdb.SelectByMAC("now", "AA:BB:CC:DD:EE:01")
	if len(hosts) != 1 {
		t.Fatalf("seeded hosts len = %d, want 1", len(hosts))
	}

	compareHosts(map[string]models.Host{
		"AA:BB:CC:DD:EE:01": {
			Iface: "wifi0",
			IP:    "192.168.1.254",
			Mac:   "AA:BB:CC:DD:EE:01",
			Hw:    "Scanned Gateway Vendor",
			Date:  "2026-08-24 09:00:00",
			Now:   1,
		},
	})

	updated := gdb.SelectByID(hosts[0].ID)
	if updated.DeviceType != "router" {
		t.Fatalf("DeviceType = %q, want router", updated.DeviceType)
	}
	if updated.Iface != "wifi0" || updated.IP != "192.168.1.254" || updated.Now != 1 {
		t.Fatalf("network-derived fields were not refreshed: %+v", updated)
	}
}

func TestNewHostCreatesOneDiscoveredEvent(t *testing.T) {
	setupScanRoutineTest(t)

	processScanResult([]models.Host{
		{
			Iface: "eth0",
			IP:    "127.0.0.1",
			Mac:   "AA:BB:CC:DD:EE:10",
			Hw:    "New Device Vendor",
			Date:  "2026-08-24 10:00:00",
			Now:   1,
		},
	}, true)

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].EventType != string(models.EventDiscovered) {
		t.Fatalf("EventType = %q, want %q", events[0].EventType, models.EventDiscovered)
	}
	if events[0].Mac != "AA:BB:CC:DD:EE:10" {
		t.Fatalf("event Mac = %q, want AA:BB:CC:DD:EE:10", events[0].Mac)
	}
}

func TestRepeatedSuccessfulScanDoesNotDuplicateDiscoveredEvent(t *testing.T) {
	setupScanRoutineTest(t)

	host := models.Host{
		Iface: "eth0",
		IP:    "192.168.1.10",
		Mac:   "AA:BB:CC:DD:EE:10",
		Hw:    "New Device Vendor",
		Date:  "2026-08-24 10:00:00",
		Now:   1,
	}

	processScanResult([]models.Host{host}, true)
	host.Date = "2026-08-24 10:05:00"
	processScanResult([]models.Host{host}, true)

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1 discovered event: %+v", len(events), events)
	}
	if events[0].EventType != string(models.EventDiscovered) {
		t.Fatalf("EventType = %q, want %q", events[0].EventType, models.EventDiscovered)
	}
}

func TestSuccessfulScanCreatesAndUpdatesLifecycle(t *testing.T) {
	setupScanRoutineTest(t)

	host := models.Host{
		Iface: "eth0",
		IP:    "192.168.1.10",
		Mac:   "AA:BB:CC:DD:EE:10",
		Hw:    "New Device Vendor",
		Date:  "2026-08-24 10:00:00",
		Now:   1,
	}

	processScanResult([]models.Host{host}, true)
	assertScanLifecycle(t, host.Mac, "2026-08-24 10:00:00", "2026-08-24 10:00:00", false)

	host.Date = "2026-08-24 10:05:00"
	processScanResult([]models.Host{host}, true)
	assertScanLifecycle(t, host.Mac, "2026-08-24 10:00:00", "2026-08-24 10:05:00", false)
}

func TestOfflineScanDoesNotUpdateLifecycleLastSeen(t *testing.T) {
	setupScanRoutineTest(t)

	host := models.Host{
		ID:    1,
		Name:  "router",
		Iface: "eth0",
		IP:    "192.168.1.1",
		Mac:   "AA:BB:CC:DD:EE:01",
		Date:  "2026-08-24 08:00:00",
		Known: 1,
		Now:   1,
	}
	gdb.Update("now", host)
	if err := gdb.RecordHostObservation(host.Mac, host.Date); err != nil {
		t.Fatalf("RecordHostObservation: %v", err)
	}

	processScanResult([]models.Host{}, true)

	updated := gdb.SelectByID(host.ID)
	if updated.Now != 0 {
		t.Fatalf("Now after offline scan = %d, want 0", updated.Now)
	}
	assertScanLifecycle(t, host.Mac, "2026-08-24 08:00:00", "2026-08-24 08:00:00", false)
}

func TestFailedScanDoesNotUpdateLifecycle(t *testing.T) {
	setupScanRoutineTest(t)

	host := models.Host{
		ID:    1,
		Name:  "router",
		Iface: "eth0",
		IP:    "192.168.1.1",
		Mac:   "AA:BB:CC:DD:EE:01",
		Date:  "2026-08-24 08:00:00",
		Known: 1,
		Now:   1,
	}
	gdb.Update("now", host)
	if err := gdb.RecordHostObservation(host.Mac, host.Date); err != nil {
		t.Fatalf("RecordHostObservation: %v", err)
	}

	if processScanResult([]models.Host{
		{
			Iface: "eth0",
			IP:    "192.168.1.1",
			Mac:   "AA:BB:CC:DD:EE:01",
			Hw:    "Gateway Vendor",
			Date:  "2026-08-24 09:00:00",
			Now:   1,
		},
	}, false) {
		t.Fatal("processScanResult returned true for failed scan")
	}

	assertScanLifecycle(t, host.Mac, "2026-08-24 08:00:00", "2026-08-24 08:00:00", false)
}

func TestManualHostLifecycleStartsOnLaterRealObservation(t *testing.T) {
	setupScanRoutineTest(t)

	host := models.Host{
		ID:    1,
		Name:  "manual-host",
		Mac:   "AA:BB:CC:DD:EE:30",
		Known: 1,
		Now:   0,
	}
	gdb.Update("now", host)
	if err := gdb.EnsureHostLifecyclePlaceholder(host.Mac); err != nil {
		t.Fatalf("EnsureHostLifecyclePlaceholder: %v", err)
	}
	assertScanLifecycle(t, host.Mac, "", "", false)

	processScanResult([]models.Host{
		{
			Iface: "eth0",
			IP:    "192.168.1.30",
			Mac:   "AA:BB:CC:DD:EE:30",
			Hw:    "Manual Device Vendor",
			Date:  "2026-08-24 12:00:00",
			Now:   1,
		},
	}, true)

	assertScanLifecycle(t, host.Mac, "2026-08-24 12:00:00", "2026-08-24 12:00:00", false)
}

func TestOnlineOfflineTransitionsCreateSingleEvents(t *testing.T) {
	setupScanRoutineTest(t)

	gdb.Update("now", models.Host{
		Name:  "desktop",
		Iface: "eth0",
		IP:    "192.168.1.42",
		Mac:   "AA:BB:CC:DD:EE:42",
		Hw:    "Workstation NIC",
		Date:  "2026-08-24 08:00:00",
		Known: 1,
		Now:   0,
	})

	processScanResult([]models.Host{
		{
			Iface: "eth0",
			IP:    "192.168.1.42",
			Mac:   "AA:BB:CC:DD:EE:42",
			Hw:    "Workstation NIC",
			Date:  "2026-08-24 09:00:00",
			Now:   1,
		},
	}, true)
	assertEventTypes(t, []models.HostEventType{models.EventOnline})

	processScanResult([]models.Host{
		{
			Iface: "eth0",
			IP:    "192.168.1.42",
			Mac:   "AA:BB:CC:DD:EE:42",
			Hw:    "Workstation NIC",
			Date:  "2026-08-24 09:05:00",
			Now:   1,
		},
	}, true)
	assertEventTypes(t, []models.HostEventType{models.EventOnline})

	processScanResult([]models.Host{}, true)
	assertEventTypes(t, []models.HostEventType{models.EventOffline, models.EventOnline})

	processScanResult([]models.Host{}, true)
	assertEventTypes(t, []models.HostEventType{models.EventOffline, models.EventOnline})
}

func assertScanLifecycle(t *testing.T, mac, firstSeen, lastSeen string, estimated bool) {
	t.Helper()

	lifecycle, ok, err := gdb.SelectHostLifecycleByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostLifecycleByMAC: %v", err)
	}
	if !ok {
		t.Fatalf("lifecycle for %s not found", mac)
	}
	if lifecycle.FirstSeen != firstSeen || lifecycle.LastSeen != lastSeen || lifecycle.FirstSeenEstimated != estimated {
		t.Fatalf("lifecycle for %s = %+v, want first=%q last=%q estimated=%v", mac, lifecycle, firstSeen, lastSeen, estimated)
	}
}

func TestFailedScanDoesNotCreateOfflineEvent(t *testing.T) {
	setupScanRoutineTest(t)

	gdb.Update("now", models.Host{
		Name:  "router",
		Iface: "eth0",
		IP:    "192.168.1.1",
		Mac:   "AA:BB:CC:DD:EE:01",
		Hw:    "Gateway Vendor",
		Date:  "2026-08-24 08:00:00",
		Known: 1,
		Now:   1,
	})
	hosts := gdb.SelectByMAC("now", "AA:BB:CC:DD:EE:01")
	if len(hosts) != 1 {
		t.Fatalf("seeded hosts len = %d, want 1", len(hosts))
	}

	if processScanResult(nil, false) {
		t.Fatal("processScanResult returned true for failed scan")
	}

	updated := gdb.SelectByID(hosts[0].ID)
	if updated.Now != 1 {
		t.Fatalf("Now after failed scan = %d, want 1", updated.Now)
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 0 {
		t.Fatalf("events len after failed scan = %d, want 0: %+v", len(events), events)
	}
}

func TestHostReturnsAfterFailedScanCreatesOnlyOnlineTransition(t *testing.T) {
	setupScanRoutineTest(t)

	gdb.Update("now", models.Host{
		Name:  "phone",
		Iface: "wifi0",
		IP:    "192.168.1.83",
		Mac:   "AA:BB:CC:DD:EE:83",
		Hw:    "Mobile Vendor",
		Date:  "2026-08-24 08:00:00",
		Known: 1,
		Now:   0,
	})

	if processScanResult(nil, false) {
		t.Fatal("processScanResult returned true for failed scan")
	}

	processScanResult([]models.Host{
		{
			Iface: "wifi0",
			IP:    "192.168.1.83",
			Mac:   "AA:BB:CC:DD:EE:83",
			Hw:    "Mobile Vendor",
			Date:  "2026-08-24 09:00:00",
			Now:   1,
		},
	}, true)

	assertEventTypes(t, []models.HostEventType{models.EventOnline})
}

func TestRetentionDeletesOldPresenceAndOnlyOldConnectivity(t *testing.T) {
	setupScanRoutineTest(t)

	host := models.Host{
		ID:    7,
		Name:  "storage",
		Iface: "eth0",
		IP:    "192.168.1.20",
		Mac:   "AA:BB:CC:DD:EE:20",
		Known: 1,
		Now:   1,
	}

	seededEvents := []struct {
		eventType models.HostEventType
		date      string
	}{
		{models.EventOnline, "2026-08-22 08:00:00"},
		{models.EventOffline, "2026-08-22 08:05:00"},
		{models.EventOnline, "2026-08-24 08:00:00"},
		{models.EventOffline, "2026-08-24 08:05:00"},
		{models.EventDiscovered, "2026-08-22 08:10:00"},
		{models.EventKnown, "2026-08-22 08:15:00"},
		{models.EventUnknown, "2026-08-22 08:20:00"},
		{models.EventDeviceTypeChanged, "2026-08-22 08:25:00"},
	}
	for _, item := range seededEvents {
		event := models.NewHostEvent(host, item.eventType, "", "")
		event.Date = item.date
		if err := gdb.AddEvent(event); err != nil {
			t.Fatalf("AddEvent %s: %v", item.eventType, err)
		}
	}

	gdb.Update("history", models.Host{
		Name: "old-presence-row",
		Mac:  "AA:BB:CC:DD:EE:20",
		Date: "2026-08-22 08:00:00",
	})
	gdb.Update("history", models.Host{
		Name: "recent-presence-row",
		Mac:  "AA:BB:CC:DD:EE:20",
		Date: "2026-08-24 08:00:00",
	})

	if deleted := gdb.DeleteOldHistory("2026-08-23 00:00:00"); deleted != 1 {
		t.Fatalf("DeleteOldHistory deleted = %d, want 1", deleted)
	}

	if deleted := gdb.DeleteOldConnectivityEvents("2026-08-23 00:00:00"); deleted != 2 {
		t.Fatalf("DeleteOldConnectivityEvents deleted = %d, want 2", deleted)
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	eventCounts := map[string]int{}
	for _, event := range events {
		eventCounts[event.EventType]++
		if (event.EventType == string(models.EventOnline) || event.EventType == string(models.EventOffline)) && event.Date < "2026-08-23 00:00:00" {
			t.Fatalf("old connectivity event remained: %+v", event)
		}
	}

	wantCounts := map[models.HostEventType]int{
		models.EventOnline:            1,
		models.EventOffline:           1,
		models.EventDiscovered:        1,
		models.EventKnown:             1,
		models.EventUnknown:           1,
		models.EventDeviceTypeChanged: 1,
	}
	for eventType, want := range wantCounts {
		if got := eventCounts[string(eventType)]; got != want {
			t.Fatalf("remaining %s events = %d, want %d; events: %+v", eventType, got, want, events)
		}
	}

	history, ok := gdb.Select("history")
	if !ok {
		t.Fatal("Select history failed")
	}
	if len(history) != 1 || history[0].Name != "recent-presence-row" {
		t.Fatalf("history rows = %+v, want only recent presence row", history)
	}
}

func assertEventTypes(t *testing.T, want []models.HostEventType) {
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
