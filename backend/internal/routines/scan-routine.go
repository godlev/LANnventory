package routines

import (
	"context"
	"log/slog"
	"time"

	"github.com/godlev/LANnventory/internal/arp"
	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/influx"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/notify"
	"github.com/godlev/LANnventory/internal/prometheus"
)

var (
	scannerNow = func() time.Time {
		return time.Now().UTC()
	}
	scanNetwork           = arp.ScanDetailedContext
	processScanResultFunc = processScanResult
	waitForNextScan       = waitUntilNextScan
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

	foundHostsMap := make(map[string]models.Host)
	for _, fHost := range foundHosts {
		foundHostsMap[fHost.Mac] = fHost
	}

	compareHosts(foundHostsMap)
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

		fHost, exists := foundHostsMap[aHost.Mac]
		if exists {

			aHost.Iface = fHost.Iface
			aHost.IP = fHost.IP
			aHost.Date = fHost.Date
			aHost.Now = 1

			delete(foundHostsMap, aHost.Mac)

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

		fHost.Name, fHost.DNS = check.DNS(fHost)
		notify.Unknown(fHost) // Log and Shoutrrr

		gdb.Update("now", fHost)
		recordHostObservation(fHost)
		hosts := gdb.SelectByMAC("now", fHost.Mac)
		if len(hosts) > 0 {
			gdb.RecordHostEvent(hosts[0], models.EventDiscovered, "", "")
		}
	}
}

func recordHostObservation(host models.Host) {
	if err := gdb.RecordHostObservation(host.Mac, host.Date); err != nil {
		slog.Error("Failed to record host lifecycle observation", "mac", host.Mac, "date", host.Date, "err", err)
	}
}
