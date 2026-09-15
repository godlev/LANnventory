package models

// HostPort stores the latest known TCP state for a scanned host port.
type HostPort struct {
	HostID      int    `gorm:"column:HOST_ID;primaryKey" json:"hostId"`
	Port        int    `gorm:"column:PORT;primaryKey" json:"port"`
	Protocol    string `gorm:"column:PROTOCOL;primaryKey" json:"protocol"`
	Open        bool   `gorm:"column:OPEN" json:"open"`
	FirstSeen   string `gorm:"column:FIRST_SEEN" json:"firstSeen"`
	LastScanned string `gorm:"column:LAST_SCANNED" json:"lastScanned"`
	LastChanged string `gorm:"column:LAST_CHANGED" json:"lastChanged"`
}
