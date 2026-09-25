package proxmoxsync

import (
	"context"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

func TestSchedulerDisabledConfigurationsNeverRun(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)
	store := newFakeScheduleStore(
		models.ProxmoxAPIConfig{
			HypervisorMac: "AA:BB:CC:DD:EE:41", Enabled: true, AutomaticSync: false,
			SyncIntervalMinutes: 15, ConfigRevision: 2, NextSyncAt: now.Add(-time.Minute).Format(time.RFC3339),
		},
		models.ProxmoxAPIConfig{
			HypervisorMac: "AA:BB:CC:DD:EE:42", Enabled: false, AutomaticSync: true,
			SyncIntervalMinutes: 15, ConfigRevision: 3, NextSyncAt: now.Add(-time.Minute).Format(time.RFC3339),
		},
	)
	runs := 0
	scheduler := NewScheduler(store, func(context.Context, models.ProxmoxAPIConfig) error {
		runs++
		return nil
	})

	configs, err := scheduler.reconcileSchedules(now, false)
	if err != nil {
		t.Fatalf("reconcileSchedules: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("disabled configurations entered scheduler ownership: %+v", configs)
	}
	if launched := scheduler.dispatchDue(context.Background(), configs, now); launched != 0 {
		t.Fatalf("disabled configurations launched %d runs", launched)
	}
	if runs != 0 {
		t.Fatalf("disabled configurations executed %d runs", runs)
	}
}

func TestSchedulerIntervalChangeReschedulesFromCurrentConfiguration(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:43", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 60, ConfigRevision: 4, NextSyncAt: now.Add(20 * time.Minute).Format(time.RFC3339),
	}
	store := newFakeScheduleStore(config)
	scheduler := NewScheduler(store, nil)

	store.mu.Lock()
	changed := store.configs[config.HypervisorMac]
	changed.ConfigRevision = 5
	changed.SyncIntervalMinutes = 360
	changed.NextSyncAt = ""
	store.configs[config.HypervisorMac] = changed
	store.mu.Unlock()

	configs, err := scheduler.reconcileSchedules(now, false)
	if err != nil {
		t.Fatalf("reconcileSchedules: %v", err)
	}
	if len(configs) != 1 || configs[0].ConfigRevision != 5 {
		t.Fatalf("reconciled configs = %+v", configs)
	}
	next, err := parseScheduleTime(store.config(config.HypervisorMac).NextSyncAt)
	if err != nil {
		t.Fatalf("parse next: %v", err)
	}
	want := now.Add(6*time.Hour).Add(deterministicStartupSpread(config.HypervisorMac))
	if !next.Equal(want) {
		t.Fatalf("interval change next = %s, want %s", next, want)
	}
}

func TestSchedulerDisableWhileWaitingDropsFutureOwnership(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:44", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 15, ConfigRevision: 7, NextSyncAt: now.Add(5 * time.Minute).Format(time.RFC3339),
	}
	store := newFakeScheduleStore(config)
	runs := 0
	scheduler := NewScheduler(store, func(context.Context, models.ProxmoxAPIConfig) error {
		runs++
		return nil
	})

	store.mu.Lock()
	disabled := store.configs[config.HypervisorMac]
	disabled.AutomaticSync = false
	disabled.ConfigRevision = 8
	disabled.NextSyncAt = ""
	store.configs[config.HypervisorMac] = disabled
	store.mu.Unlock()
	scheduler.NotifyConfigChanged()

	configs, err := scheduler.reconcileSchedules(now.Add(10*time.Minute), false)
	if err != nil {
		t.Fatalf("reconcileSchedules: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("disabled configuration still scheduled: %+v", configs)
	}
	if launched := scheduler.dispatchDue(context.Background(), configs, now.Add(10*time.Minute)); launched != 0 {
		t.Fatalf("disabled configuration launched %d runs", launched)
	}
	if runs != 0 {
		t.Fatalf("disabled configuration executed %d runs", runs)
	}
}
