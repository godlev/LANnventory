package gdb

import (
	"errors"
	"net"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/servicescan"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	servicesTable            = "services"
	serviceScanSettingsTable = "service_scan_settings"
)

var (
	errInvalidServiceIdentity = errors.New("invalid service identity")
	errInvalidServiceState    = errors.New("invalid service state")
	errInvalidServiceProtocol = errors.New("invalid service protocol")
	errInvalidServiceInterval = errors.New("invalid service scan interval")
	errInvalidServicePorts    = errors.New("invalid service scan ports")
)

// UpsertService stores one durable service summary keyed by MAC + address + protocol + port.
// Lifecycle/event semantics are intentionally handled by higher-level code in Phase 34.3.
func UpsertService(service models.Service) (models.Service, error) {
	normalized, err := normalizeService(service)
	if err != nil {
		return models.Service{}, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.Service{}, err
	}
	defer release()

	err = activeDB.Table(servicesTable).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "MAC"},
			{Name: "ADDRESS"},
			{Name: "PROTOCOL"},
			{Name: "PORT"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"ADDRESS_FAMILY",
			"STATE",
			"FIRST_DETECTED",
			"LAST_DETECTED",
			"LAST_CHECKED",
			"STATE_CHANGED_AT",
			"SERVICE_HINT",
			"LAST_SCAN_SOURCE",
		}),
	}).Create(&normalized).Error
	if err != nil {
		return models.Service{}, err
	}

	stored, ok, err := selectServiceByIdentity(activeDB, normalized.Mac, normalized.Address, normalized.Protocol, normalized.Port)
	if err != nil {
		return models.Service{}, err
	}
	if !ok {
		return models.Service{}, gorm.ErrRecordNotFound
	}
	return stored, nil
}

// SelectServicesByMAC returns all retained service summaries for one device identity.
func SelectServicesByMAC(mac string) ([]models.Service, error) {
	canonical, err := identity.NormalizeMAC(strings.TrimSpace(mac))
	if err != nil {
		return nil, errInvalidServiceIdentity
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var services []models.Service
	err = activeDB.Table(servicesTable).
		Where("\"MAC\" = ?", canonical).
		Order("\"ADDRESS\" ASC, \"PROTOCOL\" ASC, \"PORT\" ASC").
		Find(&services).Error
	return services, err
}

// SelectServiceByIdentity returns one service summary by its durable identity.
func SelectServiceByIdentity(mac, address, protocol string, port int) (models.Service, bool, error) {
	probe, err := normalizeService(models.Service{
		Mac:      mac,
		Address:  address,
		Protocol: protocol,
		Port:     port,
		State:    string(models.ServiceStateOpen),
	})
	if err != nil {
		return models.Service{}, false, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.Service{}, false, err
	}
	defer release()

	return selectServiceByIdentity(activeDB, probe.Mac, probe.Address, probe.Protocol, probe.Port)
}

// UpsertServiceScanSettings stores scheduled service-scan configuration for one MAC identity.
func UpsertServiceScanSettings(settings models.ServiceScanSettings) (models.ServiceScanSettings, error) {
	canonical, err := identity.NormalizeMAC(strings.TrimSpace(settings.Mac))
	if err != nil {
		return models.ServiceScanSettings{}, errInvalidServiceIdentity
	}
	if settings.IntervalMinutes <= 0 {
		return models.ServiceScanSettings{}, errInvalidServiceInterval
	}
	portsJSON, ports, err := servicescan.CanonicalPortsJSON(settings.PortsJSON)
	if err != nil || (settings.Enabled && len(ports) == 0) {
		return models.ServiceScanSettings{}, errInvalidServicePorts
	}
	settings.Mac = canonical
	settings.PortsJSON = portsJSON
	settings.LastError = strings.TrimSpace(settings.LastError)

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.ServiceScanSettings{}, err
	}
	defer release()

	err = activeDB.Table(serviceScanSettingsTable).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "MAC"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"ENABLED",
			"INTERVAL_MINUTES",
			"PORTS_JSON",
			"NEXT_SCAN_AT",
			"LAST_ATTEMPT_AT",
			"LAST_SUCCESSFUL_AT",
			"LAST_ERROR",
		}),
	}).Create(&settings).Error
	if err != nil {
		return models.ServiceScanSettings{}, err
	}

	stored, ok, err := selectServiceScanSettingsByMAC(activeDB, canonical)
	if err != nil {
		return models.ServiceScanSettings{}, err
	}
	if !ok {
		return models.ServiceScanSettings{}, gorm.ErrRecordNotFound
	}
	return stored, nil
}

// SelectServiceScanSettingsByMAC returns scheduled service-scan configuration for one MAC.
func SelectServiceScanSettingsByMAC(mac string) (models.ServiceScanSettings, bool, error) {
	canonical, err := identity.NormalizeMAC(strings.TrimSpace(mac))
	if err != nil {
		return models.ServiceScanSettings{}, false, errInvalidServiceIdentity
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return models.ServiceScanSettings{}, false, err
	}
	defer release()

	return selectServiceScanSettingsByMAC(activeDB, canonical)
}

// SelectDueServiceScanSettings returns enabled scheduled scans due at or before now.
func SelectDueServiceScanSettings(now string, limit int) ([]models.ServiceScanSettings, error) {
	now = strings.TrimSpace(now)
	if now == "" {
		return nil, errors.New("service scan due time is required")
	}
	if limit <= 0 {
		limit = 32
	}
	if limit > 100 {
		limit = 100
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	settings := make([]models.ServiceScanSettings, 0)
	err = activeDB.Table(serviceScanSettingsTable).
		Where("\"ENABLED\" = ? AND (\"NEXT_SCAN_AT\" = ? OR \"NEXT_SCAN_AT\" <= ?)", true, "", now).
		Order("\"NEXT_SCAN_AT\" ASC, \"MAC\" ASC").
		Limit(limit).
		Find(&settings).Error
	return settings, err
}

// UpdateServiceScanRuntime updates scheduler-owned fields without overwriting user configuration.
func UpdateServiceScanRuntime(mac, nextScanAt, lastAttemptAt, lastError string, successful bool) error {
	canonical, err := identity.NormalizeMAC(strings.TrimSpace(mac))
	if err != nil {
		return errInvalidServiceIdentity
	}

	updates := map[string]any{
		"NEXT_SCAN_AT":    strings.TrimSpace(nextScanAt),
		"LAST_ATTEMPT_AT": strings.TrimSpace(lastAttemptAt),
		"LAST_ERROR":      strings.TrimSpace(lastError),
	}
	if successful {
		updates["LAST_SUCCESSFUL_AT"] = strings.TrimSpace(lastAttemptAt)
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return err
	}
	defer release()

	result := activeDB.Table(serviceScanSettingsTable).Where("\"MAC\" = ?", canonical).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func selectServiceByIdentity(activeDB *gorm.DB, mac, address, protocol string, port int) (service models.Service, ok bool, err error) {
	err = activeDB.Table(servicesTable).
		Where("\"MAC\" = ? AND \"ADDRESS\" = ? AND \"PROTOCOL\" = ? AND \"PORT\" = ?", mac, address, protocol, port).
		First(&service).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return service, false, nil
	}
	if err != nil {
		return service, false, err
	}
	return service, true, nil
}

func selectServiceScanSettingsByMAC(activeDB *gorm.DB, mac string) (settings models.ServiceScanSettings, ok bool, err error) {
	err = activeDB.Table(serviceScanSettingsTable).
		Where("\"MAC\" = ?", mac).
		First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return settings, false, nil
	}
	if err != nil {
		return settings, false, err
	}
	return settings, true, nil
}

func backfillServiceHints(activeDB *gorm.DB) error {
	var services []models.Service
	if err := activeDB.Table(servicesTable).
		Where("(\"SERVICE_HINT\" = ? OR \"SERVICE_HINT\" IS NULL)", "").
		Find(&services).Error; err != nil {
		return err
	}

	for _, service := range services {
		hint := models.ServiceHintForPort(strings.ToLower(strings.TrimSpace(service.Protocol)), service.Port)
		if hint == "" {
			continue
		}
		if err := activeDB.Table(servicesTable).
			Where("\"ID\" = ? AND (\"SERVICE_HINT\" = ? OR \"SERVICE_HINT\" IS NULL)", service.ID, "").
			Update("SERVICE_HINT", hint).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeService(service models.Service) (models.Service, error) {
	canonicalMAC, err := identity.NormalizeMAC(strings.TrimSpace(service.Mac))
	if err != nil {
		return models.Service{}, errInvalidServiceIdentity
	}

	ip := net.ParseIP(strings.TrimSpace(service.Address))
	if ip == nil || service.Port < 1 || service.Port > 65535 {
		return models.Service{}, errInvalidServiceIdentity
	}

	protocol := strings.ToLower(strings.TrimSpace(service.Protocol))
	if !models.IsValidServiceProtocol(protocol) {
		return models.Service{}, errInvalidServiceProtocol
	}
	state := strings.ToLower(strings.TrimSpace(service.State))
	if !models.IsValidServiceState(state) {
		return models.Service{}, errInvalidServiceState
	}

	family := "ipv6"
	address := ip.String()
	if ipv4 := ip.To4(); ipv4 != nil {
		family = "ipv4"
		address = ipv4.String()
	}

	service.Mac = canonicalMAC
	service.Address = address
	service.AddressFamily = family
	service.Protocol = protocol
	service.State = state
	service.ServiceHint = strings.TrimSpace(service.ServiceHint)
	if service.ServiceHint == "" {
		service.ServiceHint = models.ServiceHintForPort(protocol, service.Port)
	}
	service.LastScanSource = strings.TrimSpace(service.LastScanSource)
	return service, nil
}
