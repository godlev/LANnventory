package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestHostIdentityCandidatesIncludeRetainedHistoricalMAC(t *testing.T) {
	router := setupTestRouter(t)

	targetMAC := "02:AA:BB:CC:DD:10"
	historicalMAC := "06:AA:BB:CC:DD:20"
	if err := gdb.UpdateWithError("now", models.Host{
		Name: "My Phone", IP: "10.4.1.51", Mac: targetMAC, Iface: "eth0",
		Date: "2026-09-17 16:00:00", Now: 1, DeviceType: "phone",
	}); err != nil {
		t.Fatalf("seed target host: %v", err)
	}
	hosts := gdb.SelectByMAC("now", targetMAC)
	if len(hosts) != 1 {
		t.Fatalf("target host count = %d, want 1", len(hosts))
	}

	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: historicalMAC, IP: "10.4.1.50", Iface: "eth0", Date: "2026-09-17 15:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record historical MAC address: %v", err)
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: targetMAC, IP: "10.4.1.51", Iface: "eth0", Date: "2026-09-17 16:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record target address: %v", err)
	}

	for _, item := range []struct {
		mac     string
		address string
		date    string
	}{
		{historicalMAC, "10.4.1.50", "2026-09-17 15:00:00"},
		{targetMAC, "10.4.1.51", "2026-09-17 16:00:00"},
	} {
		if err := gdb.RecordHostDiscoveryEvidence(item.mac, item.address, models.DiscoverySourceMDNS, models.DiscoveryKindHostname, []string{"miros-phone.local"}, item.date); err != nil {
			t.Fatalf("record hostname evidence for %s: %v", item.mac, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(hosts[0].ID)+"/identity/candidates", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var response HostIdentityCandidatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode candidates: %v", err)
	}
	if response.Mac != targetMAC || len(response.Candidates) != 1 {
		t.Fatalf("candidate response = %+v", response)
	}
	candidate := response.Candidates[0]
	if candidate.Mac != historicalMAC {
		t.Fatalf("candidate MAC = %q, want %q", candidate.Mac, historicalMAC)
	}
	if candidate.Exists || candidate.HostID != 0 {
		t.Fatalf("historical candidate should not require current host row: %+v", candidate)
	}
	if candidate.Confidence != "medium" {
		t.Fatalf("candidate confidence = %q score=%d, want medium", candidate.Confidence, candidate.Score)
	}
}

func TestHostIdentityCandidatesDoNotTreatDHCPReuseAsDeviceMatch(t *testing.T) {
	router := setupTestRouter(t)

	targetMAC := "00:AA:BB:CC:DD:31"
	otherMAC := "00:AA:BB:CC:DD:32"
	if err := gdb.UpdateWithError("now", models.Host{
		IP: "10.4.1.80", Mac: targetMAC, Iface: "eth0", Date: "2026-09-17 14:00:00", Now: 1,
	}); err != nil {
		t.Fatalf("seed target host: %v", err)
	}
	hosts := gdb.SelectByMAC("now", targetMAC)
	if len(hosts) != 1 {
		t.Fatalf("target host count = %d, want 1", len(hosts))
	}

	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: targetMAC, IP: "10.4.1.99", Iface: "eth0", Date: "2026-09-17 13:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record target reused address: %v", err)
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: otherMAC, IP: "10.4.1.99", Iface: "eth0", Date: "2026-09-17 14:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record other reused address: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(hosts[0].ID)+"/identity/candidates", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var response HostIdentityCandidatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode candidates: %v", err)
	}
	if len(response.Candidates) != 0 {
		t.Fatalf("plain DHCP/IP reuse must not suggest same device: %+v", response.Candidates)
	}
}
