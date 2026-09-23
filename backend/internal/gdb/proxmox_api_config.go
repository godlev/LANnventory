package gdb

import (
	"errors"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const proxmoxAPIConfigsTable = "proxmox_api_configs"

func SelectProxmoxAPIConfig(hypervisorMac string) (models.ProxmoxAPIConfig, bool, error) {
	canonical, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return models.ProxmoxAPIConfig{}, false, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.ProxmoxAPIConfig{}, false, err
	}
	defer release()

	var config models.ProxmoxAPIConfig
	err = activeDB.Table(proxmoxAPIConfigsTable).
		Where(`"HYPERVISOR_MAC" = ?`, canonical).
		First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ProxmoxAPIConfig{}, false, nil
	}
	if err != nil {
		return models.ProxmoxAPIConfig{}, false, err
	}
	return config, true, nil
}

func UpsertProxmoxAPIConfig(config models.ProxmoxAPIConfig) error {
	canonical, err := identity.NormalizeMAC(config.HypervisorMac)
	if err != nil {
		return err
	}
	config.HypervisorMac = canonical
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	config.TokenID = strings.TrimSpace(config.TokenID)
	config.Status = strings.TrimSpace(config.Status)
	config.LastError = strings.TrimSpace(config.LastError)
	config.LastSyncAttemptAt = strings.TrimSpace(config.LastSyncAttemptAt)
	config.LastSuccessfulCollectionAt = strings.TrimSpace(config.LastSuccessfulCollectionAt)
	config.NextSyncAt = strings.TrimSpace(config.NextSyncAt)
	config.LastSyncStatus = strings.TrimSpace(config.LastSyncStatus)
	config.LastSyncError = strings.TrimSpace(config.LastSyncError)
	config.LastSyncTrigger = strings.TrimSpace(config.LastSyncTrigger)
	config.UpdatedAt = strings.TrimSpace(config.UpdatedAt)
	if config.SyncIntervalMinutes == 0 {
		config.SyncIntervalMinutes = 60
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	var existing models.ProxmoxAPIConfig
	err = activeDB.Table(proxmoxAPIConfigsTable).
		Where(`"HYPERVISOR_MAC" = ?`, canonical).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return activeDB.Table(proxmoxAPIConfigsTable).Create(&config).Error
	}
	if err != nil {
		return err
	}
	return activeDB.Table(proxmoxAPIConfigsTable).
		Where(`"HYPERVISOR_MAC" = ?`, canonical).
		Updates(map[string]any{
			"ENABLED":              config.Enabled,
			"BASE_URL":             config.BaseURL,
			"TOKEN_ID":             config.TokenID,
			"TOKEN_SECRET":         config.TokenSecret,
			"VERIFY_TLS":           config.VerifyTLS,
			"TIMEOUT_SECONDS":                 config.TimeoutSeconds,
			"AUTOMATIC_SYNC":                  config.AutomaticSync,
			"SYNC_INTERVAL_MINUTES":            config.SyncIntervalMinutes,
			"CONFIG_REVISION":                  config.ConfigRevision,
			"LAST_SYNC_ATTEMPT_AT":             config.LastSyncAttemptAt,
			"LAST_SUCCESSFUL_COLLECTION_AT":    config.LastSuccessfulCollectionAt,
			"NEXT_SYNC_AT":                     config.NextSyncAt,
			"LAST_SYNC_STATUS":                 config.LastSyncStatus,
			"LAST_SYNC_ERROR":                  config.LastSyncError,
			"LAST_SYNC_TRIGGER":                config.LastSyncTrigger,
			"LAST_ATTEMPT_AT":                  config.LastAttemptAt,
			"LAST_SUCCESSFUL_SYNC":             config.LastSuccessfulSync,
			"LAST_ERROR":                       config.LastError,
			"STATUS":                           config.Status,
			"UPDATED_AT":                       config.UpdatedAt,
		}).Error
}

func UpdateProxmoxAPIStatus(hypervisorMac, status, lastAttemptAt, lastSuccessfulSync, lastError string) error {
	canonical, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	updates := map[string]any{
		"LAST_ATTEMPT_AT": strings.TrimSpace(lastAttemptAt),
		"LAST_ERROR":      strings.TrimSpace(lastError),
		"STATUS":          strings.TrimSpace(status),
	}
	if success := strings.TrimSpace(lastSuccessfulSync); success != "" {
		updates["LAST_SUCCESSFUL_SYNC"] = success
		updates["ENABLED"] = true
	}
	return activeDB.Table(proxmoxAPIConfigsTable).
		Where(`"HYPERVISOR_MAC" = ?`, canonical).
		Updates(updates).Error
}

func deleteProxmoxAPIConfigByMAC(txDB *gorm.DB, mac string) error {
	canonical, err := identity.NormalizeMAC(mac)
	if err != nil {
		return err
	}
	return txDB.Table(proxmoxAPIConfigsTable).
		Where(`"HYPERVISOR_MAC" = ?`, canonical).
		Delete(&models.ProxmoxAPIConfig{}).Error
}
