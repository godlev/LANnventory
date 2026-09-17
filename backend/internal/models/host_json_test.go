package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHostJSONIncludesMetadataOnlyWhenLoaded(t *testing.T) {
	host := Host{
		ID:                 1,
		Name:               "NAS",
		IP:                 "192.168.1.20",
		Mac:                "AA:BB:CC:DD:EE:20",
		DeviceType:         "nas",
		Owner:              "Storage Team",
		Tags:               []string{"backup"},
		Pinned:             true,
		FirstSeen:          "2026-09-01 08:00:00",
		LastSeen:           "2026-09-05 09:00:00",
		FirstSeenEstimated: true,
	}

	plainPayload, err := json.Marshal(host)
	if err != nil {
		t.Fatalf("json.Marshal plain host: %v", err)
	}
	for _, field := range []string{`"ID":1`, `"Name":"NAS"`, `"IP":"192.168.1.20"`, `"DeviceType":"nas"`, `"MacType":"locally-administered"`} {
		if !strings.Contains(string(plainPayload), field) {
			t.Fatalf("plain host JSON missing field %s: %s", field, plainPayload)
		}
	}
	for _, field := range []string{`"Owner"`, `"Location"`, `"Notes"`, `"Tags"`, `"Pinned"`, `"FirstSeen"`, `"LastSeen"`, `"FirstSeenEstimated"`} {
		if strings.Contains(string(plainPayload), field) {
			t.Fatalf("plain host JSON contains enriched field %s: %s", field, plainPayload)
		}
	}

	host.MetadataLoaded = true
	enrichedPayload, err := json.Marshal(host)
	if err != nil {
		t.Fatalf("json.Marshal enriched host: %v", err)
	}
	for _, field := range []string{`"ID":1`, `"Name":"NAS"`, `"IP":"192.168.1.20"`, `"DeviceType":"nas"`, `"MacType":"locally-administered"`} {
		if !strings.Contains(string(enrichedPayload), field) {
			t.Fatalf("enriched host JSON missing field %s: %s", field, enrichedPayload)
		}
	}
	for _, field := range []string{`"Owner"`, `"Location"`, `"Notes"`, `"Tags"`, `"Pinned"`, `"FirstSeen"`, `"LastSeen"`, `"FirstSeenEstimated"`} {
		if !strings.Contains(string(enrichedPayload), field) {
			t.Fatalf("enriched host JSON missing enriched field %s: %s", field, enrichedPayload)
		}
	}
}

func TestHostJSONDerivesMACTypeWithoutPersistedState(t *testing.T) {
	tests := []struct {
		mac  string
		want string
	}{
		{mac: "00:1A:2B:3C:4D:5E", want: `"MacType":"globally-administered"`},
		{mac: "02:00:00:00:00:01", want: `"MacType":"locally-administered"`},
		{mac: "01:00:5E:00:00:FB", want: `"MacType":"multicast"`},
		{mac: "bad", want: `"MacType":"invalid"`},
	}

	for _, tt := range tests {
		payload, err := json.Marshal(Host{Mac: tt.mac})
		if err != nil {
			t.Fatalf("json.Marshal host %q: %v", tt.mac, err)
		}
		if !strings.Contains(string(payload), tt.want) {
			t.Fatalf("host %q payload = %s, want %s", tt.mac, payload, tt.want)
		}
	}
}
