package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identification"
	"github.com/godlev/LANnventory/internal/models"
)

func TestHostIdentificationAggregatesRetainedEvidenceAndExactInterfaceMAC(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		Name:  "",
		IP:    "10.4.1.27",
		Mac:   "AA:BB:CC:DD:FA:27",
		Iface: "eth0",
		Known: 0,
		Now:   1,
	})
	now := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)

	if err := gdb.RecordHostDiscoveryEvidence(
		host.Mac,
		host.IP,
		models.DiscoverySourceMDNS,
		models.DiscoveryKindHostname,
		[]string{"media-box.local"},
		now,
	); err != nil {
		t.Fatalf("RecordHostDiscoveryEvidence: %v", err)
	}
	if _, err := gdb.UpsertService(models.Service{
		Mac:            host.Mac,
		Address:        host.IP,
		Protocol:       string(models.ServiceProtocolTCP),
		Port:           554,
		State:          string(models.ServiceStateOpen),
		LastChecked:    now,
		LastDetected:   now,
		FirstDetected:  now,
		StateChangedAt: now,
		ServiceHint:    "RTSP",
		LastScanSource: "manual",
	}); err != nil {
		t.Fatalf("UpsertService: %v", err)
	}
	if _, err := gdb.UpsertInfrastructureWorkload(
		"AA:BB:CC:DD:FA:01",
		models.InfrastructureWorkloadUpsert{
			NativeID:     "119",
			WorkloadType: models.InfrastructureWorkloadTypeVM,
			Name:         "ubuntu-plex-immich",
			Status:       models.InfrastructureWorkloadStatusRunning,
			Source:       models.InfrastructureWorkloadSourceManual,
			Interfaces: []models.InfrastructureWorkloadInterface{{
				Name: "net0",
				Mac:  host.Mac,
			}},
		},
		now,
	); err != nil {
		t.Fatalf("UpsertInfrastructureWorkload: %v", err)
	}

	rec := getPath(router, "/api/host/"+strconv.Itoa(host.ID)+"/identification")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}

	var response HostIdentificationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if response.HostID != host.ID || response.Known {
		t.Fatalf("host envelope = %+v", response)
	}
	if response.Assessment.State != "suggested" {
		t.Fatalf("assessment state = %q, response=%+v", response.Assessment.State, response.Assessment)
	}
	if response.Assessment.SuggestedName == nil ||
		response.Assessment.SuggestedName.Value != "ubuntu-plex-immich" ||
		response.Assessment.SuggestedName.Confidence != identification.ConfidenceHigh {
		t.Fatalf("name suggestion = %+v", response.Assessment.SuggestedName)
	}
	if response.Assessment.SuggestedDeviceType == nil ||
		response.Assessment.SuggestedDeviceType.Value != string(models.DeviceTypeVirtualMachine) ||
		response.Assessment.SuggestedDeviceType.Confidence != identification.ConfidenceHigh {
		t.Fatalf("device type suggestion = %+v", response.Assessment.SuggestedDeviceType)
	}
	if len(response.Assessment.Cautions) == 0 {
		t.Fatalf("expected service-context caution, response=%+v", response.Assessment)
	}

	workloads := identificationSourceByName(t, response.Sources, "workloads")
	if !workloads.Available || workloads.Total != 1 || workloads.Included != 1 || workloads.Truncated {
		t.Fatalf("workload source = %+v", workloads)
	}
	if len(response.Actions) != 2 || !response.Actions[1].Available {
		t.Fatalf("actions = %+v", response.Actions)
	}
}

func TestHostIdentificationTreatsOldOpenServiceAsStale(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		IP:    "10.4.1.50",
		Mac:   "AA:BB:CC:DD:FA:50",
		Iface: "eth0",
		Known: 0,
		Now:   1,
	})
	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)

	if _, err := gdb.UpsertService(models.Service{
		Mac:            host.Mac,
		Address:        host.IP,
		Protocol:       string(models.ServiceProtocolTCP),
		Port:           554,
		State:          string(models.ServiceStateOpen),
		LastChecked:    old,
		LastDetected:   old,
		FirstDetected:  old,
		StateChangedAt: old,
		LastScanSource: "manual",
	}); err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	rec := getPath(router, "/api/host/"+strconv.Itoa(host.ID)+"/identification")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var response HostIdentificationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if response.Assessment.SuggestedDeviceType != nil {
		t.Fatalf("stale service created type suggestion: %+v", response.Assessment.SuggestedDeviceType)
	}
	if response.Assessment.State != "needs-investigation" {
		t.Fatalf("state = %q", response.Assessment.State)
	}

	foundStale := false
	for _, evidence := range response.Assessment.Evidence {
		if evidence.Category == "service" && evidence.Freshness == identification.FreshnessStale {
			foundStale = true
		}
	}
	if !foundStale {
		t.Fatalf("stale service evidence missing: %+v", response.Assessment.Evidence)
	}
}

func TestHostIdentificationWarnsAboutActiveAddressReuse(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		IP:    "10.4.1.67",
		Mac:   "AA:BB:CC:DD:FA:67",
		Iface: "eth0",
		Known: 0,
		Now:   1,
	})
	now := time.Now().UTC().Format(time.RFC3339)
	if err := gdb.RecordHostAddressObservations([]models.Host{
		{Mac: host.Mac, IP: host.IP, Iface: "eth0", Date: now, Now: 1},
		{Mac: "AA:BB:CC:DD:FA:68", IP: host.IP, Iface: "eth0", Date: now, Now: 1},
	}); err != nil {
		t.Fatalf("RecordHostAddressObservations: %v", err)
	}

	rec := getPath(router, "/api/host/"+strconv.Itoa(host.ID)+"/identification")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var response HostIdentificationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	found := false
	for _, warning := range response.Warnings {
		if warning.Code == "active-ip-reuse" && warning.Severity == "caution" {
			found = true
		}
	}
	if !found {
		t.Fatalf("active reuse warning missing: %+v", response.Warnings)
	}
	addresses := identificationSourceByName(t, response.Sources, "address-history")
	if !addresses.Available || addresses.Total != 1 || addresses.Included != 1 {
		t.Fatalf("address source = %+v", addresses)
	}
}

func TestHostIdentificationBoundsRetainedServiceResponse(t *testing.T) {
	router := setupTestRouter(t)
	host := seedHost(t, models.Host{
		IP:    "10.4.1.80",
		Mac:   "AA:BB:CC:DD:FA:80",
		Iface: "eth0",
		Known: 0,
		Now:   1,
	})
	now := time.Now().UTC().Format(time.RFC3339)

	for i := 0; i < identificationMaxServices+6; i++ {
		if _, err := gdb.UpsertService(models.Service{
			Mac:            host.Mac,
			Address:        host.IP,
			Protocol:       string(models.ServiceProtocolTCP),
			Port:           10000 + i,
			State:          string(models.ServiceStateOpen),
			LastChecked:    now,
			LastDetected:   now,
			FirstDetected:  now,
			StateChangedAt: now,
			LastScanSource: "manual",
		}); err != nil {
			t.Fatalf("UpsertService(%d): %v", i, err)
		}
	}

	rec := getPath(router, "/api/host/"+strconv.Itoa(host.ID)+"/identification")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var response HostIdentificationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	services := identificationSourceByName(t, response.Sources, "services")
	if !services.Available ||
		services.Total != identificationMaxServices+6 ||
		services.Included != identificationMaxServices ||
		!services.Truncated {
		t.Fatalf("service source = %+v", services)
	}

	serviceEvidence := 0
	for _, evidence := range response.Assessment.Evidence {
		if evidence.Category == "service" {
			serviceEvidence++
		}
	}
	if serviceEvidence != identificationMaxServices {
		t.Fatalf("service evidence count = %d, want %d", serviceEvidence, identificationMaxServices)
	}
}

func TestHostIdentificationRejectsInvalidHost(t *testing.T) {
	router := setupTestRouter(t)
	for _, path := range []string{
		"/api/host/not-a-number/identification",
		"/api/host/999999/identification",
	} {
		rec := getPath(router, path)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d; body: %s", path, rec.Code, rec.Body.String())
		}
	}
}

func identificationSourceByName(t *testing.T, sources []IdentificationSourceStatus, name string) IdentificationSourceStatus {
	t.Helper()
	for _, source := range sources {
		if source.Source == name {
			return source
		}
	}
	t.Fatalf("source %q missing from %+v", name, sources)
	return IdentificationSourceStatus{}
}
