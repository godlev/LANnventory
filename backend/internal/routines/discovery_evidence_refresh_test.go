package routines

import (
	"context"
	"testing"

	"github.com/godlev/LANnventory/internal/discovery"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestSuccessfulScanMovesMissingLocalEvidenceToPrevious(t *testing.T) {
	setupScanRoutineTest(t)
	oldLocal := localHostnameDiscovery
	localHostnameDiscovery = func(context.Context, string) []discovery.HostnameObservation {
		return []discovery.HostnameObservation{
			{Source: models.DiscoverySourceReverseDNS, Values: []string{"new-tv.home"}},
		}
	}
	t.Cleanup(func() { localHostnameDiscovery = oldLocal })

	mac := "AA:BB:CC:DD:EE:B1"
	address := "192.168.1.91"
	gdb.Update("now", models.Host{
		Name: "Living Room TV", Iface: "eth0", IP: address, Mac: mac,
		Date: "2026-09-17 18:00:00", Known: 1, Now: 1, DeviceType: "tv",
	})

	seedEvidence(t, mac, address, models.DiscoverySourceReverseDNS, models.DiscoveryKindHostname, "old-tv.home")
	seedEvidence(t, mac, address, models.DiscoverySourceSystemResolver, models.DiscoveryKindHostname, "OLD-TV")
	seedEvidence(t, mac, address, models.DiscoverySourceMDNS, models.DiscoveryKindHostname, "Old-TV.local")

	if !processScanResult([]models.Host{
		{Iface: "eth0", IP: address, Mac: mac, Hw: "Sony", Date: "2026-09-17 18:05:00", Now: 1},
	}, true) {
		t.Fatal("successful scan was not applied")
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	byKey := evidenceByKey(rows)

	if byKey[models.DiscoverySourceReverseDNS+"|old-tv.home"].Active {
		t.Fatalf("old reverse-DNS evidence remained current: %+v", rows)
	}
	if !byKey[models.DiscoverySourceReverseDNS+"|new-tv.home"].Active {
		t.Fatalf("new reverse-DNS evidence is not current: %+v", rows)
	}
	if byKey[models.DiscoverySourceSystemResolver+"|OLD-TV"].Active {
		t.Fatalf("missing system-resolver evidence remained current: %+v", rows)
	}
	if byKey[models.DiscoverySourceMDNS+"|Old-TV.local"].Active {
		t.Fatalf("missing mDNS evidence remained current: %+v", rows)
	}
}

func TestSuccessfulScanMovesMissingSSDPEvidenceToPrevious(t *testing.T) {
	setupScanRoutineTest(t)
	oldSSDP := ssdpDiscovery
	ssdpDiscovery = func(context.Context, []string) []discovery.SSDPObservation {
		return []discovery.SSDPObservation{
			{Address: "192.168.1.92", Kind: models.DiscoveryKindFriendlyName, Values: []string{"New Friendly Name"}},
		}
	}
	t.Cleanup(func() { ssdpDiscovery = oldSSDP })

	mac := "AA:BB:CC:DD:EE:B2"
	address := "192.168.1.92"
	gdb.Update("now", models.Host{
		Name: "TV", Iface: "eth0", IP: address, Mac: mac,
		Date: "2026-09-17 18:10:00", Known: 1, Now: 1, DeviceType: "tv",
	})

	seedEvidence(t, mac, address, models.DiscoverySourceSSDP, models.DiscoveryKindFriendlyName, "Old Friendly Name")
	seedEvidence(t, mac, address, models.DiscoverySourceSSDP, models.DiscoveryKindModel, "Old Model")

	if !processScanResult([]models.Host{
		{Iface: "eth0", IP: address, Mac: mac, Hw: "Sony", Date: "2026-09-17 18:15:00", Now: 1},
	}, true) {
		t.Fatal("successful scan was not applied")
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	byKey := evidenceByKey(rows)

	if byKey[models.DiscoverySourceSSDP+"|Old Friendly Name"].Active {
		t.Fatalf("old SSDP friendly name remained current: %+v", rows)
	}
	if !byKey[models.DiscoverySourceSSDP+"|New Friendly Name"].Active {
		t.Fatalf("new SSDP friendly name is not current: %+v", rows)
	}
	if byKey[models.DiscoverySourceSSDP+"|Old Model"].Active {
		t.Fatalf("missing SSDP model remained current: %+v", rows)
	}
}

func TestFailedScanDoesNotRefreshDiscoveryEvidenceScopes(t *testing.T) {
	setupScanRoutineTest(t)
	mac := "AA:BB:CC:DD:EE:B3"
	address := "192.168.1.93"
	seedEvidence(t, mac, address, models.DiscoverySourceMDNS, models.DiscoveryKindHostname, "still-current.local")

	if processScanResult([]models.Host{
		{Iface: "eth0", IP: address, Mac: mac, Hw: "Vendor", Date: "2026-09-17 18:20:00", Now: 1},
	}, false) {
		t.Fatal("failed scan was reported as applied")
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	byKey := evidenceByKey(rows)
	if !byKey[models.DiscoverySourceMDNS+"|still-current.local"].Active {
		t.Fatalf("failed scan changed current evidence: %+v", rows)
	}
}

func seedEvidence(t *testing.T, mac, address, source, kind, value string) {
	t.Helper()
	if err := gdb.RecordHostDiscoveryEvidence(mac, address, source, kind, []string{value}, "2026-09-17 18:00:00"); err != nil {
		t.Fatalf("seed discovery evidence: %v", err)
	}
}

func evidenceByKey(rows []models.HostDiscoveryEvidence) map[string]models.HostDiscoveryEvidence {
	result := make(map[string]models.HostDiscoveryEvidence, len(rows))
	for _, row := range rows {
		result[row.Source+"|"+row.Value] = row
	}
	return result
}
