package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestInfrastructureWorkloadMigrationUsesSeparateTables(t *testing.T) {
	startSelectTestDB(t)

	tests := []struct {
		table   string
		model   any
		columns []string
	}{
		{infrastructureWorkloadsTable, &models.InfrastructureWorkload{}, []string{"ID", "HYPERVISOR_MAC", "NATIVE_ID", "WORKLOAD_TYPE", "NAME", "STATUS", "SOURCE", "FIRST_SEEN", "LAST_SEEN", "RETIRED_AT", "UPDATED_AT"}},
		{infrastructureWorkloadInterfacesTable, &models.InfrastructureWorkloadInterface{}, []string{"ID", "WORKLOAD_ID", "NAME", "MAC", "BRIDGE", "VLAN_TAG", "CONFIGURED_ADDRESS", "CONFIGURED_NETWORK", "UPDATED_AT"}},
		{infrastructureWorkloadHostLinksTable, &models.InfrastructureWorkloadHostLink{}, []string{"WORKLOAD_ID", "HOST_ID", "HOST_MAC", "LINK_SOURCE", "LINKED_AT", "UPDATED_AT"}},
	}
	for _, tt := range tests {
		if !db.Migrator().HasTable(tt.table) {
			t.Fatalf("%s table missing", tt.table)
		}
		for _, column := range tt.columns {
			if !db.Table(tt.table).Migrator().HasColumn(tt.model, column) {
				t.Fatalf("%s missing %s column", tt.table, column)
			}
		}
	}

	for _, table := range []string{"now", "history"} {
		for _, column := range []string{"NATIVE_ID", "WORKLOAD_TYPE", "HYPERVISOR_MAC", "LINK_SOURCE"} {
			if db.Table(table).Migrator().HasColumn(&models.Host{}, column) {
				t.Fatalf("%s unexpectedly has workload column %s", table, column)
			}
		}
	}
}

func TestInfrastructureWorkloadDoesNotCreateLANnventoryHost(t *testing.T) {
	startSelectTestDB(t)

	record, err := UpsertInfrastructureWorkload("AA:BB:CC:DD:EE:50", models.InfrastructureWorkloadUpsert{
		NativeID:     "119",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "ubuntu-plex-immich",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceManual,
		Interfaces: []models.InfrastructureWorkloadInterface{{
			Name:              "net0",
			Mac:               "bc:24:11:a2:40:12",
			Bridge:            "vmbr0",
			ConfiguredAddress: "10.4.1.27",
		}},
	}, "2026-09-19T09:00:00Z")
	if err != nil {
		t.Fatalf("UpsertInfrastructureWorkload: %v", err)
	}
	if record.Workload.ID == 0 || record.Workload.NativeID != "119" || record.Workload.WorkloadType != "vm" {
		t.Fatalf("workload = %+v", record.Workload)
	}
	if len(record.Interfaces) != 1 || record.Interfaces[0].Mac != "BC:24:11:A2:40:12" {
		t.Fatalf("interfaces = %+v", record.Interfaces)
	}

	var hostCount int64
	if err := db.Table("now").Count(&hostCount).Error; err != nil {
		t.Fatalf("count hosts: %v", err)
	}
	if hostCount != 0 {
		t.Fatalf("current Hosts = %d, want zero fake Hosts", hostCount)
	}
}

func TestInfrastructureWorkloadInterfaceReplacementRollsBackOnInvalidInput(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:51"

	record, err := UpsertInfrastructureWorkload(mac, models.InfrastructureWorkloadUpsert{
		NativeID:     "101",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "router",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceManual,
		Interfaces:   []models.InfrastructureWorkloadInterface{{Name: "net0", Mac: "AA:BB:CC:00:00:01"}},
	}, "2026-09-19T09:00:00Z")
	if err != nil {
		t.Fatalf("seed workload: %v", err)
	}

	_, err = UpsertInfrastructureWorkload(mac, models.InfrastructureWorkloadUpsert{
		NativeID:     "101",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "router changed",
		Status:       models.InfrastructureWorkloadStatusStopped,
		Source:       models.InfrastructureWorkloadSourceManual,
		Interfaces: []models.InfrastructureWorkloadInterface{
			{Name: "net0", Mac: "AA:BB:CC:00:00:01"},
			{Name: "net0", Mac: "AA:BB:CC:00:00:02"},
		},
	}, "2026-09-19T09:10:00Z")
	if err == nil {
		t.Fatal("duplicate interface update unexpectedly succeeded")
	}

	persisted, found, err := SelectInfrastructureWorkloadByID(record.Workload.ID)
	if err != nil || !found {
		t.Fatalf("reload found=%v err=%v", found, err)
	}
	if persisted.Workload.Name != "router" || persisted.Workload.Status != models.InfrastructureWorkloadStatusRunning {
		t.Fatalf("failed update changed workload: %+v", persisted.Workload)
	}
	if len(persisted.Interfaces) != 1 || persisted.Interfaces[0].Name != "net0" || persisted.Interfaces[0].Mac != "AA:BB:CC:00:00:01" {
		t.Fatalf("failed update changed interfaces: %+v", persisted.Interfaces)
	}
}

func TestDeletingMatchedGuestHostUnlinksButKeepsWorkload(t *testing.T) {
	startSelectTestDB(t)

	hypervisor := models.Host{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:52", DeviceType: "server"}
	guest := models.Host{ID: 2, Name: "guest", Mac: "AA:BB:CC:DD:EE:53", DeviceType: "server"}
	if err := UpdateWithError("now", hypervisor); err != nil {
		t.Fatalf("seed hypervisor: %v", err)
	}
	if err := UpdateWithError("now", guest); err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	record, err := UpsertInfrastructureWorkload(hypervisor.Mac, models.InfrastructureWorkloadUpsert{
		NativeID:     "200",
		WorkloadType: models.InfrastructureWorkloadTypeVM,
		Name:         "guest",
		Status:       models.InfrastructureWorkloadStatusRunning,
		Source:       models.InfrastructureWorkloadSourceManual,
	}, "2026-09-19T09:00:00Z")
	if err != nil {
		t.Fatalf("seed workload: %v", err)
	}
	if _, err := SetInfrastructureWorkloadHostLink(record.Workload.ID, guest, models.InfrastructureWorkloadLinkSourceManual, "2026-09-19T09:05:00Z"); err != nil {
		t.Fatalf("link workload: %v", err)
	}

	if err := DeleteCurrentHostWithMetadata(guest); err != nil {
		t.Fatalf("delete guest: %v", err)
	}
	persisted, found, err := SelectInfrastructureWorkloadByID(record.Workload.ID)
	if err != nil || !found {
		t.Fatalf("workload after guest delete found=%v err=%v", found, err)
	}
	if persisted.Link != nil {
		t.Fatalf("guest delete left stale link: %+v", persisted.Link)
	}
}

func TestDeletingHypervisorHostDeletesItsHostedWorkloads(t *testing.T) {
	startSelectTestDB(t)

	hypervisor := models.Host{ID: 1, Name: "pve", Mac: "AA:BB:CC:DD:EE:54", DeviceType: "server"}
	if err := UpdateWithError("now", hypervisor); err != nil {
		t.Fatalf("seed hypervisor: %v", err)
	}
	record, err := UpsertInfrastructureWorkload(hypervisor.Mac, models.InfrastructureWorkloadUpsert{
		NativeID:     "201",
		WorkloadType: models.InfrastructureWorkloadTypeContainer,
		Name:         "service",
		Status:       models.InfrastructureWorkloadStatusStopped,
		Source:       models.InfrastructureWorkloadSourceManual,
		Interfaces:   []models.InfrastructureWorkloadInterface{{Name: "net0", Mac: "AA:BB:CC:00:00:20"}},
	}, "2026-09-19T09:00:00Z")
	if err != nil {
		t.Fatalf("seed workload: %v", err)
	}

	if err := DeleteCurrentHostWithMetadata(hypervisor); err != nil {
		t.Fatalf("delete hypervisor: %v", err)
	}
	if _, found, err := SelectInfrastructureWorkloadByID(record.Workload.ID); err != nil || found {
		t.Fatalf("workload after hypervisor delete found=%v err=%v, want absent", found, err)
	}

	var interfaceCount, linkCount int64
	if err := db.Table(infrastructureWorkloadInterfacesTable).Count(&interfaceCount).Error; err != nil {
		t.Fatalf("count workload interfaces: %v", err)
	}
	if err := db.Table(infrastructureWorkloadHostLinksTable).Count(&linkCount).Error; err != nil {
		t.Fatalf("count workload links: %v", err)
	}
	if interfaceCount != 0 || linkCount != 0 {
		t.Fatalf("dependent rows remain: interfaces=%d links=%d", interfaceCount, linkCount)
	}
}
