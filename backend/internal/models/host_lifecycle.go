package models

// HostLifecycle stores scanner-managed durable lifecycle state keyed by MAC.
type HostLifecycle struct {
	Mac                string `gorm:"column:MAC;primaryKey"`
	FirstSeen          string `gorm:"column:FIRST_SEEN"`
	LastSeen           string `gorm:"column:LAST_SEEN"`
	FirstSeenEstimated bool   `gorm:"column:FIRST_SEEN_ESTIMATED"`
}

func ApplyHostLifecycle(host *Host, lifecycle HostLifecycle, ok bool) {
	if !ok {
		host.FirstSeen = ""
		host.LastSeen = ""
		host.FirstSeenEstimated = false
		return
	}

	host.FirstSeen = lifecycle.FirstSeen
	host.LastSeen = lifecycle.LastSeen
	host.FirstSeenEstimated = lifecycle.FirstSeenEstimated
}
