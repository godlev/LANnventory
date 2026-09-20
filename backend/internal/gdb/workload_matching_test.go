package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestProxmoxImportAutoLinksSingleExactMACMatch(t *testing.T) {
	startSelectTestDB(t)

	hypervisor := models.Host{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:A0", DeviceType: "server", Now: 1}
	guest := models.Host{ID: 2, Name: "guest", Mac: "AA:BB:CC:DD:EE:A1", IP: "10.4.1.41", Now: 1}
	if err := UpdateWithError("now", hypervisor); err != nil {
		t.Fatalf("seed hypervisor: %v", err)
	}
	if err := UpdateWithError("now", guest); err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	state := testProxmoxSourceState("2026-09-19T12:00:00Z", "2026-09-19T12:01:00Z", "auto-link-1")
	err := ApplyProxmoxScriptImport(hypervisor.Mac, state, []models.InfrastructureWorkloadUpsert{{
		NativeID:     "201",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "guest",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceScriptImport,
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name: "net0",
			Mac:  guest.Mac,
		}},
	}})
	if err != nil {
		t.Fatalf("ApplyProxmoxScriptImport: %v", err)
	}

	records, err := SelectInfrastructureWorkloadsByHypervisorMAC(hypervisor.Mac)
	if err != nil {
		t.Fatalf("SelectInfrastructureWorkloadsByHypervisorMAC: %v", err)
	}
	if len(records) != 1 || records[0].Link == nil {
		t.Fatalf("imported workload link = %+v", records)
	}
	if records[0].Link.HostID != guest.ID ||
		records[0].Link.HostMac != guest.Mac ||
		records[0].Link.LinkSource != models.InfrastructureWorkloadLinkSourceExactMAC {
		t.Fatalf("exact MAC link = %+v", records[0].Link)
	}
}

func TestProxmoxImportDoesNotAutoLinkAddressOnlyCandidate(t *testing.T) {
	startSelectTestDB(t)

	hypervisor := models.Host{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:B0", DeviceType: "server", Now: 1}
	guest := models.Host{ID: 2, Name: "guest", Mac: "AA:BB:CC:DD:EE:B1", IP: "10.4.1.51", Now: 1}
	if err := UpdateWithError("now", hypervisor); err != nil {
		t.Fatalf("seed hypervisor: %v", err)
	}
	if err := UpdateWithError("now", guest); err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	state := testProxmoxSourceState("2026-09-19T13:00:00Z", "2026-09-19T13:01:00Z", "address-only")
	err := ApplyProxmoxScriptImport(hypervisor.Mac, state, []models.InfrastructureWorkloadUpsert{{
		NativeID:     "202",
		WorkloadType: models.InfrastructureWorkloadTypeContainer,
		Name:         "guest",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceScriptImport,
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			Mac:               "AA:BB:CC:DD:EE:BF",
			ConfiguredAddress: "10.4.1.51/24",
			ConfiguredNetwork: "10.4.1.0/24",
		}},
	}})
	if err != nil {
		t.Fatalf("ApplyProxmoxScriptImport: %v", err)
	}

	records, err := SelectInfrastructureWorkloadsByHypervisorMAC(hypervisor.Mac)
	if err != nil {
		t.Fatalf("SelectInfrastructureWorkloadsByHypervisorMAC: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("workloads = %+v", records)
	}
	if records[0].Link != nil {
		t.Fatalf("address-only candidate was auto-linked: %+v", records[0].Link)
	}
}

func TestProxmoxImportPreservesManualWorkloadLink(t *testing.T) {
	startSelectTestDB(t)

	hypervisor := models.Host{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:C0", DeviceType: "server", Now: 1}
	exactGuest := models.Host{ID: 2, Name: "exact", Mac: "AA:BB:CC:DD:EE:C1", Now: 1}
	manualGuest := models.Host{ID: 3, Name: "manual", Mac: "AA:BB:CC:DD:EE:C2", Now: 1}
	for _, host := range []models.Host{hypervisor, exactGuest, manualGuest} {
		if err := UpdateWithError("now", host); err != nil {
			t.Fatalf("seed host %+v: %v", host, err)
		}
	}

	state := testProxmoxSourceState("2026-09-19T14:00:00Z", "2026-09-19T14:01:00Z", "manual-1")
	input := []models.InfrastructureWorkloadUpsert{{
		NativeID:     "203",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "guest",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceScriptImport,
		Interfaces:   []models.InfrastructureWorkloadInterface{{Name: "net0", Mac: exactGuest.Mac}},
	}}
	if err := ApplyProxmoxScriptImport(hypervisor.Mac, state, input); err != nil {
		t.Fatalf("initial import: %v", err)
	}
	records, err := SelectInfrastructureWorkloadsByHypervisorMAC(hypervisor.Mac)
	if err != nil || len(records) != 1 {
		t.Fatalf("load workload: len=%d err=%v", len(records), err)
	}
	if _, err := SetInfrastructureWorkloadHostLink(
		records[0].Workload.ID,
		manualGuest,
		models.InfrastructureWorkloadLinkSourceManual,
		"2026-09-19T14:02:00Z",
	); err != nil {
		t.Fatalf("set manual link: %v", err)
	}

	state = testProxmoxSourceState("2026-09-19T14:10:00Z", "2026-09-19T14:11:00Z", "manual-2")
	if err := ApplyProxmoxScriptImport(hypervisor.Mac, state, input); err != nil {
		t.Fatalf("second import: %v", err)
	}
	records, err = SelectInfrastructureWorkloadsByHypervisorMAC(hypervisor.Mac)
	if err != nil {
		t.Fatalf("reload workload: %v", err)
	}
	if records[0].Link == nil ||
		records[0].Link.HostID != manualGuest.ID ||
		records[0].Link.LinkSource != models.InfrastructureWorkloadLinkSourceManual {
		t.Fatalf("manual link was changed by auto matching: %+v", records[0].Link)
	}
}

func TestProxmoxImportAmbiguousExactMACRemovesOnlyAutoLink(t *testing.T) {
	startSelectTestDB(t)

	hypervisor := models.Host{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:D0", DeviceType: "server", Now: 1}
	guestA := models.Host{ID: 2, Name: "guest-a", Mac: "AA:BB:CC:DD:EE:D1", Now: 1}
	if err := UpdateWithError("now", hypervisor); err != nil {
		t.Fatalf("seed hypervisor: %v", err)
	}
	if err := UpdateWithError("now", guestA); err != nil {
		t.Fatalf("seed guest A: %v", err)
	}

	input := []models.InfrastructureWorkloadUpsert{{
		NativeID:     "204",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "guest",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceScriptImport,
		Interfaces:   []models.InfrastructureWorkloadInterface{{Name: "net0", Mac: guestA.Mac}},
	}}
	state := testProxmoxSourceState("2026-09-19T15:00:00Z", "2026-09-19T15:01:00Z", "ambiguous-1")
	if err := ApplyProxmoxScriptImport(hypervisor.Mac, state, input); err != nil {
		t.Fatalf("initial import: %v", err)
	}
	records, err := SelectInfrastructureWorkloadsByHypervisorMAC(hypervisor.Mac)
	if err != nil || len(records) != 1 || records[0].Link == nil {
		t.Fatalf("initial auto link records=%+v err=%v", records, err)
	}

	guestB := models.Host{ID: 3, Name: "guest-b", Mac: guestA.Mac, Now: 1}
	if err := UpdateWithError("now", guestB); err != nil {
		t.Fatalf("seed duplicate-MAC guest B: %v", err)
	}

	state = testProxmoxSourceState("2026-09-19T15:10:00Z", "2026-09-19T15:11:00Z", "ambiguous-2")
	if err := ApplyProxmoxScriptImport(hypervisor.Mac, state, input); err != nil {
		t.Fatalf("ambiguous re-import: %v", err)
	}
	records, err = SelectInfrastructureWorkloadsByHypervisorMAC(hypervisor.Mac)
	if err != nil {
		t.Fatalf("reload workload: %v", err)
	}
	if records[0].Link != nil {
		t.Fatalf("ambiguous exact MAC retained automatic link: %+v", records[0].Link)
	}
}

func testProxmoxSourceState(collectedAt, importedAt, digest string) models.ProxmoxSourceState {
	return models.ProxmoxSourceState{
		Source:           models.InfrastructureWorkloadSourceScriptImport,
		SchemaVersion:    1,
		CollectorVersion: "1.0.0",
		CollectedAt:      collectedAt,
		Complete:         true,
		NodeHostname:     "pve",
		NodePVEVersion:   "pve-manager/9.2.10",
		NodeStatus:       "online",
		SnapshotDigest:   digest,
		ImportedAt:       importedAt,
	}
}


func TestWorkloadCandidateRejectionPersistsWithoutChangingLinkOrIdentity(t *testing.T) {
	startSelectTestDB(t)

	hypervisor := models.Host{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:E0", DeviceType: "server", Now: 1}
	candidateHost := models.Host{ID: 2, Name: "physical", Mac: "AA:BB:CC:DD:EE:E1", IP: "10.4.1.67", Now: 1}
	for _, host := range []models.Host{hypervisor, candidateHost} {
		if err := UpdateWithError("now", host); err != nil {
			t.Fatalf("seed host %+v: %v", host, err)
		}
	}
	record, err := UpsertInfrastructureWorkload(hypervisor.Mac, models.InfrastructureWorkloadUpsert{
		NativeID:     "104",
		WorkloadType: models.InfrastructureWorkloadTypeContainer,
		Name:         "actualbudget",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceScriptImport,
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			Mac:               "BC:24:11:14:02:A5",
			ConfiguredAddress: "10.4.1.67/24",
		}},
	}, "2026-09-20T20:00:00Z")
	if err != nil {
		t.Fatalf("seed workload: %v", err)
	}

	row, err := SetInfrastructureWorkloadCandidateRejection(
		record.Workload.ID,
		candidateHost,
		"fingerprint-1",
		"address",
		"2026-09-20T20:05:00Z",
	)
	if err != nil {
		t.Fatalf("SetInfrastructureWorkloadCandidateRejection: %v", err)
	}
	if row.HostID != candidateHost.ID || row.HostMac != candidateHost.Mac || row.EvidenceFingerprint != "fingerprint-1" {
		t.Fatalf("rejection = %+v", row)
	}

	rows, err := SelectInfrastructureWorkloadCandidateRejections(record.Workload.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("SelectInfrastructureWorkloadCandidateRejections rows=%+v err=%v", rows, err)
	}
	persisted, found, err := SelectInfrastructureWorkloadByID(record.Workload.ID)
	if err != nil || !found || persisted.Link != nil {
		t.Fatalf("rejection changed workload/link: record=%+v found=%v err=%v", persisted, found, err)
	}
	hosts := SelectByMAC("now", candidateHost.Mac)
	if len(hosts) != 1 || hosts[0].ID != candidateHost.ID {
		t.Fatalf("rejection changed Host identity: %+v", hosts)
	}

	if err := DeleteInfrastructureWorkloadCandidateRejection(record.Workload.ID, candidateHost.ID); err != nil {
		t.Fatalf("DeleteInfrastructureWorkloadCandidateRejection: %v", err)
	}
	rows, err = SelectInfrastructureWorkloadCandidateRejections(record.Workload.ID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("rejections after delete rows=%+v err=%v", rows, err)
	}
}
