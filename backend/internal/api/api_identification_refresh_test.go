package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/discovery"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestIdentificationNameRefreshUpdatesOnlyReturnedHostnameScopes(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "",
		IP:    "10.4.1.77",
		Mac:   "AA:BB:CC:DD:FA:77",
		Iface: "eth0",
		Date:  "2026-09-26 21:00:00",
		Known: 0,
		Now:   1,
	})

	oldObservedAt := "2026-09-26 20:00:00"
	if err := gdb.RecordHostDiscoveryEvidence(
		host.Mac,
		host.IP,
		models.DiscoverySourceSSDP,
		models.DiscoveryKindFriendlyName,
		[]string{"Old SSDP Name"},
		oldObservedAt,
	); err != nil {
		t.Fatalf("seed SSDP evidence: %v", err)
	}
	if err := gdb.RecordHostDiscoveryEvidence(
		host.Mac,
		host.IP,
		models.DiscoverySourceSystemResolver,
		models.DiscoveryKindHostname,
		[]string{"retained-system-name"},
		oldObservedAt,
	); err != nil {
		t.Fatalf("seed system resolver evidence: %v", err)
	}

	originalDiscovery := identificationHostnameDiscovery
	originalNow := identificationRefreshNow
	identificationHostnameDiscovery = func(_ context.Context, address string) []discovery.HostnameObservation {
		if address != host.IP {
			t.Fatalf("discovery address = %q, want %q", address, host.IP)
		}
		return []discovery.HostnameObservation{
			{Source: models.DiscoverySourceReverseDNS, Values: []string{"printer.home"}},
			{Source: models.DiscoverySourceMDNS, Values: []string{"printer.local"}},
			{Source: models.DiscoverySourceSSDP, Values: []string{"must-not-be-written"}},
		}
	}
	fixedNow := time.Date(2026, 9, 27, 1, 15, 0, 0, time.UTC)
	identificationRefreshNow = func() time.Time { return fixedNow }
	t.Cleanup(func() {
		identificationHostnameDiscovery = originalDiscovery
		identificationRefreshNow = originalNow
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/identification/refresh-names", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response IdentificationNameRefreshResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if response.RefreshedAt != fixedNow.Format(models.HostEventDateLayout) {
		t.Fatalf("refreshedAt = %q", response.RefreshedAt)
	}
	if len(response.Sources) != 2 {
		t.Fatalf("sources = %+v, want reverse DNS and mDNS only", response.Sources)
	}

	current := gdb.SelectByID(host.ID)
	if current.ID != host.ID || current.IP != host.IP || current.Mac != host.Mac || current.Now != host.Now || current.Date != host.Date {
		t.Fatalf("targeted name refresh changed host state: before=%+v after=%+v", host, current)
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}

	type key struct {
		source string
		kind   string
		value  string
	}
	byKey := make(map[key]models.HostDiscoveryEvidence)
	for _, row := range rows {
		byKey[key{row.Source, row.Kind, row.Value}] = row
	}

	reverse := byKey[key{models.DiscoverySourceReverseDNS, models.DiscoveryKindHostname, "printer.home"}]
	if !reverse.Active || reverse.LastSeen != response.RefreshedAt {
		t.Fatalf("reverse evidence = %+v", reverse)
	}
	mdns := byKey[key{models.DiscoverySourceMDNS, models.DiscoveryKindHostname, "printer.local"}]
	if !mdns.Active || mdns.LastSeen != response.RefreshedAt {
		t.Fatalf("mDNS evidence = %+v", mdns)
	}
	system := byKey[key{models.DiscoverySourceSystemResolver, models.DiscoveryKindHostname, "retained-system-name"}]
	if !system.Active || system.LastSeen != oldObservedAt {
		t.Fatalf("omitted system resolver evidence should remain untouched: %+v", system)
	}
	ssdp := byKey[key{models.DiscoverySourceSSDP, models.DiscoveryKindFriendlyName, "Old SSDP Name"}]
	if !ssdp.Active || ssdp.LastSeen != oldObservedAt {
		t.Fatalf("SSDP evidence changed during targeted name refresh: %+v", ssdp)
	}
	if _, exists := byKey[key{models.DiscoverySourceSSDP, models.DiscoveryKindHostname, "must-not-be-written"}]; exists {
		t.Fatal("targeted name refresh wrote an SSDP hostname observation")
	}
}

func TestIdentificationNameRefreshRejectsBindingChangeBeforeWrite(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		IP:    "10.4.1.88",
		Mac:   "AA:BB:CC:DD:FA:88",
		Iface: "eth0",
		Known: 0,
		Now:   1,
	})

	originalDiscovery := identificationHostnameDiscovery
	identificationHostnameDiscovery = func(_ context.Context, address string) []discovery.HostnameObservation {
		changed := host
		changed.IP = "10.4.1.89"
		if err := gdb.UpdateWithError("now", changed); err != nil {
			t.Fatalf("UpdateWithError: %v", err)
		}
		return []discovery.HostnameObservation{
			{Source: models.DiscoverySourceMDNS, Values: []string{"stale.local"}},
		}
	}
	t.Cleanup(func() {
		identificationHostnameDiscovery = originalDiscovery
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/identification/refresh-names", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostDiscoveryEvidenceByMAC: %v", err)
	}
	for _, row := range rows {
		if row.Value == "stale.local" {
			t.Fatalf("stale targeted refresh evidence was persisted: %+v", row)
		}
	}
}

func TestIdentificationNameRefreshRequiresCurrentIP(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Mac:   "AA:BB:CC:DD:FA:99",
		Iface: "eth0",
		Known: 0,
		Now:   0,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/host/"+strconv.Itoa(host.ID)+"/identification/refresh-names", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
