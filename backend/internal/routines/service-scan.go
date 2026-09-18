package routines

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/portscan"
	"github.com/godlev/LANnventory/internal/servicescan"
)

const (
	serviceScanPollInterval = 30 * time.Second
	serviceScanBatchSize    = 32
	serviceScanWorkers      = 32
)

var (
	scheduledServiceScanNow = time.Now
	scheduledScanPorts      = portscan.ScanPorts

	serviceScanRestartMu sync.Mutex
	serviceScanCancel    context.CancelFunc
	serviceScanDone      chan struct{}
	startServiceScanFunc = startServiceScan
)

// ServiceScanRestart stops any existing scheduled service scanner before
// starting a replacement. The database must already be available.
func ServiceScanRestart() {
	serviceScanRestartMu.Lock()
	defer serviceScanRestartMu.Unlock()

	stopActiveServiceScanLocked()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	serviceScanCancel = cancel
	serviceScanDone = done

	go func() {
		defer close(done)
		startServiceScanFunc(ctx)
	}()
}

// ServiceScanStop cancels the scheduled service scanner and waits for it.
func ServiceScanStop() {
	serviceScanRestartMu.Lock()
	defer serviceScanRestartMu.Unlock()
	stopActiveServiceScanLocked()
}

func stopActiveServiceScanLocked() {
	if serviceScanCancel == nil {
		return
	}

	serviceScanCancel()
	if serviceScanDone != nil {
		<-serviceScanDone
	}
	serviceScanCancel = nil
	serviceScanDone = nil
}

func startServiceScan(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		runDueServiceScansAt(ctx, scheduledServiceScanNow())

		timer := time.NewTimer(serviceScanPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
	}
}

func runDueServiceScansAt(ctx context.Context, now time.Time) int {
	dueAt := now.Format(models.HostEventDateLayout)
	settings, err := gdb.SelectDueServiceScanSettings(dueAt, serviceScanBatchSize)
	if err != nil {
		slog.Error("Failed to load due service scans", "err", err)
		return 0
	}

	attempted := 0
	for _, setting := range settings {
		if ctx.Err() != nil {
			return attempted
		}
		attempted++
		runScheduledServiceScan(ctx, setting, now)
	}
	return attempted
}

func runScheduledServiceScan(ctx context.Context, settings models.ServiceScanSettings, now time.Time) {
	if ctx.Err() != nil {
		return
	}

	attemptAt := now.Format(models.HostEventDateLayout)
	nextAt := now.Add(time.Duration(settings.IntervalMinutes) * time.Minute).Format(models.HostEventDateLayout)

	ports, err := servicescan.DecodePortsJSON(settings.PortsJSON)
	if err != nil || len(ports) == 0 {
		updateScheduledServiceScanRuntime(settings.Mac, nextAt, attemptAt, "scheduled scan has invalid or empty port configuration", false)
		return
	}

	host, ok := currentHostForScheduledServiceScan(settings.Mac)
	if !ok {
		updateScheduledServiceScanRuntime(settings.Mac, nextAt, attemptAt, "host is no longer present in current inventory", false)
		return
	}

	addresses, err := activeAddressesForScheduledServiceScan(host)
	if err != nil {
		updateScheduledServiceScanRuntime(settings.Mac, nextAt, attemptAt, "failed to load active host addresses", false)
		return
	}
	if len(addresses) == 0 {
		updateScheduledServiceScanRuntime(settings.Mac, nextAt, attemptAt, "host has no active addresses", false)
		return
	}

	totalProbes := 0
	indeterminate := 0
	persistenceFailures := 0

	for _, address := range addresses {
		if ctx.Err() != nil {
			return
		}

		results := scheduledScanPorts(ctx, scheduledProbeTarget(address), ports, serviceScanWorkers)
		eventHost := host
		eventHost.IP = address.Address
		if strings.TrimSpace(address.Iface) != "" {
			eventHost.Iface = address.Iface
		}

		for _, result := range results {
			if ctx.Err() != nil || result.Result.State == portscan.ProbeCanceled {
				return
			}
			totalProbes++

			if result.Result.State == portscan.ProbeIndeterminate {
				indeterminate++
				continue
			}
			if result.Result.State != portscan.ProbeOpen && result.Result.State != portscan.ProbeClosed {
				indeterminate++
				continue
			}

			if _, _, _, err := gdb.RecordHostServiceObservation(eventHost, models.Service{
				Mac:            settings.Mac,
				Address:        address.Address,
				Protocol:       string(models.ServiceProtocolTCP),
				Port:           result.Port,
				State:          string(result.Result.State),
				LastScanSource: "scheduled",
			}, attemptAt); err != nil {
				persistenceFailures++
				slog.Error(
					"Failed to persist scheduled service observation",
					"mac", settings.Mac,
					"address", address.Address,
					"port", result.Port,
					"err", err,
				)
			}
		}
	}

	if totalProbes == 0 {
		updateScheduledServiceScanRuntime(settings.Mac, nextAt, attemptAt, "scheduled scan produced no probe results", false)
		return
	}

	errors := make([]string, 0, 2)
	if indeterminate > 0 {
		errors = append(errors, fmt.Sprintf("%d of %d probes indeterminate", indeterminate, totalProbes))
	}
	if persistenceFailures > 0 {
		errors = append(errors, fmt.Sprintf("%d service observations could not be persisted", persistenceFailures))
	}

	successful := len(errors) == 0
	updateScheduledServiceScanRuntime(settings.Mac, nextAt, attemptAt, strings.Join(errors, "; "), successful)
}

func currentHostForScheduledServiceScan(mac string) (models.Host, bool) {
	hosts := gdb.SelectByMAC("now", mac)
	for _, host := range hosts {
		if host.ID > 0 {
			return host, true
		}
	}
	return models.Host{}, false
}

func activeAddressesForScheduledServiceScan(host models.Host) ([]models.HostAddress, error) {
	retained, err := gdb.SelectHostAddressesByMAC(host.Mac)
	if err != nil {
		return nil, err
	}

	active := make([]models.HostAddress, 0, len(retained))
	for _, address := range retained {
		if address.Active {
			active = append(active, address)
		}
	}
	if len(active) > 0 {
		return active, nil
	}

	if host.Now == 1 && net.ParseIP(strings.TrimSpace(host.IP)) != nil {
		return []models.HostAddress{{
			Mac:     host.Mac,
			Address: strings.TrimSpace(host.IP),
			Iface:   strings.TrimSpace(host.Iface),
			Active:  true,
		}}, nil
	}
	return active, nil
}

func scheduledProbeTarget(address models.HostAddress) string {
	ip := net.ParseIP(strings.TrimSpace(address.Address))
	if ip != nil && ip.To4() == nil && ip.IsLinkLocalUnicast() && strings.TrimSpace(address.Iface) != "" {
		return strings.TrimSpace(address.Address) + "%" + strings.TrimSpace(address.Iface)
	}
	return strings.TrimSpace(address.Address)
}

func updateScheduledServiceScanRuntime(mac, nextAt, attemptAt, lastError string, successful bool) {
	if err := gdb.UpdateServiceScanRuntime(mac, nextAt, attemptAt, lastError, successful); err != nil {
		slog.Error("Failed to update scheduled service scan runtime", "mac", mac, "err", err)
	}
}
