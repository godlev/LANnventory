package updater

import (
	"context"
	"log/slog"
	"time"
)

type AutoConfig struct {
	CurrentVersion string
	Channel        string
	Automatic      bool
	IntervalHours  int
	HealthURL      string
}

type AutoScheduler struct {
	Config   func() AutoConfig
	Check    func(context.Context, string, string, bool) (Status, error)
	Schedule func(context.Context, string, string, string, string) (ApplyResult, error)
	Log      *slog.Logger

	configChanged chan struct{}
}

func NewAutoScheduler(service *Service, config func() AutoConfig) *AutoScheduler {
	return &AutoScheduler{
		Config:        config,
		Check:         service.Check,
		Schedule:      service.Schedule,
		Log:           slog.Default(),
		configChanged: make(chan struct{}, 1),
	}
}

func (s *AutoScheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

// NotifyConfigChanged reschedules the next automatic update check from now
// using the latest persisted configuration. Notifications are coalesced so
// rapid Settings changes cannot block request handlers or queue stale timers.
func (s *AutoScheduler) NotifyConfigChanged() {
	if s == nil || s.configChanged == nil {
		return
	}
	select {
	case s.configChanged <- struct{}{}:
	default:
	}
}

func (s *AutoScheduler) RunOnce(ctx context.Context) error {
	config := s.readConfig()
	if !config.Automatic {
		return nil
	}

	channel := NormalizeChannel(config.Channel)
	status, err := s.Check(ctx, config.CurrentVersion, channel, true)
	if err != nil {
		s.log().Warn("Automatic update check failed", "err", err)
		return err
	}
	if status.Updating || !status.Available {
		return nil
	}
	if !status.InstallSupported {
		if status.InstallReason != "" {
			s.log().Info("Automatic update skipped", "reason", status.InstallReason)
		}
		return nil
	}

	if _, err := s.Schedule(ctx, config.CurrentVersion, channel, status.LatestVersion, config.HealthURL); err != nil {
		s.log().Warn("Automatic update scheduling failed", "err", err)
		return err
	}
	return nil
}

func (s *AutoScheduler) NextInterval() time.Duration {
	return time.Duration(NormalizeAutoIntervalHours(s.readConfig().IntervalHours)) * time.Hour
}

func (s *AutoScheduler) run(ctx context.Context) {
	for {
		timer := time.NewTimer(s.NextInterval())
		select {
		case <-ctx.Done():
			stopAndDrainTimer(timer)
			return
		case <-s.configChanged:
			stopAndDrainTimer(timer)
			continue
		case <-timer.C:
			_ = s.RunOnce(ctx)
		}
	}
}

func stopAndDrainTimer(timer *time.Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

func (s *AutoScheduler) readConfig() AutoConfig {
	if s.Config == nil {
		return AutoConfig{Channel: DefaultChannel, IntervalHours: 24}
	}
	config := s.Config()
	config.Channel = NormalizeChannel(config.Channel)
	config.IntervalHours = NormalizeAutoIntervalHours(config.IntervalHours)
	return config
}

func (s *AutoScheduler) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}
