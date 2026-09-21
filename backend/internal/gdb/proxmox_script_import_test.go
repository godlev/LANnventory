package gdb

import (
	"errors"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestApplyProxmoxScriptImportCreatesUpdatesAndRetiresAtomically(t *testing.T) {
	startSelectTestDB(t)
	hypervisorMac := "AA:BB:CC:DD:EE:80"

	state := models.ProxmoxSourceState{
		Source:           models.InfrastructureWorkloadSourceScriptImport,
		SchemaVersion:    1,
		CollectorVersion: "1.0.0",
		CollectedAt:      "2026-09-19T09:00:00Z",
		Complete:         true,
		NodeHostname:     "pve-1",
		NodePVEVersion:   "pve-manager/9.2.10",
		NodeStatus:       "online",
		SnapshotDigest:   "digest-1",
		ImportedAt:       "2026-09-19T09:01:00Z",
	}
	workloads := []models.InfrastructureWorkloadUpsert{
		{
			NativeID:     "119",
			WorkloadType: models.InfrastructureWorkloadTypeVM,
			Name:         "media",
			Status:       models.InfrastructureWorkloadStatusRunning,
			Source:       models.InfrastructureWorkloadSourceScriptImport,
			Interfaces: []models.InfrastructureWorkloadInterface{{
				Name:              "net0",
				Mac:               "AA:BB:CC:00:01:19",
				Bridge:            "vmbr0",
				ConfiguredAddress: "10.4.1.19/24",
				ConfiguredNetwork: "10.4.1.0/24",
			}},
		},
		{
			NativeID:     "127",
			WorkloadType: models.InfrastructureWorkloadTypeContainer,
			Name:         "yubal",
			Status:       models.InfrastructureWorkloadStatusStopped,
			Source:       models.InfrastructureWorkloadSourceScriptImport,
		},
	}
	if err := ApplyProxmoxScriptImport(hypervisorMac, state, workloads); err != nil {
		t.Fatalf("ApplyProxmoxScriptImport first: %v", err)
	}

	records, err := SelectInfrastructureWorkloadsByHypervisorMAC(hypervisorMac)
	if err != nil {
		t.Fatalf("SelectInfrastructureWorkloadsByHypervisorMAC: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("workload count = %d, want 2", len(records))
	}
	for _, record := range records {
		if record.Workload.Source != models.InfrastructureWorkloadSourceScriptImport || record.Workload.RetiredAt != "" {
			t.Fatalf("imported workload = %+v", record.Workload)
		}
	}

	persistedState, found, err := SelectProxmoxSourceState(hypervisorMac, models.InfrastructureWorkloadSourceScriptImport)
	if err != nil || !found {
		t.Fatalf("SelectProxmoxSourceState found=%v err=%v", found, err)
	}
	if persistedState.NodeHostname != "pve-1" || persistedState.SnapshotDigest != "digest-1" {
		t.Fatalf("source state = %+v", persistedState)
	}

	state.CollectedAt = "2026-09-19T10:00:00Z"
	state.ImportedAt = "2026-09-19T10:01:00Z"
	state.SnapshotDigest = "digest-2"
	workloads = []models.InfrastructureWorkloadUpsert{{
		NativeID:     "119",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "media-renamed",
		Status:       models.InfrastructureWorkloadStatusStopped,
		Source:       models.InfrastructureWorkloadSourceScriptImport,
		Interfaces:   []models.InfrastructureWorkloadInterface{},
	}}
	if err := ApplyProxmoxScriptImport(hypervisorMac, state, workloads); err != nil {
		t.Fatalf("ApplyProxmoxScriptImport second: %v", err)
	}

	records, err = SelectInfrastructureWorkloadsByHypervisorMAC(hypervisorMac)
	if err != nil {
		t.Fatalf("reload workloads: %v", err)
	}
	var media, retired *models.InfrastructureWorkloadRecord
	for i := range records {
		switch records[i].Workload.NativeID {
		case "119":
			media = &records[i]
		case "127":
			retired = &records[i]
		}
	}
	if media == nil || media.Workload.Name != "media-renamed" || media.Workload.Status != models.InfrastructureWorkloadStatusStopped ||
		media.Workload.LastSeen != state.CollectedAt || media.Workload.RetiredAt != "" || len(media.Interfaces) != 0 {
		t.Fatalf("updated media = %+v", media)
	}
	if retired == nil || retired.Workload.RetiredAt != state.CollectedAt {
		t.Fatalf("retired workload = %+v", retired)
	}
}

func TestApplyProxmoxScriptImportManualConflictRollsBackWholeSnapshot(t *testing.T) {
	startSelectTestDB(t)
	hypervisorMac := "AA:BB:CC:DD:EE:81"

	if _, err := UpsertInfrastructureWorkload(hypervisorMac, models.InfrastructureWorkloadUpsert{
		NativeID:     "121",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "manual",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceManual,
	}, "2026-09-19T08:00:00Z"); err != nil {
		t.Fatalf("seed manual workload: %v", err)
	}

	state := models.ProxmoxSourceState{
		Source:           models.InfrastructureWorkloadSourceScriptImport,
		SchemaVersion:    1,
		CollectorVersion: "1.0.0",
		CollectedAt:      "2026-09-19T09:00:00Z",
		Complete:         true,
		NodeHostname:     "pve",
		NodePVEVersion:   "pve-manager/9.2.10",
		NodeStatus:       "online",
		SnapshotDigest:   "digest-conflict",
		ImportedAt:       "2026-09-19T09:01:00Z",
	}
	err := ApplyProxmoxScriptImport(hypervisorMac, state, []models.InfrastructureWorkloadUpsert{
		{
			NativeID:     "120",
			WorkloadType: models.InfrastructureWorkloadTypeVM,
			Name:         "would-be-created",
			Status:       models.InfrastructureWorkloadStatusRunning,
			Source:       models.InfrastructureWorkloadSourceScriptImport,
		},
		{
			NativeID:     "121",
			WorkloadType: models.InfrastructureWorkloadTypeVM,
			Name:         "collision",
			Status:       models.InfrastructureWorkloadStatusStopped,
			Source:       models.InfrastructureWorkloadSourceScriptImport,
		},
	})
	if !errors.Is(err, ErrInfrastructureWorkloadSourceConflict) {
		t.Fatalf("conflict error = %v", err)
	}

	records, err := SelectInfrastructureWorkloadsByHypervisorMAC(hypervisorMac)
	if err != nil {
		t.Fatalf("reload workloads: %v", err)
	}
	if len(records) != 1 || records[0].Workload.Source != models.InfrastructureWorkloadSourceManual || records[0].Workload.Name != "manual" {
		t.Fatalf("transaction did not roll back: %+v", records)
	}
	if _, found, err := SelectProxmoxSourceState(hypervisorMac, models.InfrastructureWorkloadSourceScriptImport); err != nil || found {
		t.Fatalf("source state persisted after rollback found=%v err=%v", found, err)
	}
}

func TestApplyProxmoxScriptImportRejectsIncompleteSnapshot(t *testing.T) {
	startSelectTestDB(t)
	err := ApplyProxmoxScriptImport("AA:BB:CC:DD:EE:82", models.ProxmoxSourceState{
		Source:         models.InfrastructureWorkloadSourceScriptImport,
		Complete:       false,
		CollectedAt:    "2026-09-19T09:00:00Z",
		ImportedAt:     "2026-09-19T09:01:00Z",
		SnapshotDigest: "digest",
	}, nil)
	if err == nil {
		t.Fatal("incomplete snapshot unexpectedly applied")
	}
}
