package models

import "encoding/json"

// HostMetadata stores manually maintained inventory metadata keyed by MAC.
type HostMetadata struct {
	Mac      string `gorm:"column:MAC;primaryKey"`
	Owner    string `gorm:"column:OWNER"`
	Location string `gorm:"column:LOCATION"`
	Notes    string `gorm:"column:NOTES;type:text"`
	TagsJSON string `gorm:"column:TAGS_JSON;type:text"`
	Pinned   bool   `gorm:"column:PINNED"`
}

// HostMetadataUpdate describes a partial metadata update.
type HostMetadataUpdate struct {
	Owner    *string
	Location *string
	Notes    *string
	Tags     *[]string
	Pinned   *bool
}

func EncodeMetadataTags(tags []string) string {
	payload, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}

	return string(payload)
}

func DecodeMetadataTags(tagsJSON string) []string {
	if tagsJSON == "" {
		return []string{}
	}

	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return []string{}
	}
	if tags == nil {
		return []string{}
	}

	return tags
}

func ApplyHostMetadata(host *Host, metadata HostMetadata, ok bool) {
	host.MetadataLoaded = true
	host.Tags = []string{}
	if !ok {
		return
	}

	host.Owner = metadata.Owner
	host.Location = metadata.Location
	host.Notes = metadata.Notes
	host.Tags = DecodeMetadataTags(metadata.TagsJSON)
	host.Pinned = metadata.Pinned
}
