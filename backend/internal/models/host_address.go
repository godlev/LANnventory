package models

// HostAddress stores scanner-observed address history for one MAC identity.
// IFACE is the LANnventory scanner/source interface, not a proven remote NIC.
type HostAddress struct {
	ID        uint   `gorm:"column:ID;primaryKey;autoIncrement" json:"id"`
	Mac       string `gorm:"column:MAC;not null;uniqueIndex:idx_host_addresses_mac_address,priority:1" json:"mac"`
	Address   string `gorm:"column:ADDRESS;not null;uniqueIndex:idx_host_addresses_mac_address,priority:2" json:"address"`
	Family    string `gorm:"column:FAMILY" json:"family"`
	Iface     string `gorm:"column:IFACE" json:"iface"`
	FirstSeen string `gorm:"column:FIRST_SEEN" json:"firstSeen"`
	LastSeen  string `gorm:"column:LAST_SEEN" json:"lastSeen"`
	Active    bool   `gorm:"column:ACTIVE" json:"active"`
}
