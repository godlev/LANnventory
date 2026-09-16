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
	ScanErrorExecution ScanErrorKind = "execution"
	ScanErrorTimeout   ScanErrorKind = "timeout"
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
	Interfaces []string
	Errors     []ScanError
}

type commandResult struct {
	Output string
	Error  *ScanError
}

func scanIface(iface, scanArgs string) commandResult {
	args := []string{"-glNx"}
	args = append(args, strings.Fields(scanArgs)...)
	args = append(args, "-I", iface)

	result := commandRunner("arp-scan", args...)
	if result.Error != nil {
		scanErr := *result.Error
		scanErr.Source = iface
		result.Error = &scanErr
	}

	return result
}

func scanStr(str string) commandResult {
	args := strings.Fields(str)
	if len(args) == 0 {
		return commandResult{}
	}

	result := commandRunner("arp-scan", args...)
	if result.Error != nil {
		scanErr := *result.Error
		scanErr.Source = str
		result.Error = &scanErr
	}

	return result
}

func runCommand(name string, args ...string) commandResult {
	ctx, cancel := context.WithTimeout(context.Background(), scanCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	out, err := cmd.CombinedOutput()
	slog.Debug(cmd.String())

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
	result := ScanResult{
		Hosts:   []models.Host{},
		Success: true,
	}

	if ifaces != "" {
		for _, iface := range strings.Fields(ifaces) {
			result.Interfaces = appendUniqueInterface(result.Interfaces, iface)
			slog.Debug("Scanning interface " + iface)

			cmdResult := scanIface(iface, args)
			if cmdResult.Error != nil {
				result.Success = false
				result.Errors = append(result.Errors, *cmdResult.Error)
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

		scanArgs := strings.Fields(scanString)
		if iface := interfaceFromScanArgs(scanArgs); iface != "" {
			result.Interfaces = appendUniqueInterface(result.Interfaces, iface)
		}

		slog.Debug("Scanning string " + scanString)
		cmdResult := scanStr(scanString)
		if cmdResult.Error != nil {
			result.Success = false
			result.Errors = append(result.Errors, *cmdResult.Error)
			continue
		}

		slog.Debug("Found IPs: \n" + cmdResult.Output)
		result.Hosts = append(result.Hosts, parseOutput(cmdResult.Output, scanArgs[len(scanArgs)-1])...)
	}

	return result
}

// Scan preserves the existing scanner contract while callers migrate to ScanDetailed.
func Scan(ifaces, args string, strs []string) ([]models.Host, bool) {
	result := ScanDetailed(ifaces, args, strs)
	return result.Hosts, result.Success
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
