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

func TestHostIdentityGroupReturnsTransitiveConfirmedMembers(t *testing.T) {
	router := setupTestRouter(t)
	macA := "02:AA:BB:CC:F0:01"
	macB := "06:AA:BB:CC:F0:02"
	macC := "0A:AA:BB:CC:F0:03"

	if err := gdb.UpdateWithError("now", models.Host{
		Name: "Phone current", Mac: macA, IP: "10.4.1.51", Iface: "eth0",
		Date: "2026-09-17 20:00:00", Now: 1, DeviceType: "phone",
	}); err != nil {
		t.Fatalf("seed host A: %v", err)
	}
	if err := gdb.UpdateWithError("now", models.Host{
		Name: "Phone old identity", Mac: macB, IP: "10.4.1.52", Iface: "eth0",
		Date: "2026-09-17 19:30:00", Now: 0, DeviceType: "phone",
	}); err != nil {
		t.Fatalf("seed host B: %v", err)
	}
	hostsA := gdb.SelectByMAC("now", macA)
	if len(hostsA) != 1 {
		t.Fatalf("host A count = %d, want 1", len(hostsA))
	}

	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: macA, IP: "10.4.1.51", Iface: "eth0", Date: "2026-09-17 20:00:00", Now: 1},
		{Mac: macB, IP: "10.4.1.52", Iface: "eth0", Date: "2026-09-17 19:30:00", Now: 1},
		{Mac: macC, IP: "10.4.1.53", Iface: "eth0", Date: "2026-09-17 19:00:00", Now: 1},
	}); err != nil {
		t.Fatalf("record group addresses: %v", err)
	}
	if _, err := gdb.SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationConfirmed, "2026-09-17 20:01:00"); err != nil {
		t.Fatalf("confirm A/B: %v", err)
	}
	if _, err := gdb.SetIdentityCorrelationDecision(macB, macC, models.IdentityCorrelationConfirmed, "2026-09-17 20:02:00"); err != nil {
		t.Fatalf("confirm B/C: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(hostsA[0].ID)+"/identity/group", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("group status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var response HostIdentityGroupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	if response.Mac != macA || !response.Confirmed || len(response.Members) != 3 {
		t.Fatalf("group response = %+v", response)
	}
	if response.Members[0].Mac != macA {
		t.Fatalf("viewed MAC should be first, members = %+v", response.Members)
	}

	members := make(map[string]ConfirmedIdentityGroupMember, len(response.Members))
	for _, member := range response.Members {
		members[member.Mac] = member
	}
	if !members[macA].Exists || !members[macA].Active || members[macA].Name != "Phone current" {
		t.Fatalf("member A = %+v", members[macA])
	}
	if !members[macB].Exists || members[macB].Active {
		t.Fatalf("member B = %+v", members[macB])
	}
	if members[macC].Exists || members[macC].Active || len(members[macC].Addresses) != 1 || members[macC].Addresses[0] != "10.4.1.53" {
		t.Fatalf("historical member C = %+v", members[macC])
	}
}

func TestHostIdentityGroupReturnsSingletonWithoutConfirmedRelationships(t *testing.T) {
	router := setupTestRouter(t)
	mac := "02:AA:BB:CC:F0:11"
	if err := gdb.UpdateWithError("now", models.Host{
		Mac: mac, IP: "10.4.1.61", Iface: "eth0", Date: "2026-09-17 20:10:00", Now: 1,
	}); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	hosts := gdb.SelectByMAC("now", mac)
	if len(hosts) != 1 {
		t.Fatalf("host count = %d, want 1", len(hosts))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/host/"+strconv.Itoa(hosts[0].ID)+"/identity/group", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("group status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var response HostIdentityGroupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	if response.Confirmed || len(response.Members) != 1 || response.Members[0].Mac != mac {
		t.Fatalf("singleton group = %+v", response)
	}
}
