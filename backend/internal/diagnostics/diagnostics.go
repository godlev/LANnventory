package diagnostics

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/godlev/LANnventory/internal/arp"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/routines"
	"github.com/godlev/LANnventory/internal/version"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
)

type Item struct {
	Check        string `json:"check"`
	Status       Status `json:"status"`
	Details      string `json:"details"`
	SuggestedFix string `json:"suggestedFix,omitempty"`
}

type Report struct {
	OK          bool      `json:"ok"`
	GeneratedAt time.Time `json:"generatedAt"`
	Checks      []Item    `json:"checks"`
}

type interfaceInfo struct {
	Exists         bool
	Up             bool
	Loopback       bool
	HasIPv4        bool
	HardwareAddr   string
	Error          string
}

type dependencies struct {
	now              func() time.Time
	lookPath         func(string) (string, error)
	geteuid          func() int
	stat             func(string) (os.FileInfo, error)
	getCapabilities  func(string) (string, error)
	inspectInterface func(string) interfaceInfo
	databaseHealth   func() gdb.DatabaseHealth
	scannerState     func() routines.ScannerState
}

var errGetcapUnavailable = errors.New("getcap is not available")

func Run(config models.Conf) Report {
	return run(config, defaultDependencies())
}

func run(config models.Conf, deps dependencies) Report {
	report := Report{
		OK:          true,
		GeneratedAt: deps.now(),
		Checks:      make([]Item, 0, 10),
	}

	versionText := strings.TrimSpace(config.Version)
	if versionText == "" {
		versionText = version.Version
	}
	add(&report, Item{
		Check:   "LANnventory",
		Status:  StatusOK,
		Details: "Running v" + strings.TrimPrefix(versionText, "v"),
	})

	dbHealth := deps.databaseHealth()
	if dbHealth.Connected {
		add(&report, Item{
			Check:   "Database",
			Status:  StatusOK,
			Details: databaseBackendLabel(dbHealth.Backend) + " connected",
		})
	} else {
		details := "Database is not connected"
		if dbHealth.Error != "" {
			details += ": " + dbHealth.Error
		}
		add(&report, Item{
			Check:        "Database",
			Status:       StatusError,
			Details:      details,
			SuggestedFix: "Check the configured database settings and LANnventory service logs.",
		})
	}

	arpPath, pathErr := deps.lookPath("arp-scan")
	if pathErr != nil || strings.TrimSpace(arpPath) == "" {
		add(&report, Item{
			Check:        "arp-scan",
			Status:       StatusError,
			Details:      "arp-scan is not available in PATH",
			SuggestedFix: "Install the arp-scan package and restart LANnventory.",
		})
		add(&report, Item{
			Check:        "Permissions",
			Status:       StatusError,
			Details:      "Cannot verify scanner permissions because arp-scan was not found",
			SuggestedFix: "Install arp-scan first, then run Diagnostics again.",
		})
	} else {
		add(&report, Item{
			Check:   "arp-scan",
			Status:  StatusOK,
			Details: arpPath,
		})
		add(&report, permissionCheck(arpPath, deps))
	}

	add(&report, interfaceCheck(config, deps))

	state := deps.scannerState()
	add(&report, scannerStatusCheck(state))
	add(&report, lastScanCheck(state))
	add(&report, lastSuccessfulScanCheck(state))
	add(&report, scanDurationCheck(state))
	add(&report, scannerErrorsCheck(state))

	return report
}

func add(report *Report, item Item) {
	report.Checks = append(report.Checks, item)
	if item.Status == StatusError {
		report.OK = false
	}
}

func permissionCheck(arpPath string, deps dependencies) Item {
	if deps.geteuid() == 0 {
		return Item{
			Check:   "Permissions",
			Status:  StatusOK,
			Details: "LANnventory is running as root; arp-scan can request raw-socket access",
		}
	}

	info, err := deps.stat(arpPath)
	if err == nil && info.Mode()&os.ModeSetuid != 0 {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Uid == 0 {
			return Item{
				Check:   "Permissions",
				Status:  StatusOK,
				Details: "arp-scan is setuid root",
			}
		}
	}

	caps, err := deps.getCapabilities(arpPath)
	if err == nil && hasNetRawPermission(caps) {
		return Item{
			Check:   "Permissions",
			Status:  StatusOK,
			Details: strings.TrimSpace(caps),
		}
	}
	if errors.Is(err, errGetcapUnavailable) {
		return Item{
			Check:        "Permissions",
			Status:       StatusWarning,
			Details:      "LANnventory is not running as root and file capabilities could not be inspected",
			SuggestedFix: "Install getcap (libcap2-bin on Debian) and run Diagnostics again, or verify arp-scan has CAP_NET_RAW.",
		}
	}

	details := "Missing CAP_NET_RAW for non-root arp-scan execution"
	if strings.TrimSpace(caps) != "" {
		details += ": " + strings.TrimSpace(caps)
	}
	return Item{
		Check:        "Permissions",
		Status:       StatusError,
		Details:      details,
		SuggestedFix: "Grant CAP_NET_RAW to the arp-scan executable or run LANnventory with the required raw-network privilege.",
	}
}

func hasNetRawPermission(caps string) bool {
	caps = strings.ToLower(strings.TrimSpace(caps))
	if !strings.Contains(caps, "cap_net_raw") {
		return false
	}

	for _, field := range strings.Fields(caps) {
		if !strings.Contains(field, "cap_net_raw") {
			continue
		}
		parts := strings.SplitN(field, "=", 2)
		if len(parts) == 2 && strings.Contains(parts[1], "p") {
			return true
		}
	}
	return false
}

func interfaceCheck(config models.Conf, deps dependencies) Item {
	configured := strings.Fields(config.Ifaces)
	if len(configured) == 0 {
		if hasConfiguredARPStrings(config.ArpStrs) {
			return Item{
				Check:   "Interface",
				Status:  StatusWarning,
				Details: "No manual interface is configured; scanning relies on configured ARP Strings",
			}
		}
		return Item{
			Check:        "Interface",
			Status:       StatusError,
			Details:      "No scan interface or ARP String source is configured",
			SuggestedFix: "Set a real interface in Settings → Scanning. Automatic interface detection is not enabled.",
		}
	}

	usable := make([]string, 0, len(configured))
	problems := make([]string, 0)
	for _, name := range configured {
		info := deps.inspectInterface(name)
		switch {
		case !info.Exists:
			detail := name + " does not exist"
			if info.Error != "" {
				detail += ": " + info.Error
			}
			problems = append(problems, detail)
		case !info.Up:
			problems = append(problems, name+" is down")
		case info.Loopback:
			problems = append(problems, name+" is a loopback interface")
		case !info.HasIPv4:
			problems = append(problems, name+" has no usable IPv4 address")
		case info.HardwareAddr == "":
			problems = append(problems, name+" has no hardware address")
		default:
			usable = append(usable, name)
		}
	}

	if len(problems) > 0 {
		details := strings.Join(problems, "; ")
		if len(usable) > 0 {
			details = "Usable: " + strings.Join(usable, ", ") + "; problems: " + details
		}
		return Item{
			Check:        "Interface",
			Status:       StatusError,
			Details:      details,
			SuggestedFix: "Connect or enable the configured interface, or update Settings → Scanning to an interface that exists and has IPv4.",
		}
	}

	return Item{
		Check:   "Interface",
		Status:  StatusOK,
		Details: strings.Join(usable, ", ") + " available",
	}
}

func hasConfiguredARPStrings(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func scannerStatusCheck(state routines.ScannerState) Item {
	switch state.Status {
	case routines.ScannerStatusHealthy:
		return Item{Check: "Scanner", Status: StatusOK, Details: "Healthy"}
	case routines.ScannerStatusScanning:
		return Item{Check: "Scanner", Status: StatusOK, Details: "Scanning"}
	case routines.ScannerStatusProblem:
		return Item{
			Check:        "Scanner",
			Status:       StatusError,
			Details:      "Last scanner cycle has a problem",
			SuggestedFix: "Review the scanner errors and interface/permission checks below.",
		}
	default:
		return Item{
			Check:   "Scanner",
			Status:  StatusWarning,
			Details: "Scanner state has not been initialized yet",
		}
	}
}

func lastScanCheck(state routines.ScannerState) Item {
	if state.LastScanAt.IsZero() {
		if state.Status == routines.ScannerStatusScanning {
			return Item{Check: "Last scan", Status: StatusWarning, Details: "Initial scan is still in progress"}
		}
		return Item{Check: "Last scan", Status: StatusWarning, Details: "No completed scan has been recorded yet"}
	}

	status := StatusOK
	prefix := fmt.Sprintf("%d devices; completed %s", state.DevicesFound, formatTime(state.LastScanAt))
	if state.Status == routines.ScannerStatusProblem {
		status = StatusError
		prefix = "Failed; " + prefix
	}
	return Item{Check: "Last scan", Status: status, Details: prefix}
}

func lastSuccessfulScanCheck(state routines.ScannerState) Item {
	if state.LastSuccessfulScanAt.IsZero() {
		status := StatusWarning
		if state.Status == routines.ScannerStatusProblem && !state.LastScanAt.IsZero() {
			status = StatusError
		}
		return Item{
			Check:   "Last successful scan",
			Status:  status,
			Details: "No successful scan has been recorded in this runtime",
		}
	}

	return Item{
		Check:   "Last successful scan",
		Status:  StatusOK,
		Details: formatTime(state.LastSuccessfulScanAt),
	}
}

func scanDurationCheck(state routines.ScannerState) Item {
	if state.LastScanAt.IsZero() {
		return Item{
			Check:   "Scan duration",
			Status:  StatusWarning,
			Details: "No completed scan is available",
		}
	}

	return Item{
		Check:   "Scan duration",
		Status:  StatusOK,
		Details: formatDuration(state.Duration),
	}
}

func scannerErrorsCheck(state routines.ScannerState) Item {
	if len(state.LastErrors) == 0 {
		return Item{
			Check:   "Scanner errors",
			Status:  StatusOK,
			Details: "No scanner errors captured",
		}
	}

	parts := make([]string, 0, len(state.LastErrors))
	hasTimeout := false
	for _, scanErr := range state.LastErrors {
		detail := strings.TrimSpace(scanErr.Source)
		if detail == "" {
			detail = "scanner"
		}
		if scanErr.Message != "" {
			detail += ": " + scanErr.Message
		}
		if scanErr.Output != "" {
			detail += " — " + scanErr.Output
		}
		parts = append(parts, detail)
		if scanErr.Kind == arp.ScanErrorTimeout {
			hasTimeout = true
		}
	}

	fix := "Review the reported arp-scan command/output and the interface and permission checks above."
	if hasTimeout {
		fix = "Verify the interface is usable and arp-scan has the required privilege; then retry the scan."
	}
	return Item{
		Check:        "Scanner errors",
		Status:       StatusError,
		Details:      strings.Join(parts, "; "),
		SuggestedFix: fix,
	}
}

func databaseBackendLabel(backend string) string {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "sqlite":
		return "SQLite"
	case "postgres":
		return "PostgreSQL"
	default:
		if backend == "" {
			return "Database"
		}
		return backend
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	if value < time.Second {
		return fmt.Sprintf("%d ms", value.Milliseconds())
	}
	return fmt.Sprintf("%.1f sec", value.Seconds())
}

func defaultDependencies() dependencies {
	return dependencies{
		now:      func() time.Time { return time.Now().UTC() },
		lookPath: exec.LookPath,
		geteuid:  os.Geteuid,
		stat:     os.Stat,
		getCapabilities: func(path string) (string, error) {
			getcap, err := exec.LookPath("getcap")
			if err != nil {
				return "", errGetcapUnavailable
			}
			output, cmdErr := exec.Command(getcap, path).CombinedOutput()
			return strings.TrimSpace(string(output)), cmdErr
		},
		inspectInterface: inspectInterface,
		databaseHealth:   gdb.GetDatabaseHealth,
		scannerState:     routines.GetScannerState,
	}
}

func inspectInterface(name string) interfaceInfo {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return interfaceInfo{Error: err.Error()}
	}

	info := interfaceInfo{
		Exists:       true,
		Up:           iface.Flags&net.FlagUp != 0,
		Loopback:     iface.Flags&net.FlagLoopback != 0,
		HardwareAddr: iface.HardwareAddr.String(),
	}

	addrs, err := iface.Addrs()
	if err != nil {
		info.Error = err.Error()
		return info
	}
	for _, addr := range addrs {
		ip, _, parseErr := net.ParseCIDR(addr.String())
		if parseErr != nil {
			continue
		}
		if ip.To4() != nil && !ip.IsLoopback() {
			info.HasIPv4 = true
			break
		}
	}
	return info
}
