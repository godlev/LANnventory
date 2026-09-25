package proxmoxsync

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	defaultSchedulerConcurrency = 2
	schedulerRetryInterval      = 30 * time.Second
	startupSpreadMin            = 5 * time.Second
	startupSpreadWindow         = 55 * time.Second
)

type ScheduleStore interface {
	ListAutomatic() ([]models.ProxmoxAPIConfig, error)
	UpdateNextIfRevision(hypervisorMac string, configRevision uint64, nextSyncAt string) (bool, error)
}

type ScheduledRunFunc func(context.Context, models.ProxmoxAPIConfig) error

type Scheduler struct {
	Store          ScheduleStore
	Run            ScheduledRunFunc
	Now            func() time.Time
	MaxConcurrency int
	Log            *slog.Logger

	configChanged chan struct{}
	workerDone    chan struct{}
	sem           chan struct{}
	wg            sync.WaitGroup
}

func NewScheduler(store ScheduleStore, run ScheduledRunFunc) *Scheduler {
	return &Scheduler{
		Store:          store,
		Run:            run,
		Now:            time.Now,
		MaxConcurrency: defaultSchedulerConcurrency,
		Log:            slog.Default(),
		configChanged:  make(chan struct{}, 1),
		workerDone:     make(chan struct{}, 1),
	}
}

func NewGDBScheduler(run ScheduledRunFunc) *Scheduler {
	return NewScheduler(gdbScheduleStore{}, run)
}

func (s *Scheduler) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.run(ctx)
	}()
	return done
}

func (s *Scheduler) NotifyConfigChanged() {
	if s == nil || s.configChanged == nil {
		return
	}
	select {
	case s.configChanged <- struct{}{}:
	default:
	}
}

func (s *Scheduler) run(ctx context.Context) {
	s.ensureRuntime()
	startup := true
	for {
		now := s.nowUTC()
		configs, err := s.reconcileSchedules(now, startup)
		startup = false
		if err != nil {
			s.log().Warn("Failed to load Proxmox automatic sync schedule", "err", err)
			if !s.wait(ctx, schedulerRetryInterval) {
				s.wg.Wait()
				return
			}
			continue
		}

		s.dispatchDue(ctx, configs, now)

		configs, err = s.Store.ListAutomatic()
		if err != nil {
			s.log().Warn("Failed to reload Proxmox automatic sync schedule", "err", err)
			if !s.wait(ctx, schedulerRetryInterval) {
				s.wg.Wait()
				return
			}
			continue
		}

		delay, hasSchedule := nextWakeDelay(configs, s.nowUTC())
		if !hasSchedule {
			if !s.waitWithoutTimer(ctx) {
				s.wg.Wait()
				return
			}
			continue
		}
		if delay == 0 && len(s.sem) >= cap(s.sem) {
			if !s.waitWithoutTimer(ctx) {
				s.wg.Wait()
				return
			}
			continue
		}
		if !s.wait(ctx, delay) {
			s.wg.Wait()
			return
		}
	}
}

func (s *Scheduler) reconcileSchedules(now time.Time, startup bool) ([]models.ProxmoxAPIConfig, error) {
	s.ensureRuntime()
	configs, err := s.Store.ListAutomatic()
	if err != nil {
		return nil, err
	}
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].HypervisorMac < configs[j].HypervisorMac
	})

	for i := range configs {
		config := &configs[i]
		interval := scheduleInterval(config.SyncIntervalMinutes)
		next, parseErr := parseScheduleTime(config.NextSyncAt)

		needsInitialization := strings.TrimSpace(config.NextSyncAt) == "" || parseErr != nil
		overdueAtStartup := startup && parseErr == nil && !next.After(now)
		if !needsInitialization && !overdueAtStartup {
			continue
		}

		if overdueAtStartup {
			next = now.Add(deterministicStartupSpread(config.HypervisorMac))
		} else {
			next = NextScheduledAfterConfigChange(config.HypervisorMac, config.SyncIntervalMinutes, now)
		}
		nextValue := next.UTC().Format(time.RFC3339)
		updated, err := s.Store.UpdateNextIfRevision(config.HypervisorMac, config.ConfigRevision, nextValue)
		if err != nil {
			return nil, err
		}
		if updated {
			config.NextSyncAt = nextValue
		}
	}
	return configs, nil
}

func (s *Scheduler) dispatchDue(ctx context.Context, configs []models.ProxmoxAPIConfig, now time.Time) int {
	s.ensureRuntime()
	type dueConfig struct {
		config models.ProxmoxAPIConfig
		dueAt  time.Time
	}
	due := make([]dueConfig, 0)
	for _, config := range configs {
		next, err := parseScheduleTime(config.NextSyncAt)
		if err != nil || next.After(now) {
			continue
		}
		due = append(due, dueConfig{config: config, dueAt: next})
	}
	sort.Slice(due, func(i, j int) bool {
		if !due[i].dueAt.Equal(due[j].dueAt) {
			return due[i].dueAt.Before(due[j].dueAt)
		}
		return due[i].config.HypervisorMac < due[j].config.HypervisorMac
	})

	launched := 0
	for _, item := range due {
		if ctx.Err() != nil || !s.tryWorkerSlot() {
			break
		}

		next := nextScheduledAfter(item.dueAt, now, scheduleInterval(item.config.SyncIntervalMinutes))
		updated, err := s.Store.UpdateNextIfRevision(
			item.config.HypervisorMac,
			item.config.ConfigRevision,
			next.Format(time.RFC3339),
		)
		if err != nil {
			s.releaseWorkerSlot()
			s.log().Warn("Failed to claim Proxmox automatic sync", "mac", item.config.HypervisorMac, "err", err)
			continue
		}
		if !updated {
			s.releaseWorkerSlot()
			continue
		}

		launched++
		config := item.config
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.releaseWorkerSlot()
			if s.Run != nil {
				if err := s.Run(ctx, config); err != nil && ctx.Err() == nil {
					s.log().Warn("Proxmox automatic sync failed", "mac", config.HypervisorMac, "err", err)
				}
			}
			select {
			case s.workerDone <- struct{}{}:
			default:
			}
		}()
	}
	return launched
}

func (s *Scheduler) ensureRuntime() {
	if s.configChanged == nil {
		s.configChanged = make(chan struct{}, 1)
	}
	if s.workerDone == nil {
		s.workerDone = make(chan struct{}, 1)
	}
	max := s.MaxConcurrency
	if max < 1 {
		max = defaultSchedulerConcurrency
	}
	if s.sem == nil || cap(s.sem) != max {
		s.sem = make(chan struct{}, max)
	}
}

func (s *Scheduler) tryWorkerSlot() bool {
	select {
	case s.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Scheduler) releaseWorkerSlot() {
	select {
	case <-s.sem:
	default:
	}
}

func (s *Scheduler) wait(ctx context.Context, delay time.Duration) bool {
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	defer stopAndDrainSchedulerTimer(timer)
	select {
	case <-ctx.Done():
		return false
	case <-s.configChanged:
		return true
	case <-s.workerDone:
		return true
	case <-timer.C:
		return true
	}
}

func (s *Scheduler) waitWithoutTimer(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.configChanged:
		return true
	case <-s.workerDone:
		return true
	}
}

func (s *Scheduler) nowUTC() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Scheduler) log() *slog.Logger {
	if s != nil && s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// NextScheduledAfterConfigChange returns the deterministic first scheduled run
// after an automatic-sync configuration change. API responses and the scheduler
// use the same calculation so the persisted NextSyncAt is immediately truthful.
func NextScheduledAfterConfigChange(hypervisorMac string, intervalMinutes int, now time.Time) time.Time {
	return now.UTC().
		Add(scheduleInterval(intervalMinutes)).
		Add(deterministicStartupSpread(hypervisorMac))
}

func scheduleInterval(minutes int) time.Duration {
	switch minutes {
	case 15, 30, 60, 360, 720, 1440:
		return time.Duration(minutes) * time.Minute
	default:
		return time.Hour
	}
}

func parseScheduleTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.TrimSpace(value))
}

func nextScheduledAfter(previous, now time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		interval = time.Hour
	}
	if previous.After(now) {
		return previous.UTC()
	}
	elapsed := now.Sub(previous)
	steps := elapsed/interval + 1
	return previous.Add(steps * interval).UTC()
}

func deterministicStartupSpread(identity string) time.Duration {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.ToUpper(strings.TrimSpace(identity))))
	offset := time.Duration(hash.Sum32()%uint32(startupSpreadWindow/time.Second+1)) * time.Second
	return startupSpreadMin + offset
}

func nextWakeDelay(configs []models.ProxmoxAPIConfig, now time.Time) (time.Duration, bool) {
	var earliest time.Time
	for _, config := range configs {
		next, err := parseScheduleTime(config.NextSyncAt)
		if err != nil {
			continue
		}
		if earliest.IsZero() || next.Before(earliest) {
			earliest = next
		}
	}
	if earliest.IsZero() {
		return 0, false
	}
	delay := earliest.Sub(now)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func stopAndDrainSchedulerTimer(timer *time.Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

type gdbScheduleStore struct{}

func (gdbScheduleStore) ListAutomatic() ([]models.ProxmoxAPIConfig, error) {
	return gdb.SelectAutomaticProxmoxAPIConfigs()
}

func (gdbScheduleStore) UpdateNextIfRevision(mac string, revision uint64, nextSyncAt string) (bool, error) {
	return gdb.UpdateProxmoxAPINextSyncIfRevision(mac, revision, nextSyncAt)
}
