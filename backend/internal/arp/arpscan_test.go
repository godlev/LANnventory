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
