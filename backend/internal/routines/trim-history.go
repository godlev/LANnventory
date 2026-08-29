package routines

import (
	"context"
	"log/slog"
	"time"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/timestamp"
)

// HistoryTrim - routine for History
func HistoryTrim() {
	HistoryTrimContext(context.Background())
}

// HistoryTrimContext starts the history trim routine until ctx is cancelled.
func HistoryTrimContext(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(1) * time.Hour): // Every hour
			}

			trimHistory(time.Now())
		}
	}()
}

func trimHistory(now time.Time) {
	config := conf.GetAppConfig()
	presenceHours := config.TrimHist
	presenceCutoff := now.Add(-time.Duration(presenceHours) * time.Hour)
	presenceDate := timestamp.Format(presenceCutoff)

	slog.Info("Removing all Presence before", "date", presenceDate)

	n := gdb.DeleteOldHistory(presenceDate)
	slog.Info("Removed records from Presence", "n", n)

	connectivityHours := config.ConnectivityRetention
	if connectivityHours < 1 {
		connectivityHours = config.TrimHist
	}
	connectivityCutoff := now.Add(-time.Duration(connectivityHours) * time.Hour)
	connectivityDate := timestamp.Format(connectivityCutoff)

	n = gdb.DeleteOldConnectivityEvents(connectivityDate)
	slog.Info("Removed old Connectivity events", "n", n)
}
