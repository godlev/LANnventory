package arp

import (
	"testing"
	"time"
)

func TestParseOutputEmpty(t *testing.T) {
	hosts := parseOutput("", "eth0")
	if len(hosts) != 0 {
		t.Fatalf("parseOutput returned %d hosts, want 0", len(hosts))
	}
}

func TestParseOutputValidRows(t *testing.T) {
	text := "192.168.1.1\tAA:BB:CC:DD:EE:FF\tRouter Inc\n" +
		"192.168.1.20\t11:22:33:44:55:66\tNAS Vendor\n"

	hosts := parseOutput(text, "eth0")
	if len(hosts) != 2 {
		t.Fatalf("parseOutput returned %d hosts, want 2", len(hosts))
	}

	first := hosts[0]
	if first.Iface != "eth0" {
		t.Errorf("Iface = %q, want eth0", first.Iface)
	}
	if first.IP != "192.168.1.1" {
		t.Errorf("IP = %q, want 192.168.1.1", first.IP)
	}
	if first.Mac != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("Mac = %q, want AA:BB:CC:DD:EE:FF", first.Mac)
	}
	if first.Hw != "Router Inc" {
		t.Errorf("Hw = %q, want Router Inc", first.Hw)
	}
	if first.Now != 1 {
		t.Errorf("Now = %d, want 1", first.Now)
	}
	if _, err := time.Parse("2006-01-02 15:04:05", first.Date); err != nil {
		t.Errorf("Date = %q, want layout 2006-01-02 15:04:05: %v", first.Date, err)
	}
}

func TestParseOutputIgnoresMalformedRows(t *testing.T) {
	text := "not-a-valid-row\n" +
		"192.168.1.10\tAA:BB:CC:DD:EE:FF\n" +
		"\t\t\n" +
		"192.168.1.11\t11:22:33:44:55:66\tDesktop Vendor\r\n"

	hosts := parseOutput(text, "wifi0")
	if len(hosts) != 1 {
		t.Fatalf("parseOutput returned %d hosts, want 1", len(hosts))
	}

	host := hosts[0]
	if host.Iface != "wifi0" {
		t.Errorf("Iface = %q, want wifi0", host.Iface)
	}
	if host.IP != "192.168.1.11" {
		t.Errorf("IP = %q, want 192.168.1.11", host.IP)
	}
	if host.Mac != "11:22:33:44:55:66" {
		t.Errorf("Mac = %q, want 11:22:33:44:55:66", host.Mac)
	}
	if host.Hw != "Desktop Vendor" {
		t.Errorf("Hw = %q, want Desktop Vendor", host.Hw)
	}
}
