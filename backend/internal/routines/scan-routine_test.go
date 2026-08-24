package routines

import (
	"path/filepath"
	"testing"

	"github.com/aceberg/WatchYourLAN/internal/conf"
	"github.com/aceberg/WatchYourLAN/internal/gdb"
	"github.com/aceberg/WatchYourLAN/internal/models"
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

func TestDeleteOldEventsDeletesOnlyExpiredActivity(t *testing.T) {
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
	oldEvent := models.NewHostEvent(host, models.EventOnline, "", "")
	oldEvent.Date = "2026-08-22 08:00:00"
	recentEvent := models.NewHostEvent(host, models.EventOffline, "", "")
	recentEvent.Date = "2026-08-24 08:00:00"

	if err := gdb.AddEvent(oldEvent); err != nil {
		t.Fatalf("AddEvent old: %v", err)
	}
	if err := gdb.AddEvent(recentEvent); err != nil {
		t.Fatalf("AddEvent recent: %v", err)
	}

	gdb.Update("history", models.Host{
		Name: "history-row",
		Mac:  "AA:BB:CC:DD:EE:20",
		Date: "2026-08-22 08:00:00",
	})

	if deleted := gdb.DeleteOldEvents("2026-08-23 00:00:00"); deleted != 1 {
		t.Fatalf("DeleteOldEvents deleted = %d, want 1", deleted)
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 1 || events[0].EventType != string(models.EventOffline) {
		t.Fatalf("remaining events = %+v, want only offline recent event", events)
	}

	history, ok := gdb.Select("history")
	if !ok {
		t.Fatal("Select history failed")
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
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
