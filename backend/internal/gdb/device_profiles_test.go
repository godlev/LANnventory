package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestDeviceProfileMigrationCreatesSeparateManagedTable(t *testing.T) {
	startSelectTestDB(t)

	if !db.Migrator().HasTable(deviceProfilesTable) {
		t.Fatal("device_profiles table was not migrated")
	}
	for _, column := range []string{"MAC", "MANUFACTURER", "MODEL", "MANAGEMENT_ADDRESS", "UPDATED_AT"} {
		if !db.Table(deviceProfilesTable).Migrator().HasColumn(&models.DeviceProfile{}, column) {
			t.Fatalf("device_profiles missing %s column", column)
		}
	}

	for _, table := range []string{"now", "history"} {
		for _, column := range []string{"MANUFACTURER", "MODEL", "MANAGEMENT_ADDRESS"} {
			if db.Table(table).Migrator().HasColumn(&models.Host{}, column) {
				t.Fatalf("%s unexpectedly has profile column %s", table, column)
			}
		}
	}
}

func TestDeviceProfilePartialUpdateAndClear(t *testing.T) {
	startSelectTestDB(t)

	manufacturer := "Fujitsu"
	model := "CELSIUS W550P"
	managementAddress := "truenas.home"
	profile, found, err := UpdateDeviceProfile("AA:BB:CC:DD:EE:35", models.DeviceProfileUpdate{
		Manufacturer:      &manufacturer,
		Model:             &model,
		ManagementAddress: &managementAddress,
	})
	if err != nil {
		t.Fatalf("UpdateDeviceProfile: %v", err)
	}
	if !found {
		t.Fatal("profile not persisted")
	}
	if profile.Manufacturer != manufacturer || profile.Model != model || profile.ManagementAddress != managementAddress || profile.UpdatedAt == "" {
		t.Fatalf("profile = %+v, want submitted values and timestamp", profile)
	}

	replacementModel := "CELSIUS W550P Gen2"
	profile, found, err = UpdateDeviceProfile(profile.Mac, models.DeviceProfileUpdate{Model: &replacementModel})
	if err != nil {
		t.Fatalf("partial UpdateDeviceProfile: %v", err)
	}
	if !found || profile.Manufacturer != manufacturer || profile.Model != replacementModel || profile.ManagementAddress != managementAddress {
		t.Fatalf("partial update lost unrelated values: %+v", profile)
	}

	empty := ""
	profile, found, err = UpdateDeviceProfile(profile.Mac, models.DeviceProfileUpdate{
		Manufacturer:      &empty,
		Model:             &empty,
		ManagementAddress: &empty,
	})
	if err != nil {
		t.Fatalf("clear UpdateDeviceProfile: %v", err)
	}
	if found || profile.Mac != "" {
		t.Fatalf("cleared profile = %+v, found=%v, want no row", profile, found)
	}
	if _, found, err := SelectDeviceProfileByMAC("AA:BB:CC:DD:EE:35"); err != nil || found {
		t.Fatalf("SelectDeviceProfileByMAC after clear found=%v err=%v, want absent", found, err)
	}
}

func TestDeviceProfileRequiresHostMACKey(t *testing.T) {
	startSelectTestDB(t)

	value := "Example"
	if _, _, err := UpdateDeviceProfile("  ", models.DeviceProfileUpdate{Manufacturer: &value}); !IsEmptyDeviceProfileMACError(err) {
		t.Fatalf("UpdateDeviceProfile empty MAC err = %v, want empty MAC error", err)
	}
	if _, _, err := SelectDeviceProfileByMAC(""); !IsEmptyDeviceProfileMACError(err) {
		t.Fatalf("SelectDeviceProfileByMAC empty MAC err = %v, want empty MAC error", err)
	}
}
