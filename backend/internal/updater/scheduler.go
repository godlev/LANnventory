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
}

type AutoScheduler struct {
	Config   func() AutoConfig
	Check    func(context.Context, string, string, bool) (Status, error)
	Schedule func(context.Context, string, string, string) (ApplyResult, error)
	Log      *slog.Logger
}

func NewAutoScheduler(
	service *Service,
	config func() AutoConfig,
) *AutoScheduler {
	return &AutoScheduler{
		Config:   config,
		Check:    service.Check,
		Schedule: service.Schedule,
		Log:      slog.Default(),
	}
}

func (s *AutoScheduler) Start(ctx context.Context) {
	go s.run(ctx)
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

	if _, err := s.Schedule(ctx, config.CurrentVersion, channel, status.LatestVersion); err != nil {
		s.log().Warn("Automatic update scheduling failed", "err", err)
		return err
	}

	return nil
}

func (s *AutoScheduler) NextInterval() time.Duration {
	return time.Duration(NormalizeAutoIntervalHours(s.readConfig().IntervalHours)) * time.Hour
}

func (s *AutoScheduler) run(ctx context.Context) {
	timer := time.NewTimer(s.NextInterval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_ = s.RunOnce(ctx)
			timer.Reset(s.NextInterval())
		}
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
