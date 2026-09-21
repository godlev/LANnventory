package gdb

import (
	"errors"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const deviceProfilesTable = "device_profiles"

var errEmptyDeviceProfileMAC = errors.New("device profile mac is empty")

// IsEmptyDeviceProfileMACError reports whether a profile cannot be stored
// because the owning Host has no MAC identity.
func IsEmptyDeviceProfileMACError(err error) bool {
	return errors.Is(err, errEmptyDeviceProfileMAC)
}

// SelectDeviceProfileByMAC returns the manually managed profile for one Host identity.
func SelectDeviceProfileByMAC(mac string) (models.DeviceProfile, bool, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return models.DeviceProfile{}, false, errEmptyDeviceProfileMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.DeviceProfile{}, false, err
	}
	defer release()

	var profile models.DeviceProfile
	err = activeDB.Table(deviceProfilesTable).Where("\"MAC\" = ?", mac).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.DeviceProfile{}, false, nil
	}
	return profile, err == nil, err
}

// UpdateDeviceProfile applies a partial managed profile update. Empty profiles are
// removed instead of retaining meaningless rows.
func UpdateDeviceProfile(mac string, update models.DeviceProfileUpdate) (models.DeviceProfile, bool, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return models.DeviceProfile{}, false, errEmptyDeviceProfileMAC
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.DeviceProfile{}, false, err
	}
	defer release()

	var saved models.DeviceProfile
	found := false
	err = activeDB.Transaction(func(txDB *gorm.DB) error {
		var profile models.DeviceProfile
		err := txDB.Table(deviceProfilesTable).Where("\"MAC\" = ?", mac).First(&profile).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			profile = models.DeviceProfile{Mac: mac}
		} else if err != nil {
			return err
		}

		if update.Manufacturer != nil {
			profile.Manufacturer = *update.Manufacturer
		}
		if update.Model != nil {
			profile.Model = *update.Model
		}
		if update.ManagementAddress != nil {
			profile.ManagementAddress = *update.ManagementAddress
		}

		if deviceProfileEmpty(profile) {
			if err := txDB.Table(deviceProfilesTable).Where("\"MAC\" = ?", mac).Delete(&models.DeviceProfile{}).Error; err != nil {
				return err
			}
			saved = models.DeviceProfile{}
			found = false
			return nil
		}

		profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := txDB.Table(deviceProfilesTable).Save(&profile).Error; err != nil {
			return err
		}
		saved = profile
		found = true
		return nil
	})

	return saved, found, err
}

func deleteDeviceProfileByMAC(txDB *gorm.DB, mac string) error {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return nil
	}
	return txDB.Table(deviceProfilesTable).Where("\"MAC\" = ?", mac).Delete(&models.DeviceProfile{}).Error
}

func deviceProfileEmpty(profile models.DeviceProfile) bool {
	return strings.TrimSpace(profile.Manufacturer) == "" &&
		strings.TrimSpace(profile.Model) == "" &&
		strings.TrimSpace(profile.ManagementAddress) == ""
}
