package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHostJSONIncludesMetadataOnlyWhenLoaded(t *testing.T) {
	host := Host{
		ID:         1,
		Name:       "NAS",
		IP:         "192.168.1.20",
		Mac:        "AA:BB:CC:DD:EE:20",
		DeviceType: "nas",
		Owner:      "Storage Team",
		Tags:       []string{"backup"},
		Pinned:     true,
	}

	plainPayload, err := json.Marshal(host)
	if err != nil {
		t.Fatalf("json.Marshal plain host: %v", err)
	}
	for _, field := range []string{`"ID":1`, `"Name":"NAS"`, `"IP":"192.168.1.20"`, `"DeviceType":"nas"`} {
		if !strings.Contains(string(plainPayload), field) {
			t.Fatalf("plain host JSON missing legacy field %s: %s", field, plainPayload)
		}
	}
	for _, field := range []string{`"Owner"`, `"Location"`, `"Notes"`, `"Tags"`, `"Pinned"`} {
		if strings.Contains(string(plainPayload), field) {
			t.Fatalf("plain host JSON contains metadata field %s: %s", field, plainPayload)
		}
	}

	host.MetadataLoaded = true
	enrichedPayload, err := json.Marshal(host)
	if err != nil {
		t.Fatalf("json.Marshal enriched host: %v", err)
	}
	for _, field := range []string{`"ID":1`, `"Name":"NAS"`, `"IP":"192.168.1.20"`, `"DeviceType":"nas"`} {
		if !strings.Contains(string(enrichedPayload), field) {
			t.Fatalf("enriched host JSON missing legacy field %s: %s", field, enrichedPayload)
		}
	}
	for _, field := range []string{`"Owner"`, `"Location"`, `"Notes"`, `"Tags"`, `"Pinned"`} {
		if !strings.Contains(string(enrichedPayload), field) {
			t.Fatalf("enriched host JSON missing metadata field %s: %s", field, enrichedPayload)
		}
	}
}
