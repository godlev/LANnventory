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
		Address:        "192.168.001.034",
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
			name: "invalid mac",
			service: models.Service{Mac: "bad", Address: "192.168.1.1", Protocol: "tcp", Port: 22, State: "open"},
			wantErr: errInvalidServiceIdentity,
		},
		{
			name: "invalid address",
			service: models.Service{Mac: "AA:BB:CC:DD:EE:36", Address: "not-an-ip", Protocol: "tcp", Port: 22, State: "open"},
			wantErr: errInvalidServiceIdentity,
		},
		{
			name: "invalid port",
			service: models.Service{Mac: "AA:BB:CC:DD:EE:36", Address: "192.168.1.1", Protocol: "tcp", Port: 65536, State: "open"},
			wantErr: errInvalidServiceIdentity,
		},
		{
			name: "invalid protocol",
			service: models.Service{Mac: "AA:BB:CC:DD:EE:36", Address: "192.168.1.1", Protocol: "udp", Port: 53, State: "open"},
			wantErr: errInvalidServiceProtocol,
		},
		{
			name: "invalid state",
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
