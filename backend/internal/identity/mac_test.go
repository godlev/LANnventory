package identity

import "testing"

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "upper colon", raw: "AA:BB:CC:DD:EE:FF", want: "AA:BB:CC:DD:EE:FF"},
		{name: "lower colon", raw: "aa:bb:cc:dd:ee:ff", want: "AA:BB:CC:DD:EE:FF"},
		{name: "hyphen", raw: "AA-BB-CC-DD-EE-FF", want: "AA:BB:CC:DD:EE:FF"},
		{name: "dotted", raw: "aabb.ccdd.eeff", want: "AA:BB:CC:DD:EE:FF"},
		{name: "surrounding whitespace", raw: "  aa:bb:cc:dd:ee:ff  ", want: "AA:BB:CC:DD:EE:FF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeMAC(tt.raw)
			if err != nil {
				t.Fatalf("NormalizeMAC(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeMAC(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeMACRejectsInvalidAndNon48BitValues(t *testing.T) {
	for _, raw := range []string{
		"",
		"AA:BB:CC",
		"GG:BB:CC:DD:EE:FF",
		"01:23:45:67:89:AB:CD:EF",
	} {
		t.Run(raw, func(t *testing.T) {
			if got, err := NormalizeMAC(raw); err == nil {
				t.Fatalf("NormalizeMAC(%q) = %q, want error", raw, got)
			}
		})
	}
}

func TestClassifyMAC(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want MACType
	}{
		{name: "globally administered unicast", mac: "00:1A:2B:3C:4D:5E", want: MACTypeGloballyAdministered},
		{name: "locally administered unicast", mac: "02:00:00:00:00:01", want: MACTypeLocallyAdministered},
		{name: "multicast group", mac: "01:00:5E:00:00:FB", want: MACTypeMulticast},
		{name: "broadcast is group", mac: "FF:FF:FF:FF:FF:FF", want: MACTypeMulticast},
		{name: "invalid", mac: "not-a-mac", want: MACTypeInvalid},
		{name: "empty", mac: "", want: MACTypeInvalid},
		{name: "eui64 is not device mac", mac: "01:23:45:67:89:AB:CD:EF", want: MACTypeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyMAC(tt.mac); got != tt.want {
				t.Fatalf("ClassifyMAC(%q) = %q, want %q", tt.mac, got, tt.want)
			}
		})
	}
}

func TestMACKeyNormalizesValidRepresentations(t *testing.T) {
	want := "AA:BB:CC:DD:EE:FF"
	for _, raw := range []string{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff", "AA-BB-CC-DD-EE-FF", "aabb.ccdd.eeff"} {
		if got := MACKey(raw); got != want {
			t.Fatalf("MACKey(%q) = %q, want %q", raw, got, want)
		}
	}

	if got := MACKey("  legacy-invalid  "); got != "LEGACY-INVALID" {
		t.Fatalf("MACKey invalid fallback = %q, want LEGACY-INVALID", got)
	}
}
