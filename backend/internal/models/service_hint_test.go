package models

import "testing"

func TestServiceHintForPortIsConservativeAndProtocolSpecific(t *testing.T) {
	tests := []struct {
		protocol string
		port     int
		want     string
	}{
		{"tcp", 22, "SSH"},
		{"tcp", 443, "HTTPS"},
		{"tcp", 445, "SMB"},
		{"tcp", 32400, "Plex Media Server"},
		{"tcp", 65000, ""},
		{"udp", 53, ""},
	}

	for _, test := range tests {
		if got := ServiceHintForPort(test.protocol, test.port); got != test.want {
			t.Fatalf("ServiceHintForPort(%q, %d) = %q, want %q", test.protocol, test.port, got, test.want)
		}
	}
}
