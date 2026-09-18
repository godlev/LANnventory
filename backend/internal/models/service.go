package models

// ServiceState is the last definitive state observed for one service endpoint.
type ServiceState string

const (
	ServiceStateOpen   ServiceState = "open"
	ServiceStateClosed ServiceState = "closed"
)

// ServiceProtocol is the transport protocol used to probe a service.
type ServiceProtocol string

const (
	ServiceProtocolTCP ServiceProtocol = "tcp"
)

// Service stores durable summary state for a service observed on one device address.
// Identity is MAC + ADDRESS + PROTOCOL + PORT so address changes do not rewrite history.
type Service struct {
	ID              uint   `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	Mac             string `gorm:"column:MAC;not null;uniqueIndex:idx_services_identity,priority:1;index:idx_services_mac" json:"mac"`
	Address         string `gorm:"column:ADDRESS;not null;uniqueIndex:idx_services_identity,priority:2" json:"address"`
	AddressFamily   string `gorm:"column:ADDRESS_FAMILY;not null" json:"addressFamily"`
	Protocol        string `gorm:"column:PROTOCOL;not null;uniqueIndex:idx_services_identity,priority:3" json:"protocol"`
	Port            int    `gorm:"column:PORT;not null;uniqueIndex:idx_services_identity,priority:4" json:"port"`
	State           string `gorm:"column:STATE;not null;index:idx_services_state" json:"state"`
	FirstDetected   string `gorm:"column:FIRST_DETECTED" json:"firstDetected"`
	LastDetected    string `gorm:"column:LAST_DETECTED" json:"lastDetected"`
	LastChecked     string `gorm:"column:LAST_CHECKED" json:"lastChecked"`
	StateChangedAt  string `gorm:"column:STATE_CHANGED_AT" json:"stateChangedAt"`
	ServiceHint     string `gorm:"column:SERVICE_HINT" json:"serviceHint"`
	LastScanSource  string `gorm:"column:LAST_SCAN_SOURCE" json:"lastScanSource"`
}

// ServiceScanSettings stores opt-in scheduled service scanning configuration per MAC identity.
// PortsJSON is a normalized JSON array kept as text for SQLite/PostgreSQL portability.
type ServiceScanSettings struct {
	Mac               string `gorm:"column:MAC;primaryKey" json:"mac"`
	Enabled           bool   `gorm:"column:ENABLED;not null;default:false" json:"enabled"`
	IntervalMinutes   int    `gorm:"column:INTERVAL_MINUTES;not null;default:1440" json:"intervalMinutes"`
	PortsJSON         string `gorm:"column:PORTS_JSON;not null" json:"portsJson"`
	NextScanAt        string `gorm:"column:NEXT_SCAN_AT" json:"nextScanAt"`
	LastAttemptAt     string `gorm:"column:LAST_ATTEMPT_AT" json:"lastAttemptAt"`
	LastSuccessfulAt  string `gorm:"column:LAST_SUCCESSFUL_AT" json:"lastSuccessfulAt"`
	LastError         string `gorm:"column:LAST_ERROR" json:"lastError"`
}

func IsValidServiceState(value string) bool {
	return value == string(ServiceStateOpen) || value == string(ServiceStateClosed)
}

func IsValidServiceProtocol(value string) bool {
	return value == string(ServiceProtocolTCP)
}
