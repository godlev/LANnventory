package routines

import (
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestMultiIPCompatibilityKeepsCurrentIPAndManagedInventory(t *testing.T) {
	setupScanRoutineTest(t)
	mac := "AA:BB:CC:DD:EE:90"

	gdb.Update("now", models.Host{
		Name:       "Living Room TV",
		Iface:      "eth0",
		IP:         "192.168.1.20",
		Mac:        mac,
		Hw:         "Original Vendor",
		Date:       "2026-09-17 15:00:00",
		Known:      1,
		Now:        1,
		DeviceType: "tv",
	})

	observations := []models.Host{
		{Iface: "wifi0", IP: "192.168.1.21", Mac: mac, Hw: "Scanner Vendor", Date: "2026-09-17 15:05:02", Now: 1},
		{Iface: "eth0", IP: "192.168.1.20", Mac: mac, Hw: "Scanner Vendor", Date: "2026-09-17 15:05:01", Now: 1},
	}
	if !processScanResult(observations, true) {
		t.Fatal("successful multi-IP scan was not applied")
	}

	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 {
		t.Fatalf("current host count = %d, want 1: %+v", len(hosts), hosts)
	}
	if hosts[0].IP != "192.168.1.20" {
		t.Fatalf("compatibility IP = %q, want preserved current 192.168.1.20", hosts[0].IP)
	}
	if hosts[0].Date != "2026-09-17 15:05:02" {
		t.Fatalf("compatibility Date = %q, want newest MAC observation", hosts[0].Date)
	}
	if hosts[0].Name != "Living Room TV" || hosts[0].DeviceType != "tv" || hosts[0].Known != 1 {
		t.Fatalf("managed inventory changed during multi-IP scan: %+v", hosts[0])
	}

	addresses, err := gdb.SelectHostAddressesByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostAddressesByMAC: %v", err)
	}
	if len(addresses) != 2 || !addresses[0].Active || !addresses[1].Active {
		t.Fatalf("address observations = %+v, want two active addresses", addresses)
	}

	lifecycle, ok, err := gdb.SelectHostLifecycleByMAC(mac)
	if err != nil || !ok {
		t.Fatalf("SelectHostLifecycleByMAC ok=%v err=%v", ok, err)
	}
	if lifecycle.LastSeen != "2026-09-17 15:05:02" {
		t.Fatalf("LastSeen = %q, want newest observation", lifecycle.LastSeen)
	}

	events, ok := gdb.SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 0 {
		t.Fatalf("stable online multi-IP scan created events: %+v", events)
	}

	history, ok := gdb.Select("history")
	if !ok {
		t.Fatal("Select history failed")
	}
	if len(history) != 1 || history[0].Mac != mac || history[0].IP != "192.168.1.20" {
		t.Fatalf("presence history = %+v, want one compatibility row for the MAC", history)
	}

	// Reverse scanner order on the next scan. Primary IP must not flap.
	observations[0], observations[1] = observations[1], observations[0]
	observations[0].Date = "2026-09-17 15:10:01"
	observations[1].Date = "2026-09-17 15:10:02"
	processScanResult(observations, true)
	hosts = gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 || hosts[0].IP != "192.168.1.20" || hosts[0].Date != "2026-09-17 15:10:02" {
		t.Fatalf("compatibility identity changed with scanner order: %+v", hosts)
	}
}

func TestMultiIPCompatibilityFallbackIsDeterministicAndIPv4First(t *testing.T) {
	setupScanRoutineTest(t)
	mac := "AA:BB:CC:DD:EE:91"

	gdb.Update("now", models.Host{
		Name: "multi-ip", Iface: "eth0", IP: "192.168.1.99", Mac: mac,
		Date: "2026-09-17 16:00:00", Known: 1, Now: 1,
	})

	processScanResult([]models.Host{
		{Iface: "eth0", IP: "2001:db8::1", Mac: mac, Date: "2026-09-17 16:05:00", Now: 1},
		{Iface: "eth0", IP: "192.168.1.10", Mac: mac, Date: "2026-09-17 16:05:00", Now: 1},
		{Iface: "eth0", IP: "192.168.1.2", Mac: mac, Date: "2026-09-17 16:05:00", Now: 1},
	}, true)

	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 || hosts[0].IP != "192.168.1.2" {
		t.Fatalf("fallback compatibility IP = %+v, want numeric-lowest IPv4 192.168.1.2", hosts)
	}
}
