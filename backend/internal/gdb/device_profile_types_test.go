package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestTypedDeviceProfileMigrationsAreAdditive(t *testing.T) {
	startSelectTestDB(t)

	tests := []struct {
		table   string
		model   any
		columns []string
	}{
		{networkDeviceProfilesTable, &models.NetworkDeviceProfile{}, []string{"MAC", "MANAGEMENT_MODE", "PHYSICAL_PORT_COUNT", "PORT_CAPABILITY_NOTES", "UPDATED_AT"}},
		{systemDeviceProfilesTable, &models.SystemDeviceProfile{}, []string{"MAC", "ROLE", "OPERATING_SYSTEM", "VERSION", "UPDATED_AT"}},
		{hypervisorProfilesTable, &models.HypervisorProfile{}, []string{"MAC", "PLATFORM", "VERSION", "NODE_NAME", "CLUSTER_NAME", "UPDATED_AT"}},
	}
	for _, tt := range tests {
		if !db.Migrator().HasTable(tt.table) {
			t.Fatalf("%s table was not migrated", tt.table)
		}
		for _, column := range tt.columns {
			if !db.Table(tt.table).Migrator().HasColumn(tt.model, column) {
				t.Fatalf("%s missing %s column", tt.table, column)
			}
		}
	}

	for _, table := range []string{"now", "history"} {
		for _, column := range []string{"MANAGEMENT_MODE", "PHYSICAL_PORT_COUNT", "ROLE", "OPERATING_SYSTEM", "PLATFORM", "NODE_NAME", "CLUSTER_NAME"} {
			if db.Table(table).Migrator().HasColumn(&models.Host{}, column) {
				t.Fatalf("%s unexpectedly has typed profile column %s", table, column)
			}
		}
	}
}

func TestTypedDeviceProfilesPersistIndependently(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:42"

	mode := "managed"
	ports := 8
	notes := "2x 2.5GbE uplinks"
	network, found, err := UpdateNetworkDeviceProfile(mac, models.NetworkDeviceProfileUpdate{
		ManagementMode: &mode, PhysicalPortCount: &ports, PortCapabilityNotes: &notes,
	})
	if err != nil || !found || network.ManagementMode != mode || network.PhysicalPortCount != ports {
		t.Fatalf("network profile = %+v found=%v err=%v", network, found, err)
	}

	role := "Virtualization host"
	os := "Debian GNU/Linux"
	osVersion := "13"
	system, found, err := UpdateSystemDeviceProfile(mac, models.SystemDeviceProfileUpdate{
		Role: &role, OperatingSystem: &os, Version: &osVersion,
	})
	if err != nil || !found || system.Role != role || system.OperatingSystem != os {
		t.Fatalf("system profile = %+v found=%v err=%v", system, found, err)
	}

	platform := "proxmox-ve"
	hvVersion := "9.2"
	node := "proxmox"
	cluster := "home"
	hypervisor, found, err := UpdateHypervisorProfile(mac, models.HypervisorProfileUpdate{
		Platform: &platform, Version: &hvVersion, NodeName: &node, ClusterName: &cluster,
	})
	if err != nil || !found || hypervisor.Platform != platform || hypervisor.NodeName != node {
		t.Fatalf("hypervisor profile = %+v found=%v err=%v", hypervisor, found, err)
	}

	updatedRole := "Primary virtualization host"
	system, found, err = UpdateSystemDeviceProfile(mac, models.SystemDeviceProfileUpdate{Role: &updatedRole})
	if err != nil || !found || system.Role != updatedRole || system.OperatingSystem != os || system.Version != osVersion {
		t.Fatalf("partial system update = %+v found=%v err=%v", system, found, err)
	}

	if _, found, err := SelectNetworkDeviceProfileByMAC(mac); err != nil || !found {
		t.Fatalf("network profile disappeared after system update: found=%v err=%v", found, err)
	}
	if _, found, err := SelectHypervisorProfileByMAC(mac); err != nil || !found {
		t.Fatalf("hypervisor profile disappeared after system update: found=%v err=%v", found, err)
	}
}

func TestTypedProfileClearDoesNotDeleteOtherLayers(t *testing.T) {
	startSelectTestDB(t)
	mac := "AA:BB:CC:DD:EE:43"
	mode := "managed"
	role := "Server"
	platform := "proxmox-ve"

	if _, found, err := UpdateNetworkDeviceProfile(mac, models.NetworkDeviceProfileUpdate{ManagementMode: &mode}); err != nil || !found {
		t.Fatalf("seed network found=%v err=%v", found, err)
	}
	if _, found, err := UpdateSystemDeviceProfile(mac, models.SystemDeviceProfileUpdate{Role: &role}); err != nil || !found {
		t.Fatalf("seed system found=%v err=%v", found, err)
	}
	if _, found, err := UpdateHypervisorProfile(mac, models.HypervisorProfileUpdate{Platform: &platform}); err != nil || !found {
		t.Fatalf("seed hypervisor found=%v err=%v", found, err)
	}

	if err := DeleteHypervisorProfileByMAC(mac); err != nil {
		t.Fatalf("DeleteHypervisorProfileByMAC: %v", err)
	}
	if _, found, err := SelectHypervisorProfileByMAC(mac); err != nil || found {
		t.Fatalf("hypervisor after delete found=%v err=%v", found, err)
	}
	if _, found, err := SelectNetworkDeviceProfileByMAC(mac); err != nil || !found {
		t.Fatalf("network affected by hypervisor delete found=%v err=%v", found, err)
	}
	if _, found, err := SelectSystemDeviceProfileByMAC(mac); err != nil || !found {
		t.Fatalf("system affected by hypervisor delete found=%v err=%v", found, err)
	}
}
