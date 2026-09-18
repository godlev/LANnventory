package routines

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/portscan"
)

func TestRunDueServiceScansPersistsDefinitiveResultsAndRuntime(t *testing.T) {
	startServiceScanTestDB(t)

	oldAddressHost := models.Host{
		Name:  "server",
		IP:    "192.168.1.89",
		Mac:   "AA:BB:CC:DD:EE:D0",
		Iface: "eth0",
		Date:  "2026-09-18 09:00:00",
		Known: 1,
		Now:   1,
	}
	gdb.Update("now", oldAddressHost)
	if err := gdb.RecordHostAddressObservations([]models.Host{oldAddressHost}); err != nil {
		t.Fatalf("RecordHostAddressObservations old: %v", err)
	}

	host := oldAddressHost
	host.IP = "192.168.1.90"
	host.Date = "2026-09-18 10:00:00"
	currentRows := gdb.SelectByMAC("now", host.Mac)
	if len(currentRows) != 1 {
		t.Fatalf("current rows before update = %+v", currentRows)
	}
	host.ID = currentRows[0].ID
	gdb.Update("now", host)
	if err := gdb.RecordHostAddressObservations([]models.Host{host}); err != nil {
		t.Fatalf("RecordHostAddressObservations current: %v", err)
	}

	if _, err := gdb.UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             host.Mac,
		Enabled:         true,
		IntervalMinutes: 60,
		PortsJSON:       "[22,443]",
		NextScanAt:      "2026-09-18 10:30:00",
	}); err != nil {
		t.Fatalf("UpsertServiceScanSettings: %v", err)
	}

	originalScanPorts := scheduledScanPorts
	var targets []string
	scheduledScanPorts = func(_ context.Context, target string, ports []int, workers int) []portscan.PortResult {
		targets = append(targets, target)
		if workers != serviceScanWorkers {
			t.Fatalf("workers = %d, want %d", workers, serviceScanWorkers)
		}
		return []portscan.PortResult{
			{Port: 22, Result: portscan.Result{State: portscan.ProbeClosed}},
			{Port: 443, Result: portscan.Result{State: portscan.ProbeOpen}},
		}
	}
	t.Cleanup(func() {
		scheduledScanPorts = originalScanPorts
	})

	now := time.Date(2026, 9, 18, 11, 0, 0, 0, time.Local)
	if attempted := runDueServiceScansAt(context.Background(), now); attempted != 1 {
		t.Fatalf("attempted = %d, want 1", attempted)
	}
	if len(targets) != 1 || targets[0] != host.IP {
		t.Fatalf("targets = %v, want only active address %s", targets, host.IP)
	}

	service, found, err := gdb.SelectServiceByIdentity(host.Mac, host.IP, "tcp", 443)
	if err != nil || !found {
		t.Fatalf("SelectServiceByIdentity open found=%v err=%v", found, err)
	}
	if service.State != "open" || service.LastScanSource != "scheduled" || service.LastChecked != "2026-09-18 11:00:00" {
		t.Fatalf("open service = %+v", service)
	}
	if _, found, err := gdb.SelectServiceByIdentity(host.Mac, host.IP, "tcp", 22); err != nil || found {
		t.Fatalf("never-open closed service found=%v err=%v", found, err)
	}

	settings, found, err := gdb.SelectServiceScanSettingsByMAC(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectServiceScanSettingsByMAC found=%v err=%v", found, err)
	}
	if settings.LastAttemptAt != "2026-09-18 11:00:00" ||
		settings.LastSuccessfulAt != "2026-09-18 11:00:00" ||
		settings.NextScanAt != "2026-09-18 12:00:00" ||
		settings.LastError != "" {
		t.Fatalf("runtime settings = %+v", settings)
	}
}

func TestRunDueServiceScansLeavesStateUntouchedOnIndeterminate(t *testing.T) {
	startServiceScanTestDB(t)

	host := models.Host{
		Name:  "nas",
		IP:    "192.168.1.91",
		Mac:   "AA:BB:CC:DD:EE:D1",
		Iface: "eth0",
		Date:  "2026-09-18 10:00:00",
		Known: 1,
		Now:   1,
	}
	gdb.Update("now", host)
	currentRows := gdb.SelectByMAC("now", host.Mac)
	host.ID = currentRows[0].ID
	if err := gdb.RecordHostAddressObservations([]models.Host{host}); err != nil {
		t.Fatalf("RecordHostAddressObservations: %v", err)
	}
	if _, _, _, err := gdb.RecordHostServiceObservation(host, models.Service{
		Mac:            host.Mac,
		Address:        host.IP,
		Protocol:       "tcp",
		Port:           445,
		State:          "open",
		LastScanSource: "manual",
	}, "2026-09-18 10:00:00"); err != nil {
		t.Fatalf("RecordHostServiceObservation seed: %v", err)
	}
	if _, err := gdb.UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             host.Mac,
		Enabled:         true,
		IntervalMinutes: 30,
		PortsJSON:       "[445]",
		NextScanAt:      "2026-09-18 10:30:00",
	}); err != nil {
		t.Fatalf("UpsertServiceScanSettings: %v", err)
	}

	originalScanPorts := scheduledScanPorts
	scheduledScanPorts = func(_ context.Context, target string, ports []int, workers int) []portscan.PortResult {
		return []portscan.PortResult{
			{Port: 445, Result: portscan.Result{State: portscan.ProbeIndeterminate}},
		}
	}
	t.Cleanup(func() {
		scheduledScanPorts = originalScanPorts
	})

	now := time.Date(2026, 9, 18, 11, 0, 0, 0, time.Local)
	if attempted := runDueServiceScansAt(context.Background(), now); attempted != 1 {
		t.Fatalf("attempted = %d, want 1", attempted)
	}

	service, found, err := gdb.SelectServiceByIdentity(host.Mac, host.IP, "tcp", 445)
	if err != nil || !found {
		t.Fatalf("SelectServiceByIdentity found=%v err=%v", found, err)
	}
	if service.State != "open" || service.LastChecked != "2026-09-18 10:00:00" || service.LastDetected != "2026-09-18 10:00:00" {
		t.Fatalf("indeterminate scan changed service = %+v", service)
	}

	settings, found, err := gdb.SelectServiceScanSettingsByMAC(host.Mac)
	if err != nil || !found {
		t.Fatalf("SelectServiceScanSettingsByMAC found=%v err=%v", found, err)
	}
	if settings.LastSuccessfulAt != "" || settings.NextScanAt != "2026-09-18 11:30:00" || !strings.Contains(settings.LastError, "indeterminate") {
		t.Fatalf("indeterminate runtime settings = %+v", settings)
	}
}

func TestScheduledProbeTargetAddsZoneForLinkLocalIPv6(t *testing.T) {
	target := scheduledProbeTarget(models.HostAddress{
		Address: "fe80::1234",
		Iface:   "eth0",
		Active:  true,
	})
	if target != "fe80::1234%eth0" {
		t.Fatalf("target = %q, want fe80::1234%%eth0", target)
	}

	global := scheduledProbeTarget(models.HostAddress{
		Address: "2001:db8::1",
		Iface:   "eth0",
		Active:  true,
	})
	if global != "2001:db8::1" {
		t.Fatalf("global target = %q, want 2001:db8::1", global)
	}
}

func startServiceScanTestDB(t *testing.T) {
	t.Helper()

	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "service-scheduler.db"),
	})
	t.Cleanup(func() {
		ServiceScanStop()
		if err := gdb.Close(); err != nil {
			t.Errorf("gdb.Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := gdb.StartErr(); err != nil {
		t.Fatalf("gdb.StartErr: %v", err)
	}
}
