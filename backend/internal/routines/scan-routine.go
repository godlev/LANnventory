package routines

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/arp"
	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/discovery"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/influx"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/notify"
	"github.com/godlev/LANnventory/internal/prometheus"
)

var (
	scannerNow = func() time.Time {
		return time.Now().UTC()
	}
	scanNetwork            = arp.ScanDetailedContext
	lookupDNS              = check.DNS
	localHostnameDiscovery = discovery.LocalHostnames
	ssdpDiscovery          = discovery.DiscoverSSDP
	processScanResultFunc  = processScanResult
	waitForNextScan        = waitUntilNextScan
)

func startScan(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		config := conf.GetAppConfig()
		started := scannerNow()
		markScanStarted(started)

		scanResult := scanNetwork(ctx, config.Ifaces, config.ArpArgs, config.ArpStrs)
		if ctx.Err() != nil || scanResult.Canceled {
			return
		}

		applied := processScanResultFunc(scanResult.Hosts, scanResult.Success)
		completed := scannerNow()

		interval := time.Duration(config.Timeout) * time.Second
		if interval <= 0 {
			interval = time.Second
		}
		next := completed.Add(interval)
		markScanCompleted(scanResult, started, completed, next, applied)

		if !waitForNextScan(ctx, next) {
			return
		}
	}
}

func waitUntilNextScan(ctx context.Context, next time.Time) bool {
	delay := time.Until(next)
	if delay < 0 {
		delay = 0
	}

	timer := time.NewTimer(delay)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func processScanResult(foundHosts []models.Host, scanOK bool) bool {
	if !scanOK {
		slog.Warn("Skipping host state update because ARP scan failed")
		return false
	}

	if err := gdb.RecordHostAddressObservations(foundHosts); err != nil {
		slog.Error("Failed to record host address observations", "err", err)
	}
	recordScannerDiscoveryEvidence(foundHosts)

	foundHostsMap := make(map[string]models.Host)
	for _, fHost := range foundHosts {
		key := identity.MACKey(fHost.Mac)
		if canonical, err := identity.NormalizeMAC(fHost.Mac); err == nil {
			fHost.Mac = canonical
		}
		foundHostsMap[key] = fHost
	}

	// Core host state, lifecycle and connectivity events are committed before
	// best-effort enrichment. Discovery failures must never change the success
	// semantics of an otherwise successful ARP scan.
	compareHosts(foundHostsMap)
	recordLocalHostnameDiscoveryEvidence(foundHosts)
	recordSSDPDiscoveryEvidence(foundHosts)
	return true
}

func compareHosts(foundHostsMap map[string]models.Host) {
	config := conf.GetAppConfig()

	allHosts, ok := gdb.Select("now")
	if !ok {
		return
	}

	for _, aHost := range allHosts {
		previousNow := aHost.Now
		aHostKey := identity.MACKey(aHost.Mac)

		fHost, exists := foundHostsMap[aHostKey]
		if exists {
			aHost.Iface = fHost.Iface
			aHost.IP = fHost.IP
			aHost.Date = fHost.Date
			aHost.Now = 1
			delete(foundHostsMap, aHostKey)
		} else {
			aHost.Now = 0
		}
		gdb.Update("now", aHost)
		if exists {
			recordHostObservation(aHost)
		}

		if exists && previousNow == 0 {
			gdb.RecordHostEvent(aHost, models.EventOnline, "", "")
		}
		if !exists && previousNow == 1 {
			gdb.RecordHostEvent(aHost, models.EventOffline, "", "")
		}

		aHost.ID = 0
		aHost.Date = time.Now().Format("2006-01-02 15:04:05")
		gdb.Update("history", aHost)

		if config.InfluxEnable {
			influx.Add(config, aHost)
		}
		if config.PrometheusEnable {
			prometheus.Add(aHost)
		}
	}

	for _, fHost := range foundHostsMap {
		_, fHost.DNS = lookupDNS(fHost)
		recordReverseDNSEvidence(fHost)
		notify.Unknown(fHost) // Log and Shoutrrr

		gdb.Update("now", fHost)
		recordHostObservation(fHost)
		hosts := gdb.SelectByMAC("now", fHost.Mac)
		if len(hosts) > 0 {
			gdb.RecordHostEvent(hosts[0], models.EventDiscovered, "", "")
		}
	}
}

func recordScannerDiscoveryEvidence(hosts []models.Host) {
	for _, host := range hosts {
		vendor := strings.TrimSpace(host.Hw)
		if vendor == "" {
			continue
		}
		if err := gdb.RecordHostDiscoveryEvidence(
			host.Mac,
			host.IP,
			models.DiscoverySourceScanner,
			models.DiscoveryKindVendor,
			[]string{vendor},
			host.Date,
		); err != nil {
			slog.Error("Failed to record scanner discovery evidence", "mac", host.Mac, "ip", host.IP, "err", err)
		}
	}
}

func recordLocalHostnameDiscoveryEvidence(hosts []models.Host) {
	for _, host := range hosts {
		if strings.TrimSpace(host.Date) == "" {
			continue
		}
		observations := localHostnameDiscovery(context.Background(), host.IP)
		for _, observation := range observations {
			if len(observation.Values) == 0 {
				continue
			}
			if err := gdb.RecordHostDiscoveryEvidence(
				host.Mac,
				host.IP,
				observation.Source,
				models.DiscoveryKindHostname,
				observation.Values,
				host.Date,
			); err != nil {
				slog.Error("Failed to record local hostname discovery evidence", "mac", host.Mac, "ip", host.IP, "source", observation.Source, "err", err)
			}
		}
	}
}

func recordSSDPDiscoveryEvidence(hosts []models.Host) {
	targetByAddress := make(map[string]models.Host, len(hosts))
	ambiguous := make(map[string]bool)
	addresses := make([]string, 0, len(hosts))

	for _, host := range hosts {
		if strings.TrimSpace(host.Date) == "" {
			continue
		}
		address, ok := canonicalObservedIP(host.IP)
		if !ok {
			continue
		}
		if existing, exists := targetByAddress[address]; exists {
			if identity.MACKey(existing.Mac) != identity.MACKey(host.Mac) {
				ambiguous[address] = true
			}
			continue
		}
		targetByAddress[address] = host
		addresses = append(addresses, address)
	}

	if len(addresses) == 0 {
		return
	}

	for _, observation := range ssdpDiscovery(context.Background(), addresses) {
		address, ok := canonicalObservedIP(observation.Address)
		if !ok || ambiguous[address] {
			continue
		}
		host, exists := targetByAddress[address]
		if !exists || len(observation.Values) == 0 {
			continue
		}
		if err := gdb.RecordHostDiscoveryEvidence(
			host.Mac,
			address,
			models.DiscoverySourceSSDP,
			observation.Kind,
			observation.Values,
			host.Date,
		); err != nil {
			slog.Error("Failed to record SSDP discovery evidence", "mac", host.Mac, "ip", address, "kind", observation.Kind, "err", err)
		}
	}
}

func canonicalObservedIP(value string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return "", false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String(), true
	}
	return ip.String(), true
}

func recordReverseDNSEvidence(host models.Host) {
	values := strings.Fields(host.DNS)
	if len(values) == 0 {
		return
	}
	if err := gdb.RecordHostDiscoveryEvidence(
		host.Mac,
		host.IP,
		models.DiscoverySourceReverseDNS,
		models.DiscoveryKindHostname,
		values,
		host.Date,
	); err != nil {
		slog.Error("Failed to record reverse-DNS discovery evidence", "mac", host.Mac, "ip", host.IP, "err", err)
	}
}

func recordHostObservation(host models.Host) {
	if err := gdb.RecordHostObservation(host.Mac, host.Date); err != nil {
		slog.Error("Failed to record host lifecycle observation", "mac", host.Mac, "date", host.Date, "err", err)
	}
}
