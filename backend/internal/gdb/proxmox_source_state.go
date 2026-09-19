package gdb

import (
	"errors"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"gorm.io/gorm"
)

const proxmoxSourceStatesTable = "proxmox_source_states"

func SelectProxmoxSourceState(hypervisorMac, source string) (models.ProxmoxSourceState, bool, error) {
	canonical, err := identity.NormalizeMAC(hypervisorMac)
	if err != nil {
		return models.ProxmoxSourceState{}, false, err
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return models.ProxmoxSourceState{}, false, errors.New("Proxmox source is required")
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.ProxmoxSourceState{}, false, err
	}
	defer release()

	var state models.ProxmoxSourceState
	err = activeDB.Table(proxmoxSourceStatesTable).
		Where(`"HYPERVISOR_MAC" = ? AND "SOURCE" = ?`, canonical, source).
		First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ProxmoxSourceState{}, false, nil
	}
	return state, err == nil, err
}

func deleteProxmoxSourceStatesByMAC(txDB *gorm.DB, mac string) error {
	canonical, err := identity.NormalizeMAC(mac)
	if err != nil {
		return nil
	}
	return txDB.Table(proxmoxSourceStatesTable).
		Where(`"HYPERVISOR_MAC" = ?`, canonical).
		Delete(&models.ProxmoxSourceState{}).Error
}
