package routines

import (
	"context"
	"log/slog"
	"sync"

	"github.com/godlev/LANnventory/internal/conf"
)

var (
	scanRestartMu sync.Mutex
	scanCancel    context.CancelFunc
	scanDone      chan struct{}
	startScanFunc = startScan
)

// ScanRestart stops any existing scanner routine before starting a replacement.
func ScanRestart() {
	scanRestartMu.Lock()
	defer scanRestartMu.Unlock()

	stopActiveScanLocked()

	slog.Info("Restarting scan routine")
	setLogLevel()
	markScannerStarting()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	scanCancel = cancel
	scanDone = done

	go func() {
		defer close(done)
		startScanFunc(ctx)
	}()
}

// ScanStop cancels the active scanner routine and waits for it to stop before returning.
func ScanStop() {
	scanRestartMu.Lock()
	defer scanRestartMu.Unlock()

	stopActiveScanLocked()
}

func stopActiveScanLocked() {
	if scanCancel == nil {
		return
	}

	scanCancel()
	if scanDone != nil {
		<-scanDone
	}

	scanCancel = nil
	scanDone = nil
}

func setLogLevel() {
	var level slog.Level
	config := conf.GetAppConfig()

	slog.Info("Log level: " + config.LogLevel)

	switch config.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		slog.Error("Invalid log level. Setting default level INFO")
		level = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(level)
}
