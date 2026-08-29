package timestamp

import (
	"strings"
	"time"
)

const (
	CanonicalLayout = time.RFC3339
	LegacyLayout    = "2006-01-02 15:04:05"
)

var legacyLayouts = []string{
	LegacyLayout,
	"2006-01-02T15:04:05",
}

func Now() string {
	return Format(time.Now())
}

func Format(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(CanonicalLayout)
}

func ParseStored(value string) (time.Time, bool) {
	return ParseStoredInLocation(value, time.Local)
}

func ParseStoredInLocation(value string, legacyLocation *time.Location) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}

	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, true
	}

	if legacyLocation == nil {
		legacyLocation = time.Local
	}

	for _, layout := range legacyLayouts {
		if parsed, err := time.ParseInLocation(layout, trimmed, legacyLocation); err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

func NormalizeStored(value string) (string, bool) {
	parsed, ok := ParseStored(value)
	if !ok {
		return value, false
	}

	return Format(parsed), true
}

func NormalizeStoredInLocation(value string, legacyLocation *time.Location) (string, bool) {
	parsed, ok := ParseStoredInLocation(value, legacyLocation)
	if !ok {
		return value, false
	}

	return Format(parsed), true
}
