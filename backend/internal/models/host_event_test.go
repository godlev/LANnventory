package models

import (
	"strings"
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

func TestNewHostEventUsesTimezoneExplicitTimestamp(t *testing.T) {
	event := NewHostEvent(Host{
		ID:         1,
		Mac:        "AA:BB:CC:DD:EE:01",
		Name:       "router",
		IP:         "192.168.1.1",
		Iface:      "eth0",
		DeviceType: "router",
	}, EventDiscovered, "", "")

	if !strings.HasSuffix(event.Date, "Z") {
		t.Fatalf("Date = %q, want explicit UTC suffix", event.Date)
	}
	if _, err := time.Parse(time.RFC3339, event.Date); err != nil {
		t.Fatalf("Date = %q, want RFC3339: %v", event.Date, err)
	}
}
