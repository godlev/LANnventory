package arp

import (
	"os"
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

func TestRunCommandTimesOut(t *testing.T) {
	oldTimeout := scanCommandTimeout
	scanCommandTimeout = 10 * time.Millisecond
	t.Cleanup(func() {
		scanCommandTimeout = oldTimeout
	})
	t.Setenv("WYL_ARP_TEST_HELPER", "1")

	out, ok := runCommand(os.Args[0], "-test.run=TestHelperProcess", "--", "sleep")
	if ok {
		t.Fatal("runCommand returned ok=true after timeout, want false")
	}
	if out != "" {
		t.Fatalf("runCommand returned %q, want empty output after timeout", out)
	}
}

func TestScanReturnsFalseWhenCommandFails(t *testing.T) {
	oldRunner := commandRunner
	commandRunner = func(string, ...string) (string, bool) {
		return "", false
	}
	t.Cleanup(func() {
		commandRunner = oldRunner
	})

	hosts, ok := Scan("eth0", "", nil)
	if ok {
		t.Fatal("Scan returned ok=true, want false")
	}
	if len(hosts) != 0 {
		t.Fatalf("Scan returned %d hosts, want 0", len(hosts))
	}
}

func TestScanReturnsTrueForSuccessfulEmptyResult(t *testing.T) {
	oldRunner := commandRunner
	commandRunner = func(string, ...string) (string, bool) {
		return "", true
	}
	t.Cleanup(func() {
		commandRunner = oldRunner
	})

	hosts, ok := Scan("eth0", "", nil)
	if !ok {
		t.Fatal("Scan returned ok=false, want true")
	}
	if len(hosts) != 0 {
		t.Fatalf("Scan returned %d hosts, want 0", len(hosts))
	}
}

func TestScanSplitsArpArgsAndIgnoresIfaceWhitespace(t *testing.T) {
	oldRunner := commandRunner
	var calls [][]string
	commandRunner = func(_ string, args ...string) (string, bool) {
		calls = append(calls, append([]string(nil), args...))
		return "192.168.1.1\tAA:BB:CC:DD:EE:FF\tRouter Inc\n", true
	}
	t.Cleanup(func() {
		commandRunner = oldRunner
	})

	hosts, ok := Scan(" eth0   wifi0 ", "-r 1", nil)
	if !ok {
		t.Fatal("Scan returned ok=false, want true")
	}
	if len(hosts) != 2 {
		t.Fatalf("Scan returned %d hosts, want 2", len(hosts))
	}
	if len(calls) != 2 {
		t.Fatalf("command runner was called %d times, want 2", len(calls))
	}

	wantArgs := []string{"-glNx", "-r", "1", "-I", "eth0"}
	if len(calls[0]) != len(wantArgs) {
		t.Fatalf("call 0 args = %v, want %v", calls[0], wantArgs)
	}
	for i, wantArg := range wantArgs {
		if calls[0][i] != wantArg {
			t.Fatalf("call 0 arg %d = %q, want %q; args=%v", i, calls[0][i], wantArg, calls[0])
		}
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("WYL_ARP_TEST_HELPER") != "1" {
		return
	}

	if os.Args[len(os.Args)-1] == "sleep" {
		time.Sleep(time.Second)
	}
	os.Exit(0)
}
