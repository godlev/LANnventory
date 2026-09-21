package gdb

import (
	"errors"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const (
	networkDeviceProfilesTable = "network_device_profiles"
	systemDeviceProfilesTable  = "system_device_profiles"
	hypervisorProfilesTable    = "hypervisor_profiles"
)

// SelectNetworkDeviceProfileByMAC returns the managed network specialization.
func SelectNetworkDeviceProfileByMAC(mac string) (models.NetworkDeviceProfile, bool, error) {
	var profile models.NetworkDeviceProfile
	found, err := selectTypedProfile(mac, networkDeviceProfilesTable, &profile)
	return profile, found, err
}

// UpdateNetworkDeviceProfile applies a partial managed network profile update.
func UpdateNetworkDeviceProfile(mac string, update models.NetworkDeviceProfileUpdate) (models.NetworkDeviceProfile, bool, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return models.NetworkDeviceProfile{}, false, errEmptyDeviceProfileMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.NetworkDeviceProfile{}, false, err
	}
	defer release()

	var saved models.NetworkDeviceProfile
	found := false
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var profile models.NetworkDeviceProfile
		err := txDB.Table(networkDeviceProfilesTable).Where("\"MAC\" = ?", mac).First(&profile).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			profile = models.NetworkDeviceProfile{Mac: mac}
		} else if err != nil {
			return err
		}

		if update.ManagementMode != nil {
			profile.ManagementMode = *update.ManagementMode
		}
		if update.PhysicalPortCount != nil {
			profile.PhysicalPortCount = *update.PhysicalPortCount
		}
		if update.PortCapabilityNotes != nil {
			profile.PortCapabilityNotes = *update.PortCapabilityNotes
		}

		if networkDeviceProfileEmpty(profile) {
			if err := txDB.Table(networkDeviceProfilesTable).Where("\"MAC\" = ?", mac).Delete(&models.NetworkDeviceProfile{}).Error; err != nil {
				return err
			}
			saved = models.NetworkDeviceProfile{}
			found = false
			return nil
		}

		profile.UpdatedAt = typedProfileUpdatedAt()
		if err := txDB.Table(networkDeviceProfilesTable).Save(&profile).Error; err != nil {
			return err
		}
		saved = profile
		found = true
		return nil
	})
	return saved, found, err
}

// SelectSystemDeviceProfileByMAC returns the managed Server/NAS specialization.
func SelectSystemDeviceProfileByMAC(mac string) (models.SystemDeviceProfile, bool, error) {
	var profile models.SystemDeviceProfile
	found, err := selectTypedProfile(mac, systemDeviceProfilesTable, &profile)
	return profile, found, err
}

// UpdateSystemDeviceProfile applies a partial managed system profile update.
func UpdateSystemDeviceProfile(mac string, update models.SystemDeviceProfileUpdate) (models.SystemDeviceProfile, bool, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return models.SystemDeviceProfile{}, false, errEmptyDeviceProfileMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.SystemDeviceProfile{}, false, err
	}
	defer release()

	var saved models.SystemDeviceProfile
	found := false
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var profile models.SystemDeviceProfile
		err := txDB.Table(systemDeviceProfilesTable).Where("\"MAC\" = ?", mac).First(&profile).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			profile = models.SystemDeviceProfile{Mac: mac}
		} else if err != nil {
			return err
		}

		if update.Role != nil {
			profile.Role = *update.Role
		}
		if update.OperatingSystem != nil {
			profile.OperatingSystem = *update.OperatingSystem
		}
		if update.Version != nil {
			profile.Version = *update.Version
		}

		if systemDeviceProfileEmpty(profile) {
			if err := txDB.Table(systemDeviceProfilesTable).Where("\"MAC\" = ?", mac).Delete(&models.SystemDeviceProfile{}).Error; err != nil {
				return err
			}
			saved = models.SystemDeviceProfile{}
			found = false
			return nil
		}

		profile.UpdatedAt = typedProfileUpdatedAt()
		if err := txDB.Table(systemDeviceProfilesTable).Save(&profile).Error; err != nil {
			return err
		}
		saved = profile
		found = true
		return nil
	})
	return saved, found, err
}

// SelectHypervisorProfileByMAC returns the managed hypervisor specialization.
func SelectHypervisorProfileByMAC(mac string) (models.HypervisorProfile, bool, error) {
	var profile models.HypervisorProfile
	found, err := selectTypedProfile(mac, hypervisorProfilesTable, &profile)
	return profile, found, err
}

// UpdateHypervisorProfile creates or updates a managed hypervisor profile.
// Platform is required for a persisted row; setting it to empty clears the profile.
func UpdateHypervisorProfile(mac string, update models.HypervisorProfileUpdate) (models.HypervisorProfile, bool, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return models.HypervisorProfile{}, false, errEmptyDeviceProfileMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.HypervisorProfile{}, false, err
	}
	defer release()

	var saved models.HypervisorProfile
	found := false
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var profile models.HypervisorProfile
		err := txDB.Table(hypervisorProfilesTable).Where("\"MAC\" = ?", mac).First(&profile).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			profile = models.HypervisorProfile{Mac: mac}
		} else if err != nil {
			return err
		}

		if update.Platform != nil {
			profile.Platform = *update.Platform
		}
		if update.Version != nil {
			profile.Version = *update.Version
		}
		if update.NodeName != nil {
			profile.NodeName = *update.NodeName
		}
		if update.ClusterName != nil {
			profile.ClusterName = *update.ClusterName
		}

		if strings.TrimSpace(profile.Platform) == "" {
			if err := txDB.Table(hypervisorProfilesTable).Where("\"MAC\" = ?", mac).Delete(&models.HypervisorProfile{}).Error; err != nil {
				return err
			}
			saved = models.HypervisorProfile{}
			found = false
			return nil
		}

		profile.UpdatedAt = typedProfileUpdatedAt()
		if err := txDB.Table(hypervisorProfilesTable).Save(&profile).Error; err != nil {
			return err
		}
		saved = profile
		found = true
		return nil
	})
	return saved, found, err
}

// DeleteHypervisorProfileByMAC removes only the managed hypervisor capability.
func DeleteHypervisorProfileByMAC(mac string) error {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return errEmptyDeviceProfileMAC
	}
	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	return activeDB.Table(hypervisorProfilesTable).Where("\"MAC\" = ?", mac).Delete(&models.HypervisorProfile{}).Error
}

func selectTypedProfile(mac, table string, destination any) (bool, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return false, errEmptyDeviceProfileMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return false, err
	}
	defer release()

	err = activeDB.Table(table).Where("\"MAC\" = ?", mac).First(destination).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func deleteTypedDeviceProfilesByMAC(txDB *gorm.DB, mac string) error {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return nil
	}
	for _, spec := range []struct {
		table string
		value any
	}{
		{networkDeviceProfilesTable, &models.NetworkDeviceProfile{}},
		{systemDeviceProfilesTable, &models.SystemDeviceProfile{}},
		{hypervisorProfilesTable, &models.HypervisorProfile{}},
	} {
		if err := txDB.Table(spec.table).Where("\"MAC\" = ?", mac).Delete(spec.value).Error; err != nil {
			return err
		}
	}
	return nil
}

func networkDeviceProfileEmpty(profile models.NetworkDeviceProfile) bool {
	return strings.TrimSpace(profile.ManagementMode) == "" &&
		profile.PhysicalPortCount == 0 &&
		strings.TrimSpace(profile.PortCapabilityNotes) == ""
}

func systemDeviceProfileEmpty(profile models.SystemDeviceProfile) bool {
	return strings.TrimSpace(profile.Role) == "" &&
		strings.TrimSpace(profile.OperatingSystem) == "" &&
		strings.TrimSpace(profile.Version) == ""
}

func typedProfileUpdatedAt() string {
	return time.Now().UTC().Format(time.RFC3339)
}
