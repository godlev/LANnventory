package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestInfrastructureWorkloadSummariesExcludeRetiredInventory(t *testing.T) {
	startSelectTestDB(t)

	const hypervisorMac = "AA:BB:CC:DD:F3:10"
	for _, input := range []models.InfrastructureWorkloadUpsert{
		{NativeID: "100", WorkloadType: models.InfrastructureWorkloadTypeVM, Status: models.InfrastructureWorkloadStatusStopped},
		{NativeID: "101", WorkloadType: models.InfrastructureWorkloadTypeVM, Status: models.InfrastructureWorkloadStatusRunning},
		{NativeID: "102", WorkloadType: models.InfrastructureWorkloadTypeContainer, Status: models.InfrastructureWorkloadStatusStopped},
	} {
		if _, err := UpsertInfrastructureWorkload(hypervisorMac, input, "2026-09-21T06:00:00Z"); err != nil {
			t.Fatalf("UpsertInfrastructureWorkload: %v", err)
		}
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		t.Fatalf("acquireDB: %v", err)
	}
	if err := activeDB.Table(infrastructureWorkloadsTable).
		Where(`"HYPERVISOR_MAC" = ? AND "NATIVE_ID" = ? AND "WORKLOAD_TYPE" = ?`, hypervisorMac, "101", models.InfrastructureWorkloadTypeVM).
		Update("RETIRED_AT", "2026-09-21T06:10:00Z").Error; err != nil {
		release()
		t.Fatalf("retire workload: %v", err)
	}
	release()

	summaries, err := SelectInfrastructureWorkloadSummaries()
	if err != nil {
		t.Fatalf("SelectInfrastructureWorkloadSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summaries = %+v", summaries)
	}
	if summaries[0].HypervisorMac != hypervisorMac ||
		summaries[0].VMCount != 1 ||
		summaries[0].ContainerCount != 1 {
		t.Fatalf("summary = %+v", summaries[0])
	}
}
