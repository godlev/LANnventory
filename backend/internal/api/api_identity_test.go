package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestHostIdentityExposesAddressHistoryAndCrossMACReuse(t *testing.T) {
	router := setupTestRouter(t)

	macA := "AA:BB:CC:DD:EE:81"
	macB := "AA:BB:CC:DD:EE:82"
	if err := gdb.UpdateWithError("now", models.Host{
		Name: "Living Room TV", IP: "192.168.1.20", Mac: macA, Iface: "eth0",
		Date: "2026-09-17 10:10:00", Known: 1, Now: 1,
	}); err != nil {
		t.Fatalf("seed host A: %v", err)
	}
	hosts := gdb.SelectByMAC("now", macA)
	if len(hosts) != 1 {
		t.Fatalf("host A count = %d, want 1", len(hosts))
	}

	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: macA, IP: "192.168.1.10", Iface: "eth0", Date: "2026-09-17 10:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record first address: %v", err)
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: macA, IP: "192.168.1.20", Iface: "eth0", Date: "2026-09-17 10:10:00", Now: 1},
	}); err != nil {
		t.Fatalf("record second address: %v", err)
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: macB, IP: "192.168.1.20", Iface: "wlan0", Date: "2026-09-17 10:20:00", Now: 1},
	}); err != nil {
		t.Fatalf("record reused address: %v", err)
	}

	if err := gdb.RecordHostDiscoveryEvidence(macA, "192.168.1.20", models.DiscoverySourceScanner, models.DiscoveryKindVendor, []string{"Sony"}, "2026-09-17 10:10:00"); err != nil {
		t.Fatalf("record scanner evidence: %v", err)
	}
	if err := gdb.RecordHostDiscoveryEvidence(macA, "192.168.1.20", models.DiscoverySourceMDNS, models.DiscoveryKindHostname, []string{"Sony-TV.local"}, "2026-09-17 10:10:00"); err != nil {
		t.Fatalf("record mDNS evidence: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(hosts[0].ID)+"/identity", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("host identity status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var response HostIdentityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode host identity: %v", err)
	}
	if response.Mac != macA {
		t.Fatalf("response MAC = %q, want %q", response.Mac, macA)
	}
	if len(response.Addresses) != 2 {
		t.Fatalf("addresses = %+v, want 2 retained addresses", response.Addresses)
	}
	if len(response.DataSources) != 2 || response.DataSources[0] != models.DiscoverySourceMDNS || response.DataSources[1] != models.DiscoverySourceScanner {
		t.Fatalf("dataSources = %v, want [mdns scanner]", response.DataSources)
	}

	var reused *HostIdentityAddress
	for i := range response.Addresses {
		if response.Addresses[i].Address == "192.168.1.20" {
			reused = &response.Addresses[i]
			break
		}
	}
	if reused == nil {
		t.Fatal("reused address 192.168.1.20 missing")
	}
	if reused.FirstSeen != "2026-09-17 10:10:00" || reused.LastSeen != "2026-09-17 10:10:00" || reused.Active {
		t.Fatalf("host A reused-address observation = %+v", reused)
	}
	if len(reused.MacHistory) != 2 {
		t.Fatalf("MAC history = %+v, want A and B", reused.MacHistory)
	}

	seen := map[string]AddressMACObservation{}
	for _, observation := range reused.MacHistory {
		seen[observation.Mac] = observation
	}
	if got := seen[macA]; got.FirstSeen != "2026-09-17 10:10:00" || got.LastSeen != "2026-09-17 10:10:00" || got.Active {
		t.Fatalf("MAC A history = %+v", got)
	}
	if got := seen[macB]; got.FirstSeen != "2026-09-17 10:20:00" || got.LastSeen != "2026-09-17 10:20:00" || !got.Active {
		t.Fatalf("MAC B history = %+v", got)
	}
}

func TestAddressIdentityReturnsAllObservedMACs(t *testing.T) {
	router := setupTestRouter(t)
	address := "192.168.1.30"
	macA := "AA:BB:CC:DD:EE:83"
	macB := "AA:BB:CC:DD:EE:84"

	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: macA, IP: address, Iface: "eth0", Date: "2026-09-17 11:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record MAC A: %v", err)
	}
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: macB, IP: address, Iface: "eth0", Date: "2026-09-17 12:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record MAC B: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/identity/address?address="+url.QueryEscape(address), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("address identity status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var response AddressIdentityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode address identity: %v", err)
	}
	if response.Address != address || len(response.MacHistory) != 2 {
		t.Fatalf("address identity = %+v", response)
	}
}

func TestIdentityEndpointsRejectInvalidInput(t *testing.T) {
	router := setupTestRouter(t)

	for _, path := range []string{
		"/api/host/not-a-number/identity",
		"/api/host/999/identity",
		"/api/identity/address?address=not-an-ip",
		"/api/identity/address",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400; body: %s", path, rec.Code, rec.Body.String())
		}
	}
}
