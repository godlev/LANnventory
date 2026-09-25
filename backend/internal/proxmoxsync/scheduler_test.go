package proxmoxsync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

type fakeScheduleStore struct {
	mu      sync.Mutex
	configs map[string]models.ProxmoxAPIConfig
}

func newFakeScheduleStore(configs ...models.ProxmoxAPIConfig) *fakeScheduleStore {
	store := &fakeScheduleStore{configs: make(map[string]models.ProxmoxAPIConfig)}
	for _, config := range configs {
		store.configs[config.HypervisorMac] = config
	}
	return store
}

func (s *fakeScheduleStore) ListAutomatic() ([]models.ProxmoxAPIConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]models.ProxmoxAPIConfig, 0, len(s.configs))
	for _, config := range s.configs {
		if config.Enabled && config.AutomaticSync {
			result = append(result, config)
		}
	}
	return result, nil
}

func (s *fakeScheduleStore) UpdateNextIfRevision(mac string, revision uint64, next string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, ok := s.configs[mac]
	if !ok || !config.Enabled || !config.AutomaticSync || config.ConfigRevision != revision {
		return false, nil
	}
	config.NextSyncAt = next
	s.configs[mac] = config
	return true, nil
}

func (s *fakeScheduleStore) config(mac string) models.ProxmoxAPIConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configs[mac]
}

func TestSchedulerInitializesMissingNextRunDeterministically(t *testing.T) {
	now := time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:01", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 60, ConfigRevision: 3,
	}
	store := newFakeScheduleStore(config)
	scheduler := NewScheduler(store, nil)

	got, err := scheduler.reconcileSchedules(now, true)
	if err != nil {
		t.Fatalf("reconcileSchedules: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("configs len = %d, want 1", len(got))
	}
	next, err := parseScheduleTime(store.config(config.HypervisorMac).NextSyncAt)
	if err != nil {
		t.Fatalf("parse next: %v", err)
	}
	want := now.Add(time.Hour).Add(deterministicStartupSpread(config.HypervisorMac))
	if !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next, want)
	}
}

func TestSchedulerStartupDefersOverdueRun(t *testing.T) {
	now := time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:02", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 15, ConfigRevision: 1,
		NextSyncAt: now.Add(-time.Hour).Format(time.RFC3339),
	}
	store := newFakeScheduleStore(config)
	scheduler := NewScheduler(store, nil)

	if _, err := scheduler.reconcileSchedules(now, true); err != nil {
		t.Fatalf("reconcileSchedules: %v", err)
	}
	next, err := parseScheduleTime(store.config(config.HypervisorMac).NextSyncAt)
	if err != nil {
		t.Fatalf("parse next: %v", err)
	}
	want := now.Add(deterministicStartupSpread(config.HypervisorMac))
	if !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next, want)
	}
}

func TestSchedulerDueRunAdvancesCadenceAndRunsOnce(t *testing.T) {
	now := time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)
	due := now.Add(-2 * time.Minute)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:03", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 15, ConfigRevision: 4, NextSyncAt: due.Format(time.RFC3339),
	}
	store := newFakeScheduleStore(config)
	run := make(chan models.ProxmoxAPIConfig, 1)
	scheduler := NewScheduler(store, func(_ context.Context, got models.ProxmoxAPIConfig) error {
		run <- got
		return nil
	})

	configs, err := store.ListAutomatic()
	if err != nil {
		t.Fatalf("ListAutomatic: %v", err)
	}
	if launched := scheduler.dispatchDue(context.Background(), configs, now); launched != 1 {
		t.Fatalf("launched = %d, want 1", launched)
	}
	select {
	case got := <-run:
		if got.HypervisorMac != config.HypervisorMac {
			t.Fatalf("run mac = %q", got.HypervisorMac)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduled run did not execute")
	}
	scheduler.wg.Wait()

	next, err := parseScheduleTime(store.config(config.HypervisorMac).NextSyncAt)
	if err != nil {
		t.Fatalf("parse next: %v", err)
	}
	want := nextScheduledAfter(due, now, 15*time.Minute)
	if !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next, want)
	}

	configs, _ = store.ListAutomatic()
	if launched := scheduler.dispatchDue(context.Background(), configs, now); launched != 0 {
		t.Fatalf("same due run launched again: %d", launched)
	}
}

func TestSchedulerBoundsConcurrency(t *testing.T) {
	now := time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)
	store := newFakeScheduleStore(
		models.ProxmoxAPIConfig{HypervisorMac: "AA:BB:CC:DD:EE:11", Enabled: true, AutomaticSync: true, SyncIntervalMinutes: 15, ConfigRevision: 1, NextSyncAt: now.Format(time.RFC3339)},
		models.ProxmoxAPIConfig{HypervisorMac: "AA:BB:CC:DD:EE:12", Enabled: true, AutomaticSync: true, SyncIntervalMinutes: 15, ConfigRevision: 1, NextSyncAt: now.Format(time.RFC3339)},
		models.ProxmoxAPIConfig{HypervisorMac: "AA:BB:CC:DD:EE:13", Enabled: true, AutomaticSync: true, SyncIntervalMinutes: 15, ConfigRevision: 1, NextSyncAt: now.Format(time.RFC3339)},
	)
	started := make(chan string, 3)
	release := make(chan struct{})
	scheduler := NewScheduler(store, func(ctx context.Context, config models.ProxmoxAPIConfig) error {
		started <- config.HypervisorMac
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	})
	scheduler.MaxConcurrency = 2

	configs, _ := store.ListAutomatic()
	if launched := scheduler.dispatchDue(context.Background(), configs, now); launched != 2 {
		t.Fatalf("launched = %d, want 2", launched)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("expected worker did not start")
		}
	}
	select {
	case extra := <-started:
		t.Fatalf("third worker started despite bound: %s", extra)
	default:
	}
	close(release)
	scheduler.wg.Wait()

	configs, _ = store.ListAutomatic()
	if launched := scheduler.dispatchDue(context.Background(), configs, now); launched != 1 {
		t.Fatalf("remaining launched = %d, want 1", launched)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("remaining due worker did not start")
	}
	scheduler.wg.Wait()
}

func TestSchedulerRevisionGuardRejectsStaleClaim(t *testing.T) {
	now := time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:21", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 60, ConfigRevision: 7, NextSyncAt: now.Format(time.RFC3339),
	}
	store := newFakeScheduleStore(config)
	configs, _ := store.ListAutomatic()

	store.mu.Lock()
	changed := store.configs[config.HypervisorMac]
	changed.ConfigRevision = 8
	changed.SyncIntervalMinutes = 360
	changed.NextSyncAt = ""
	store.configs[config.HypervisorMac] = changed
	store.mu.Unlock()

	runs := 0
	scheduler := NewScheduler(store, func(context.Context, models.ProxmoxAPIConfig) error {
		runs++
		return nil
	})
	if launched := scheduler.dispatchDue(context.Background(), configs, now); launched != 0 {
		t.Fatalf("stale config launched %d runs", launched)
	}
	if runs != 0 {
		t.Fatalf("stale config executed %d runs", runs)
	}
	if got := store.config(config.HypervisorMac); got.NextSyncAt != "" || got.ConfigRevision != 8 {
		t.Fatalf("stale scheduler overwrote current config: %+v", got)
	}
}

func TestSchedulerNotifyConfigChangedCoalesces(t *testing.T) {
	scheduler := NewScheduler(newFakeScheduleStore(), nil)
	for i := 0; i < 20; i++ {
		scheduler.NotifyConfigChanged()
	}
	if len(scheduler.configChanged) != 1 {
		t.Fatalf("configChanged len = %d, want 1", len(scheduler.configChanged))
	}
}

func TestNextScheduledAfterPreservesCadence(t *testing.T) {
	previous := time.Date(2026, 9, 23, 16, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 23, 18, 7, 0, 0, time.UTC)
	got := nextScheduledAfter(previous, now, time.Hour)
	want := time.Date(2026, 9, 23, 19, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}
}


func TestSchedulerStartupPreservesFutureNextRun(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	future := now.Add(37 * time.Minute)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:31", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 60, ConfigRevision: 2, NextSyncAt: future.Format(time.RFC3339),
	}
	store := newFakeScheduleStore(config)
	scheduler := NewScheduler(store, nil)

	if _, err := scheduler.reconcileSchedules(now, true); err != nil {
		t.Fatalf("reconcileSchedules: %v", err)
	}
	if got := store.config(config.HypervisorMac).NextSyncAt; got != config.NextSyncAt {
		t.Fatalf("future NextSyncAt changed on restart: got %q want %q", got, config.NextSyncAt)
	}
}

func TestSchedulerCancellationReleasesInFlightWorker(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	config := models.ProxmoxAPIConfig{
		HypervisorMac: "AA:BB:CC:DD:EE:32", Enabled: true, AutomaticSync: true,
		SyncIntervalMinutes: 15, ConfigRevision: 1, NextSyncAt: now.Format(time.RFC3339),
	}
	store := newFakeScheduleStore(config)
	started := make(chan struct{})
	finished := make(chan struct{})
	scheduler := NewScheduler(store, func(ctx context.Context, _ models.ProxmoxAPIConfig) error {
		close(started)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	configs, _ := store.ListAutomatic()
	if launched := scheduler.dispatchDue(ctx, configs, now); launched != 1 {
		t.Fatalf("launched = %d, want 1", launched)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scheduled worker did not start")
	}
	cancel()
	scheduler.wg.Wait()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("scheduled worker did not stop after context cancellation")
	}
	if len(scheduler.sem) != 0 {
		t.Fatalf("worker slot leaked after cancellation: %d", len(scheduler.sem))
	}
}


func TestNextScheduledAfterConfigChangeMatchesSchedulerInitialization(t *testing.T) {
	now := time.Date(2026, 9, 25, 18, 30, 0, 0, time.UTC)
	mac := "AA:BB:CC:DD:EE:51"

	got := NextScheduledAfterConfigChange(mac, 60, now)
	want := now.Add(time.Hour).Add(deterministicStartupSpread(mac))
	if !got.Equal(want) {
		t.Fatalf("next = %s, want %s", got, want)
	}

	fallback := NextScheduledAfterConfigChange(mac, 999, now)
	wantFallback := now.Add(time.Hour).Add(deterministicStartupSpread(mac))
	if !fallback.Equal(wantFallback) {
		t.Fatalf("fallback next = %s, want %s", fallback, wantFallback)
	}
}
