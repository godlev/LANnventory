package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestDeleteCurrentHostRetainsIdentityObservationHistory(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:A0"
	address := "10.4.1.80"

	if err := UpdateWithError("now", models.Host{
		Name: "temporary-device", Iface: "eth0", IP: address, Mac: mac,
		Date: "2026-09-17 17:00:00", Known: 1, Now: 1, DeviceType: "iot",
	}); err != nil {
		t.Fatalf("seed current host: %v", err)
	}
	hosts := SelectByMAC("now", mac)
	if len(hosts) != 1 {
		t.Fatalf("seeded hosts = %+v", hosts)
	}
	host := hosts[0]

	if err := RecordHostAddressObservations([]models.Host{
		{Mac: mac, IP: address, Iface: "eth0", Date: "2026-09-17 17:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("RecordHostAddressObservations: %v", err)
	}
	if err := RecordHostDiscoveryEvidence(mac, address, models.DiscoverySourceMDNS, models.DiscoveryKindHostname, []string{"device.local"}, "2026-09-17 17:00:00"); err != nil {
		t.Fatalf("RecordHostDiscoveryEvidence: %v", err)
	}

	connectivity := models.NewHostEvent(host, models.EventOnline, "", "")
	connectivity.Date = "2026-09-17 17:00:00"
	if err := AddEvent(connectivity); err != nil {
		t.Fatalf("AddEvent online: %v", err)
	}
	deviceChange := models.NewHostEvent(host, models.EventDeviceTypeChanged, "", "iot")
	deviceChange.Date = "2026-09-17 17:01:00"
	if err := AddEvent(deviceChange); err != nil {
		t.Fatalf("AddEvent device type: %v", err)
	}

	if err := DeleteCurrentHostWithMetadata(host); err != nil {
		t.Fatalf("DeleteCurrentHostWithMetadata: %v", err)
	}
	if remaining := SelectByMAC("now", mac); len(remaining) != 0 {
		t.Fatalf("deleted current host still present: %+v", remaining)
	}

	addresses, err := SelectHostAddressesByMAC(mac)
	if err != nil || len(addresses) != 1 || addresses[0].Address != address {
		t.Fatalf("retained address history = %+v err=%v", addresses, err)
	}
	evidence, err := SelectHostDiscoveryEvidenceByMAC(mac)
	if err != nil || len(evidence) != 1 || evidence[0].Value != "device.local" {
		t.Fatalf("retained discovery evidence = %+v err=%v", evidence, err)
	}

	events, ok := SelectEvents(10, "")
	if !ok {
		t.Fatal("SelectEvents failed")
	}
	if len(events) != 1 || events[0].EventType != string(models.EventOnline) || events[0].Mac != mac {
		t.Fatalf("delete event retention = %+v, want only immutable connectivity observation", events)
	}
}
