package models

import "encoding/json"

type hostJSONBase struct {
	ID         int
	Name       string
	DNS        string
	Iface      string
	IP         string
	Mac        string
	Hw         string
	Date       string
	Known      int
	Now        int
	DeviceType string
}

type hostJSONWithMetadata struct {
	hostJSONBase
	Owner    string
	Location string
	Notes    string
	Tags     []string
	Pinned   bool
}

func (host Host) MarshalJSON() ([]byte, error) {
	base := hostJSONBase{
		ID:         host.ID,
		Name:       host.Name,
		DNS:        host.DNS,
		Iface:      host.Iface,
		IP:         host.IP,
		Mac:        host.Mac,
		Hw:         host.Hw,
		Date:       host.Date,
		Known:      host.Known,
		Now:        host.Now,
		DeviceType: host.DeviceType,
	}

	if !host.MetadataLoaded {
		return json.Marshal(base)
	}

	tags := host.Tags
	if tags == nil {
		tags = []string{}
	}

	return json.Marshal(hostJSONWithMetadata{
		hostJSONBase: base,
		Owner:        host.Owner,
		Location:     host.Location,
		Notes:        host.Notes,
		Tags:         tags,
		Pinned:       host.Pinned,
	})
}
