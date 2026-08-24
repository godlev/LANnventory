package routines

import (
	"path/filepath"
	"testing"

	"github.com/aceberg/WatchYourLAN/internal/conf"
	"github.com/aceberg/WatchYourLAN/internal/gdb"
	"github.com/aceberg/WatchYourLAN/internal/models"
)

func TestCompareHostsPreservesDeviceType(t *testing.T) {
	oldConfig := conf.AppConfig
	conf.AppConfig.UseDB = "sqlite"
	conf.AppConfig.DBPath = filepath.Join(t.TempDir(), "scan-refresh-test.db")
	conf.AppConfig.InfluxEnable = false
	conf.AppConfig.PrometheusEnable = false
	gdb.Start()

	t.Cleanup(func() {
		if err := gdb.Close(); err != nil {
			t.Errorf("gdb.Close: %v", err)
		}
		conf.AppConfig = oldConfig
	})

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
