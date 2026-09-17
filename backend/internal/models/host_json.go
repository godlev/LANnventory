package models

import (
	"encoding/json"

	"github.com/godlev/LANnventory/internal/identity"
)

type hostJSONBase struct {
	ID                      int
	Name                    string
	DNS                     string
	Iface                   string
	IP                      string
	Mac                     string
	MacType                 identity.MACType
	MacAssessment           identity.MACAssessmentCode
	MacAssessmentConfidence identity.MACAssessmentConfidence
	MacAssessmentReason     string
	Hw                      string
	Date                    string
	Known                   int
	Now                     int
	DeviceType              string
}

type hostJSONWithMetadata struct {
	hostJSONBase
	Owner              string
	Location           string
	Notes              string
	Tags               []string
	Pinned             bool
	FirstSeen          string
	LastSeen           string
	FirstSeenEstimated bool
}

func (host Host) MarshalJSON() ([]byte, error) {
	assessment := identity.AssessMAC(host.Mac, host.DeviceType)
	base := hostJSONBase{
		ID:                      host.ID,
		Name:                    host.Name,
		DNS:                     host.DNS,
		Iface:                   host.Iface,
		IP:                      host.IP,
		Mac:                     host.Mac,
		MacType:                 identity.ClassifyMAC(host.Mac),
		MacAssessment:           assessment.Code,
		MacAssessmentConfidence: assessment.Confidence,
		MacAssessmentReason:     assessment.Reason,
		Hw:                      host.Hw,
		Date:                    host.Date,
		Known:                   host.Known,
		Now:                     host.Now,
		DeviceType:              host.DeviceType,
	}

	if !host.MetadataLoaded {
		return json.Marshal(base)
	}

	tags := host.Tags
	if tags == nil {
		tags = []string{}
	}

	return json.Marshal(hostJSONWithMetadata{
		hostJSONBase:       base,
		Owner:              host.Owner,
		Location:           host.Location,
		Notes:              host.Notes,
		Tags:               tags,
		Pinned:             host.Pinned,
		FirstSeen:          host.FirstSeen,
		LastSeen:           host.LastSeen,
		FirstSeenEstimated: host.FirstSeenEstimated,
	})
}
