package gdb

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestServiceInventoryMigrationAndRoundTrip(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "services.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}
	if !db.Migrator().HasTable(servicesTable) {
		t.Fatal("services table missing")
	}
	if !db.Migrator().HasTable(serviceScanSettingsTable) {
		t.Fatal("service_scan_settings table missing")
	}

	first, err := UpsertService(models.Service{
		Mac:            "aa-bb-cc-dd-ee-34",
		Address:        "192.168.1.34",
		Protocol:       "TCP",
		Port:           443,
		State:          "OPEN",
		FirstDetected:  "2026-09-18 10:00:00",
		LastDetected:   "2026-09-18 10:00:00",
		LastChecked:    "2026-09-18 10:00:00",
		StateChangedAt: "2026-09-18 10:00:00",
		ServiceHint:    " HTTPS ",
		LastScanSource: " manual ",
	})
	if err != nil {
		t.Fatalf("UpsertService first: %v", err)
	}
	if first.Mac != "AA:BB:CC:DD:EE:34" || first.Address != "192.168.1.34" || first.AddressFamily != "ipv4" {
		t.Fatalf("normalized service = %+v", first)
	}
	if first.Protocol != "tcp" || first.State != "open" || first.ServiceHint != "HTTPS" || first.LastScanSource != "manual" {
		t.Fatalf("normalized service fields = %+v", first)
	}

	second, err := UpsertService(models.Service{
		Mac:            "AA:BB:CC:DD:EE:34",
		Address:        "192.168.1.34",
		Protocol:       "tcp",
		Port:           443,
		State:          "closed",
		FirstDetected:  "2026-09-18 10:00:00",
		LastDetected:   "2026-09-18 10:00:00",
		LastChecked:    "2026-09-18 12:00:00",
		StateChangedAt: "2026-09-18 12:00:00",
		ServiceHint:    "HTTPS",
		LastScanSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("UpsertService second: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("service identity duplicated: first ID=%d second ID=%d", first.ID, second.ID)
	}
	if second.State != "closed" || second.LastChecked != "2026-09-18 12:00:00" || second.LastScanSource != "scheduled" {
		t.Fatalf("service upsert did not refresh summary: %+v", second)
	}

	services, err := SelectServicesByMAC("aa:bb:cc:dd:ee:34")
	if err != nil {
		t.Fatalf("SelectServicesByMAC: %v", err)
	}
	if len(services) != 1 || services[0].ID != first.ID {
		t.Fatalf("services = %+v, want one retained summary", services)
	}

	selected, ok, err := SelectServiceByIdentity("AA:BB:CC:DD:EE:34", "192.168.1.34", "TCP", 443)
	if err != nil || !ok {
		t.Fatalf("SelectServiceByIdentity ok=%v err=%v", ok, err)
	}
	if selected.State != "closed" {
		t.Fatalf("selected service = %+v, want closed", selected)
	}
}

func TestServiceInventoryCanonicalizesIPv6Address(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "services-ipv6.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	service, err := UpsertService(models.Service{
		Mac:           "AA:BB:CC:DD:EE:37",
		Address:       "2001:0db8:0:0:0:0:0:1",
		Protocol:      "tcp",
		Port:          443,
		State:         "open",
		FirstDetected: "2026-09-18 10:00:00",
		LastDetected:  "2026-09-18 10:00:00",
		LastChecked:   "2026-09-18 10:00:00",
	})
	if err != nil {
		t.Fatalf("UpsertService IPv6: %v", err)
	}
	if service.Address != "2001:db8::1" || service.AddressFamily != "ipv6" {
		t.Fatalf("IPv6 service = %+v", service)
	}
}

func TestServiceScanSettingsUpsert(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "service-settings.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	first, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             "aa-bb-cc-dd-ee-35",
		Enabled:         false,
		IntervalMinutes: 1440,
		PortsJSON:       "[22,80,443]",
		NextScanAt:      "2026-09-19 08:00:00",
	})
	if err != nil {
		t.Fatalf("UpsertServiceScanSettings first: %v", err)
	}
	if first.Mac != "AA:BB:CC:DD:EE:35" || first.Enabled {
		t.Fatalf("first settings = %+v", first)
	}

	second, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:              "AA:BB:CC:DD:EE:35",
		Enabled:          true,
		IntervalMinutes:  60,
		PortsJSON:        "[22,443]",
		NextScanAt:       "2026-09-18 13:00:00",
		LastAttemptAt:    "2026-09-18 12:00:00",
		LastSuccessfulAt: "2026-09-18 12:00:00",
	})
	if err != nil {
		t.Fatalf("UpsertServiceScanSettings second: %v", err)
	}
	if !second.Enabled || second.IntervalMinutes != 60 || second.PortsJSON != "[22,443]" {
		t.Fatalf("second settings = %+v", second)
	}

	selected, ok, err := SelectServiceScanSettingsByMAC("aa:bb:cc:dd:ee:35")
	if err != nil || !ok {
		t.Fatalf("SelectServiceScanSettingsByMAC ok=%v err=%v", ok, err)
	}
	if !selected.Enabled || selected.IntervalMinutes != 60 {
		t.Fatalf("selected settings = %+v", selected)
	}

	var count int64
	if err := db.Table(serviceScanSettingsTable).Count(&count).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("settings count = %d, want 1", count)
	}
}

func TestServiceInventoryRejectsInvalidIdentityAndState(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "invalid-services.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	tests := []struct {
		name    string
		service models.Service
		wantErr error
	}{
		{
			name:    "invalid mac",
			service: models.Service{Mac: "bad", Address: "192.168.1.1", Protocol: "tcp", Port: 22, State: "open"},
			wantErr: errInvalidServiceIdentity,
		},
		{
			name:    "invalid address",
			service: models.Service{Mac: "AA:BB:CC:DD:EE:36", Address: "not-an-ip", Protocol: "tcp", Port: 22, State: "open"},
			wantErr: errInvalidServiceIdentity,
		},
		{
			name:    "invalid port",
			service: models.Service{Mac: "AA:BB:CC:DD:EE:36", Address: "192.168.1.1", Protocol: "tcp", Port: 65536, State: "open"},
			wantErr: errInvalidServiceIdentity,
		},
		{
			name:    "invalid protocol",
			service: models.Service{Mac: "AA:BB:CC:DD:EE:36", Address: "192.168.1.1", Protocol: "udp", Port: 53, State: "open"},
			wantErr: errInvalidServiceProtocol,
		},
		{
			name:    "invalid state",
			service: models.Service{Mac: "AA:BB:CC:DD:EE:36", Address: "192.168.1.1", Protocol: "tcp", Port: 22, State: "unknown"},
			wantErr: errInvalidServiceState,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := UpsertService(test.service)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}

	if _, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             "AA:BB:CC:DD:EE:36",
		Enabled:         true,
		IntervalMinutes: 0,
		PortsJSON:       "[22]",
	}); !errors.Is(err, errInvalidServiceInterval) {
		t.Fatalf("settings error = %v, want %v", err, errInvalidServiceInterval)
	}
}

func TestServiceScanSettingsCanonicalDueAndRuntimeUpdate(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "service-due.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	due, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             "AA:BB:CC:DD:EE:C0",
		Enabled:         true,
		IntervalMinutes: 60,
		PortsJSON:       "[443,22,443]",
		NextScanAt:      "2026-09-18 10:00:00",
	})
	if err != nil {
		t.Fatalf("Upsert due: %v", err)
	}
	if due.PortsJSON != "[22,443]" {
		t.Fatalf("canonical ports = %q, want [22,443]", due.PortsJSON)
	}

	if _, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             "AA:BB:CC:DD:EE:C1",
		Enabled:         true,
		IntervalMinutes: 60,
		PortsJSON:       "[80]",
		NextScanAt:      "2026-09-18 12:00:00",
	}); err != nil {
		t.Fatalf("Upsert future: %v", err)
	}
	if _, err := UpsertServiceScanSettings(models.ServiceScanSettings{
		Mac:             "AA:BB:CC:DD:EE:C2",
		Enabled:         false,
		IntervalMinutes: 60,
		PortsJSON:       "[]",
		NextScanAt:      "2026-09-18 09:00:00",
	}); err != nil {
		t.Fatalf("Upsert disabled: %v", err)
	}

	rows, err := SelectDueServiceScanSettings("2026-09-18 11:00:00", 10)
	if err != nil {
		t.Fatalf("SelectDueServiceScanSettings: %v", err)
	}
	if len(rows) != 1 || rows[0].Mac != "AA:BB:CC:DD:EE:C0" {
		t.Fatalf("due rows = %+v", rows)
	}

	if err := UpdateServiceScanRuntime(
		due.Mac,
		"2026-09-18 12:00:00",
		"2026-09-18 11:00:00",
		"2 of 4 probes indeterminate",
		false,
	); err != nil {
		t.Fatalf("UpdateServiceScanRuntime failure: %v", err)
	}

	stored, found, err := SelectServiceScanSettingsByMAC(due.Mac)
	if err != nil || !found {
		t.Fatalf("Select after failure found=%v err=%v", found, err)
	}
	if stored.LastAttemptAt != "2026-09-18 11:00:00" || stored.LastSuccessfulAt != "" || stored.LastError == "" || stored.NextScanAt != "2026-09-18 12:00:00" {
		t.Fatalf("failure runtime = %+v", stored)
	}

	if err := UpdateServiceScanRuntime(
		due.Mac,
		"2026-09-18 13:00:00",
		"2026-09-18 12:00:00",
		"",
		true,
	); err != nil {
		t.Fatalf("UpdateServiceScanRuntime success: %v", err)
	}
	stored, found, err = SelectServiceScanSettingsByMAC(due.Mac)
	if err != nil || !found {
		t.Fatalf("Select after success found=%v err=%v", found, err)
	}
	if stored.LastSuccessfulAt != "2026-09-18 12:00:00" || stored.LastError != "" || stored.NextScanAt != "2026-09-18 13:00:00" {
		t.Fatalf("success runtime = %+v", stored)
	}
}
