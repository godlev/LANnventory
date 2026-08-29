package timestamp

import (
	"testing"
	"time"
)

func TestFormatUsesExplicitUTC(t *testing.T) {
	sofia := time.FixedZone("UTC+03", 3*60*60)
	got := Format(time.Date(2026, 8, 29, 0, 23, 45, 987654321, sofia))
	want := "2026-08-28T21:23:45Z"

	if got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}

func TestNowUsesTimezoneExplicitTimestamp(t *testing.T) {
	got := Now()

	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Fatalf("Now() returned %q, which is not RFC3339: %v", got, err)
	}
}

func TestParseStoredInterpretsLegacyWithProvidedLocation(t *testing.T) {
	sofia := time.FixedZone("UTC+03", 3*60*60)
	parsed, ok := ParseStoredInLocation("2026-08-29 00:23:45", sofia)
	if !ok {
		t.Fatal("ParseStoredInLocation() rejected a legacy timestamp")
	}

	want := "2026-08-28T21:23:45Z"
	if got := Format(parsed); got != want {
		t.Fatalf("legacy timestamp normalized to %q, want %q", got, want)
	}
}

func TestParseStoredKeepsExplicitInstantAcrossLocalZones(t *testing.T) {
	sofia := time.FixedZone("UTC+03", 3*60*60)
	parsed, ok := ParseStoredInLocation("2026-08-28T21:23:45Z", sofia)
	if !ok {
		t.Fatal("ParseStoredInLocation() rejected an explicit timestamp")
	}

	if got := Format(parsed); got != "2026-08-28T21:23:45Z" {
		t.Fatalf("explicit timestamp normalized to %q", got)
	}
}

func TestParseStoredHandlesDSTLocationForLegacyValues(t *testing.T) {
	location, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Skipf("Europe/Sofia timezone data unavailable: %v", err)
	}

	parsed, ok := ParseStoredInLocation("2026-03-29 04:30:00", location)
	if !ok {
		t.Fatal("ParseStoredInLocation() rejected a DST legacy timestamp")
	}

	if got := Format(parsed); got != "2026-03-29T01:30:00Z" {
		t.Fatalf("DST legacy timestamp normalized to %q", got)
	}
}

func TestNormalizeStoredRejectsMalformedTimestamp(t *testing.T) {
	got, ok := NormalizeStoredInLocation("not-a-date", time.UTC)
	if ok {
		t.Fatalf("NormalizeStoredInLocation() accepted malformed timestamp and returned %q", got)
	}

	if got != "not-a-date" {
		t.Fatalf("NormalizeStoredInLocation() changed malformed timestamp to %q", got)
	}
}
