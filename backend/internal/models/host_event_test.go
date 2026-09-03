package models

import (
	"testing"
	"time"
)

func TestHostEventAutoMigrateCreatesEventsTable(t *testing.T) {
	db := openTempSQLite(t)

	if err := db.Table("events").AutoMigrate(&HostEvent{}); err != nil {
		t.Fatalf("AutoMigrate(events): %v", err)
	}

	for _, column := range []string{"ID", "HOST_ID", "MAC", "NAME", "EVENT_TYPE", "DATE", "IP", "IFACE", "DEVICE_TYPE", "OLD_VALUE", "NEW_VALUE"} {
		if !db.Table("events").Migrator().HasColumn(&HostEvent{}, column) {
			t.Fatalf("events table missing %s column after AutoMigrate", column)
		}
	}

	for _, column := range []string{"DateUTC", "DATE_UTC"} {
		if db.Table("events").Migrator().HasColumn(&HostEvent{}, column) {
			t.Fatalf("events table should not persist non-storage column %s", column)
		}
	}
}

func TestHostEventTypeValidation(t *testing.T) {
	for _, eventType := range HostEventTypeValues {
		if !IsValidHostEventType(string(eventType)) {
			t.Fatalf("event type %q should be valid", eventType)
		}
	}

	if IsValidHostEventType("renamed") {
		t.Fatal("renamed should not be a valid Phase 10 event type")
	}
}

func TestHostEventDateUTCUsesServerLocation(t *testing.T) {
	sofia := time.FixedZone("Europe/Sofia", 3*60*60)

	tests := []struct {
		name     string
		date     string
		location *time.Location
		want     string
	}{
		{
			name:     "UTC server",
			date:     "2026-09-03 18:18:43",
			location: time.UTC,
			want:     "2026-09-03T18:18:43Z",
		},
		{
			name:     "UTC plus three server",
			date:     "2026-09-03 21:18:43",
			location: sofia,
			want:     "2026-09-03T18:18:43Z",
		},
		{
			name:     "malformed",
			date:     "2026-09-03T18:18:43Z",
			location: time.UTC,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostEventDateUTC(tt.date, tt.location); got != tt.want {
				t.Fatalf("HostEventDateUTC(%q) = %q, want %q", tt.date, got, tt.want)
			}
		})
	}
}

func TestAddHostEventDisplayTimesDoesNotChangeStoredDate(t *testing.T) {
	events := []HostEvent{
		{Date: "2026-09-03 18:18:43"},
	}

	AddHostEventDisplayTimes(events, time.UTC)

	if events[0].Date != "2026-09-03 18:18:43" {
		t.Fatalf("Date = %q, want original persisted value", events[0].Date)
	}
	if events[0].DateUTC != "2026-09-03T18:18:43Z" {
		t.Fatalf("DateUTC = %q, want UTC display timestamp", events[0].DateUTC)
	}
}
