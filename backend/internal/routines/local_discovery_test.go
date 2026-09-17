package routines

import (
	"context"
	"os"
	"testing"

	"github.com/godlev/LANnventory/internal/discovery"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

// Keep routines tests deterministic and independent from the runner's DNS/NSS/
// Avahi environment. Individual tests opt in by replacing the hook explicitly.
func TestMain(m *testing.M) {
	localHostnameDiscovery = func(context.Context, string) []discovery.HostnameObservation {
		return nil
	}
	os.Exit(m.Run())
}

func TestProcessScanResultPersistsLocalHostnameEvidenceForExistingManualHost(t *testing.T) {
	setupScanRoutineTest(t)
	oldDiscovery := localHostnameDiscovery
	localHostnameDiscovery = func(_ context.Context, address string) []discovery.HostnameObservation {
		if address != "192.168.1.70" {
			t.Fatalf("hostname discovery address = %q, want 192.168.1.70", address)
		}
		return []discovery.HostnameObservation{
			{Source: models.DiscoverySourceReverseDNS, Values: []string{"bravia.home"}},
			{Source: models.DiscoverySourceSystemResolver, Values: []string{"BRAVIA-4K"}},
			{Source: models.DiscoverySourceMDNS, Values: []string{"Sony-TV.local"}},
		}
	}
	t.Cleanup(func() { localHostnameDiscovery = oldDiscovery })

	mac := "AA:BB:CC:DD:EE:70"
	gdb.Update("now", models.Host{
		Name:  "Living Room TV",
		Iface: "eth0",
		IP:    "192.168.1.70",
		Mac:   mac,
		Hw:    "Sony",
		Date:  "2026-09-17 14:00:00",
		Known: 1,
		Now:   1,
	})

	if !processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.70", Mac: mac, Hw: "Sony", Date: "2026-09-17 14:30:00", Now: 1},
	}, true) {
		t.Fatal("successful scan was not applied")
	}

	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 || hosts[0].Name != "Living Room TV" {
		t.Fatalf("manual inventory name was changed by discovery: %+v", hosts)
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	active := make(map[string]bool)
	for _, row := range rows {
		if row.Active {
			active[row.Source+"|"+row.Kind+"|"+row.Value] = true
		}
	}
	for _, key := range []string{
		models.DiscoverySourceReverseDNS + "|" + models.DiscoveryKindHostname + "|bravia.home",
		models.DiscoverySourceSystemResolver + "|" + models.DiscoveryKindHostname + "|BRAVIA-4K",
		models.DiscoverySourceMDNS + "|" + models.DiscoveryKindHostname + "|Sony-TV.local",
	} {
		if !active[key] {
			t.Fatalf("missing active local hostname evidence %q in %+v", key, rows)
		}
	}
}

func TestLocalHostnameDiscoveryFailureDoesNotChangeSuccessfulScanSemantics(t *testing.T) {
	setupScanRoutineTest(t)
	oldDiscovery := localHostnameDiscovery
	localHostnameDiscovery = func(context.Context, string) []discovery.HostnameObservation {
		return nil
	}
	t.Cleanup(func() { localHostnameDiscovery = oldDiscovery })

	mac := "AA:BB:CC:DD:EE:71"
	if !processScanResult([]models.Host{
		{Iface: "eth0", IP: "192.168.1.71", Mac: mac, Hw: "Vendor", Date: "2026-09-17 14:35:00", Now: 1},
	}, true) {
		t.Fatal("best-effort discovery changed successful scan result")
	}

	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 || hosts[0].Now != 1 {
		t.Fatalf("host core state was not committed: %+v", hosts)
	}
}
