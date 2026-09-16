package diagnostics

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/arp"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/routines"
)

func healthyDeps() dependencies {
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	last := now.Add(-30 * time.Second)
	return dependencies{
		now:      func() time.Time { return now },
		lookPath: func(string) (string, error) { return "/usr/bin/arp-scan", nil },
		geteuid:  func() int { return 0 },
		stat: func(string) (os.FileInfo, error) {
			return nil, errors.New("stat should not be needed for root")
		},
		getCapabilities: func(string) (string, error) {
			return "", errors.New("getcap should not be needed for root")
		},
		inspectInterface: func(name string) interfaceInfo {
			return interfaceInfo{
				Exists:       true,
				Up:           true,
				HasIPv4:      true,
				HardwareAddr: "00:11:22:33:44:55",
			}
		},
		databaseHealth: func() gdb.DatabaseHealth {
			return gdb.DatabaseHealth{Connected: true, Backend: "sqlite"}
		},
		scannerState: func() routines.ScannerState {
			return routines.ScannerState{
				Status:               routines.ScannerStatusHealthy,
				LastScanStartedAt:    last.Add(-2800 * time.Millisecond),
				LastScanAt:           last,
				LastSuccessfulScanAt: last,
				Duration:             2800 * time.Millisecond,
				DevicesFound:         34,
				Interfaces:           []string{"eth0"},
				NextScanAt:           now.Add(90 * time.Second),
			}
		},
	}
}

func TestRunHealthyDiagnostics(t *testing.T) {
	report := run(models.Conf{Version: "0.1.0-beta.3.1", Ifaces: "eth0"}, healthyDeps())

	if !report.OK {
		t.Fatalf("report.OK = false; checks=%+v", report.Checks)
	}
	if len(report.Checks) != 10 {
		t.Fatalf("checks len = %d, want 10", len(report.Checks))
	}
	assertCheck(t, report, "LANnventory", StatusOK)
	assertCheck(t, report, "Database", StatusOK)
	assertCheck(t, report, "arp-scan", StatusOK)
	assertCheck(t, report, "Permissions", StatusOK)
	assertCheck(t, report, "Interface", StatusOK)
	assertCheck(t, report, "Scanner", StatusOK)
	assertCheck(t, report, "Last scan", StatusOK)
	assertCheck(t, report, "Last successful scan", StatusOK)
	assertCheck(t, report, "Scan duration", StatusOK)
	assertCheck(t, report, "Scanner errors", StatusOK)
}

func TestRunReportsMissingArpScanAndUnconfiguredSource(t *testing.T) {
	deps := healthyDeps()
	deps.lookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	report := run(models.Conf{Version: "0.1.0-beta.3.1"}, deps)

	if report.OK {
		t.Fatal("report.OK = true with missing arp-scan and no scan source")
	}
	assertCheck(t, report, "arp-scan", StatusError)
	assertCheck(t, report, "Permissions", StatusError)
	assertCheck(t, report, "Interface", StatusError)
}

func TestRunReportsUnavailableConfiguredInterface(t *testing.T) {
	deps := healthyDeps()
	deps.inspectInterface = func(name string) interfaceInfo {
		return interfaceInfo{Error: "route ip+net: no such network interface"}
	}

	report := run(models.Conf{Ifaces: "missing0"}, deps)
	if report.OK {
		t.Fatal("report.OK = true with missing interface")
	}

	item := assertCheck(t, report, "Interface", StatusError)
	if item.SuggestedFix == "" {
		t.Fatal("Interface suggested fix is empty")
	}
}

func TestRunAllowsARPStringsWithoutManualInterfaceAsWarning(t *testing.T) {
	deps := healthyDeps()
	report := run(models.Conf{ArpStrs: []string{"--localnet --interface=eth0"}}, deps)

	item := assertCheck(t, report, "Interface", StatusWarning)
	if item.Details == "" {
		t.Fatal("Interface warning details are empty")
	}
}

func TestRunReportsNonRootMissingNetRaw(t *testing.T) {
	deps := healthyDeps()
	deps.geteuid = func() int { return 1000 }
	deps.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
	deps.getCapabilities = func(string) (string, error) { return "", nil }

	report := run(models.Conf{Ifaces: "eth0"}, deps)
	if report.OK {
		t.Fatal("report.OK = true with missing CAP_NET_RAW")
	}
	assertCheck(t, report, "Permissions", StatusError)
}

func TestRunWarnsWhenCapabilitiesCannotBeInspected(t *testing.T) {
	deps := healthyDeps()
	deps.geteuid = func() int { return 1000 }
	deps.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
	deps.getCapabilities = func(string) (string, error) { return "", errGetcapUnavailable }

	report := run(models.Conf{Ifaces: "eth0"}, deps)
	assertCheck(t, report, "Permissions", StatusWarning)
}

func TestRunReportsScannerFailureAndPreservesLastSuccess(t *testing.T) {
	deps := healthyDeps()
	lastSuccess := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)
	deps.scannerState = func() routines.ScannerState {
		return routines.ScannerState{
			Status:               routines.ScannerStatusProblem,
			LastScanAt:           time.Date(2026, 9, 16, 9, 55, 0, 0, time.UTC),
			LastSuccessfulScanAt: lastSuccess,
			Duration:             2 * time.Minute,
			Interfaces:           []string{"eth0"},
			LastErrors: []arp.ScanError{{
				Source:  "eth0",
				Kind:    arp.ScanErrorTimeout,
				Message: "command timed out after 2m0s",
			}},
		}
	}

	report := run(models.Conf{Ifaces: "eth0"}, deps)
	if report.OK {
		t.Fatal("report.OK = true with scanner failure")
	}
	assertCheck(t, report, "Scanner", StatusError)
	assertCheck(t, report, "Last scan", StatusError)
	success := assertCheck(t, report, "Last successful scan", StatusOK)
	if success.Details != lastSuccess.Format(time.RFC3339) {
		t.Fatalf("last success details = %q, want %q", success.Details, lastSuccess.Format(time.RFC3339))
	}
	errItem := assertCheck(t, report, "Scanner errors", StatusError)
	if errItem.SuggestedFix == "" {
		t.Fatal("Scanner errors suggested fix is empty")
	}
}

func TestHasNetRawPermissionRequiresPermittedCapability(t *testing.T) {
	if !hasNetRawPermission("/usr/bin/arp-scan cap_net_raw=ep") {
		t.Fatal("cap_net_raw=ep was not accepted")
	}
	if !hasNetRawPermission("/usr/bin/arp-scan cap_net_raw=p") {
		t.Fatal("cap_net_raw=p was not accepted")
	}
	if hasNetRawPermission("/usr/bin/arp-scan cap_net_bind_service=ep") {
		t.Fatal("unrelated capability was accepted")
	}
	if hasNetRawPermission("/usr/bin/arp-scan cap_net_raw=e") {
		t.Fatal("effective-only capability was accepted without permitted set")
	}
}

func assertCheck(t *testing.T, report Report, name string, status Status) Item {
	t.Helper()
	for _, item := range report.Checks {
		if item.Check == name {
			if item.Status != status {
				t.Fatalf("%s status = %q, want %q; details=%q", name, item.Status, status, item.Details)
			}
			return item
		}
	}
	t.Fatalf("check %q not found: %+v", name, report.Checks)
	return Item{}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "arp-scan" }
func (fakeFileInfo) Size() int64        { return 1 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }
