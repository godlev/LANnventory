package api

import (
	"context"
	"sync"

	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoxsync"
)

var (
	proxmoxSchedulerMu sync.RWMutex
	proxmoxScheduler   *proxmoxsync.Scheduler
)

// StartProxmoxSyncScheduler starts the central automatic Proxmox scheduler.
// The returned stop function cancels it and waits for all in-flight workers.
func StartProxmoxSyncScheduler(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	scheduler := proxmoxsync.NewGDBScheduler(func(runCtx context.Context, config models.ProxmoxAPIConfig) error {
		return proxmoxSyncService.RunAutomatic(runCtx, config)
	})
	done := scheduler.Start(ctx)

	proxmoxSchedulerMu.Lock()
	proxmoxScheduler = scheduler
	proxmoxSchedulerMu.Unlock()

	return func() {
		cancel()
		<-done
		proxmoxSchedulerMu.Lock()
		if proxmoxScheduler == scheduler {
			proxmoxScheduler = nil
		}
		proxmoxSchedulerMu.Unlock()
	}
}

func notifyProxmoxSyncSchedulerConfigChanged() {
	proxmoxSchedulerMu.RLock()
	scheduler := proxmoxScheduler
	proxmoxSchedulerMu.RUnlock()
	if scheduler != nil {
		scheduler.NotifyConfigChanged()
	}
}
