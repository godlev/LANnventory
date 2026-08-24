package routines

import (
	"log/slog"
	"time"

	"github.com/aceberg/WatchYourLAN/internal/conf"
	"github.com/aceberg/WatchYourLAN/internal/gdb"
)

// HistoryTrim - routine for History
func HistoryTrim() {

	go func() {
		for {
			time.Sleep(time.Duration(1) * time.Hour) // Every hour

			presenceHours := conf.AppConfig.TrimHist
			presenceCutoff := time.Now().Add(-time.Duration(presenceHours) * time.Hour)
			presenceDate := presenceCutoff.Format("2006-01-02 15:04:05")

			slog.Info("Removing all Presence before", "date", presenceDate)

			n := gdb.DeleteOldHistory(presenceDate)
			slog.Info("Removed records from Presence", "n", n)

			connectivityHours := conf.AppConfig.ConnectivityRetention
			if connectivityHours < 1 {
				connectivityHours = conf.AppConfig.TrimHist
			}
			connectivityCutoff := time.Now().Add(-time.Duration(connectivityHours) * time.Hour)
			connectivityDate := connectivityCutoff.Format("2006-01-02 15:04:05")

			n = gdb.DeleteOldConnectivityEvents(connectivityDate)
			slog.Info("Removed old Connectivity events", "n", n)
		}
	}()
}
