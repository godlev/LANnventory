package arp

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/models"
)

var scanCommandTimeout = 2 * time.Minute
var commandRunner = runCommand

type ScanErrorKind string

const (
	ScanErrorCanceled      ScanErrorKind = "canceled"
	ScanErrorConfiguration ScanErrorKind = "configuration"
	ScanErrorExecution     ScanErrorKind = "execution"
	ScanErrorTimeout       ScanErrorKind = "timeout"
)

// ScanError describes one failed arp-scan source without relying on log parsing.
type ScanError struct {
	Source  string
	Command string
	Kind    ScanErrorKind
	Message string
	Output  string
}

// ScanResult is the structured outcome of one scanner execution.
type ScanResult struct {
	Hosts      []models.Host
	Success    bool
	Canceled   bool
	Interfaces []string
	Errors     []ScanError
}

type commandResult struct {
	Output string
	Error  *ScanError
}

func scanIface(ctx context.Context, iface, scanArgs string) commandResult {
	args := []string{"-glNx"}
	args = append(args, strings.Fields(scanArgs)...)
	args = append(args, "-I", iface)

	result := commandRunner(ctx, "arp-scan", args...)
	if result.Error != nil {
		scanErr := *result.Error
		scanErr.Source = iface
		result.Error = &scanErr
	}

	return result
}

func scanStr(ctx context.Context, str string) commandResult {
	args := strings.Fields(str)
	if len(args) == 0 {
		return commandResult{}
	}

	result := commandRunner(ctx, "arp-scan", args...)
	if result.Error != nil {
		scanErr := *result.Error
		scanErr.Source = str
		result.Error = &scanErr
	}

	return result
}

func runCommand(parent context.Context, name string, args ...string) commandResult {
	ctx, cancel := context.WithTimeout(parent, scanCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	out, err := cmd.CombinedOutput()
	slog.Debug(cmd.String())

	if parent.Err() != nil {
		return commandResult{
			Error: &ScanError{
				Command: cmd.String(),
				Kind:    ScanErrorCanceled,
				Message: "scan canceled",
				Output:  strings.TrimSpace(string(out)),
			},
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		message := "command timed out after " + scanCommandTimeout.String()
		slog.Error("Command timed out", "cmd", cmd.String(), "timeout", scanCommandTimeout.String())
		return commandResult{
			Error: &ScanError{
				Command: cmd.String(),
				Kind:    ScanErrorTimeout,
				Message: message,
				Output:  strings.TrimSpace(string(out)),
			},
		}
	}

	if err != nil {
		check.IfError(err)
		return commandResult{
			Error: &ScanError{
				Command: cmd.String(),
				Kind:    ScanErrorExecution,
				Message: err.Error(),
				Output:  strings.TrimSpace(string(out)),
			},
		}
	}

	return commandResult{Output: string(out)}
}

func parseOutput(text, iface string) []models.Host {
	var foundHosts = []models.Host{}

	p := strings.Split(text, "\n")

	for _, host := range p {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}

		p := strings.Split(host, "	")
		if len(p) < 3 {
			slog.Warn("Ignoring malformed arp-scan row", "iface", iface, "row", host)
			continue
		}

		var oneHost models.Host
		oneHost.Iface = iface
		oneHost.IP = strings.TrimSpace(p[0])
		oneHost.Mac = strings.TrimSpace(p[1])
		oneHost.Hw = strings.TrimSpace(strings.Join(p[2:], "	"))
		if oneHost.IP == "" || oneHost.Mac == "" || oneHost.Hw == "" {
			slog.Warn("Ignoring incomplete arp-scan row", "iface", iface, "row", host)
			continue
		}
		oneHost.Date = time.Now().Format("2006-01-02 15:04:05")
		oneHost.Now = 1
		foundHosts = append(foundHosts, oneHost)
	}

	return foundHosts
}

// ScanDetailed scans all configured sources and returns structured execution metadata.
func ScanDetailed(ifaces, args string, strs []string) ScanResult {
	return ScanDetailedContext(context.Background(), ifaces, args, strs)
}

// ScanDetailedContext scans all configured sources and stops promptly when ctx is cancelled.
func ScanDetailedContext(ctx context.Context, ifaces, args string, strs []string) ScanResult {
	result := ScanResult{
		Hosts:   []models.Host{},
		Success: true,
	}

	if ctx.Err() != nil {
		result.Success = false
		result.Canceled = true
		return result
	}

	if !hasConfiguredScanSource(ifaces, strs) {
		result.Success = false
		result.Errors = append(result.Errors, ScanError{
			Source:  "configuration",
			Kind:    ScanErrorConfiguration,
			Message: "no scan source configured",
		})
		return result
	}

	if ifaces != "" {
		for _, iface := range strings.Fields(ifaces) {
			if ctx.Err() != nil {
				result.Success = false
				result.Canceled = true
				return result
			}

			result.Interfaces = appendUniqueInterface(result.Interfaces, iface)
			slog.Debug("Scanning interface " + iface)

			cmdResult := scanIface(ctx, iface, args)
			if cmdResult.Error != nil {
				result.Success = false
				result.Errors = append(result.Errors, *cmdResult.Error)
				if cmdResult.Error.Kind == ScanErrorCanceled {
					result.Canceled = true
					return result
				}
				continue
			}

			slog.Debug("Found IPs: \n" + cmdResult.Output)
			result.Hosts = append(result.Hosts, parseOutput(cmdResult.Output, iface)...)
		}
	}

	for _, scanString := range strs {
		scanString = strings.TrimSpace(scanString)
		if scanString == "" {
			continue
		}
		if ctx.Err() != nil {
			result.Success = false
			result.Canceled = true
			return result
		}

		scanArgs := strings.Fields(scanString)
		iface := interfaceForScanArgs(scanArgs)
		if iface != "" {
			result.Interfaces = appendUniqueInterface(result.Interfaces, iface)
		}

		slog.Debug("Scanning string " + scanString)
		cmdResult := scanStr(ctx, scanString)
		if cmdResult.Error != nil {
			result.Success = false
			result.Errors = append(result.Errors, *cmdResult.Error)
			if cmdResult.Error.Kind == ScanErrorCanceled {
				result.Canceled = true
				return result
			}
			continue
		}

		slog.Debug("Found IPs: \n" + cmdResult.Output)
		result.Hosts = append(result.Hosts, parseOutput(cmdResult.Output, iface)...)
	}

	return result
}

// Scan preserves the existing scanner contract while callers migrate to ScanDetailed.
func Scan(ifaces, args string, strs []string) ([]models.Host, bool) {
	result := ScanDetailed(ifaces, args, strs)
	return result.Hosts, result.Success
}

func hasConfiguredScanSource(ifaces string, strs []string) bool {
	if len(strings.Fields(ifaces)) > 0 {
		return true
	}
	for _, scanString := range strs {
		if strings.TrimSpace(scanString) != "" {
			return true
		}
	}
	return false
}

func interfaceForScanArgs(args []string) string {
	if iface := interfaceFromScanArgs(args); iface != "" {
		return iface
	}
	if len(args) == 0 {
		return ""
	}

	// Preserve the upstream ARP_STRS convention: without an explicit -I/--interface,
	// the final argument is used as the host interface label.
	return args[len(args)-1]
}

func interfaceFromScanArgs(args []string) string {
	for i, arg := range args {
		switch {
		case (arg == "-I" || arg == "--interface") && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(arg, "--interface="):
			return strings.TrimPrefix(arg, "--interface=")
		case strings.HasPrefix(arg, "-I") && len(arg) > 2:
			return strings.TrimPrefix(arg, "-I")
		}
	}

	return ""
}

func appendUniqueInterface(ifaces []string, iface string) []string {
	if iface == "" {
		return ifaces
	}
	for _, existing := range ifaces {
		if existing == iface {
			return ifaces
		}
	}
	return append(ifaces, iface)
}
