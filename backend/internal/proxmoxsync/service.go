package proxmoxsync

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoxapi"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

type Trigger string

const (
	TriggerManual    Trigger = "manual"
	TriggerAutomatic Trigger = "automatic"
)

type ErrorKind string

const (
	ErrorConfig       ErrorKind = "config"
	ErrorState        ErrorKind = "state"
	ErrorValidation   ErrorKind = "validation"
	ErrorStalePreview ErrorKind = "stale-preview"
	ErrorApplyBlocked ErrorKind = "apply-blocked"
	ErrorConflict     ErrorKind = "conflict"
	ErrorPersistence  ErrorKind = "persistence"
	ErrorStaleConfig  ErrorKind = "stale-config"
)

type ServiceError struct {
	Kind ErrorKind
	Err  error
}

func (e *ServiceError) Error() string {
	if e == nil || e.Err == nil {
		return "Proxmox sync failed"
	}
	return e.Err.Error()
}

func (e *ServiceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func KindOf(err error) ErrorKind {
	var syncErr *ServiceError
	if errors.As(err, &syncErr) {
		return syncErr.Kind
	}
	return ""
}

type CollectOptions struct {
	RequireEnabled bool
	Trigger        Trigger
}

type ApplyOptions struct {
	Trigger                Trigger
	ExpectedConfigRevision *uint64
}

type PreviewResult struct {
	Snapshot       proxmoxsnapshot.Snapshot
	Preview        proxmoximport.Preview
	ConfigRevision uint64
	AttemptedAt    string
}

type ApplyResult struct {
	ImportedAt string
	Summary    proxmoximport.PreviewSummary
}

type Service struct {
	now func() time.Time
}

func NewService() *Service {
	return &Service{now: time.Now}
}

func (s *Service) TestConnection(ctx context.Context, hypervisorMac string) (proxmoxapi.ConnectionTestResult, error) {
	client, _, err := s.configuredClient(hypervisorMac, false)
	if err != nil {
		return proxmoxapi.ConnectionTestResult{}, err
	}

	attemptedAt := s.nowUTC().Format(time.RFC3339)
	result, err := proxmoxapi.TestConnection(ctx, client)
	if err != nil {
		s.recordLegacyStatus(hypervisorMac, "error", attemptedAt, "", err.Error())
		return result, err
	}

	s.recordLegacyStatus(hypervisorMac, "connected", attemptedAt, "", "")
	return result, nil
}

func (s *Service) CollectPreview(ctx context.Context, hypervisorMac string, options CollectOptions) (PreviewResult, error) {
	client, config, err := s.configuredClient(hypervisorMac, options.RequireEnabled)
	if err != nil {
		return PreviewResult{}, err
	}

	attemptedAt := s.nowUTC()
	snapshot, err := proxmoxapi.CollectSnapshot(ctx, client, attemptedAt)
	if err != nil {
		s.recordLegacyStatus(hypervisorMac, "error", attemptedAt.Format(time.RFC3339), "", err.Error())
		return PreviewResult{}, err
	}

	current, err := loadCurrentState(hypervisorMac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil {
		return PreviewResult{}, &ServiceError{Kind: ErrorState, Err: err}
	}

	preview, err := proxmoximport.BuildPreview(hypervisorMac, snapshot, current)
	if err != nil {
		s.recordLegacyStatus(hypervisorMac, "error", attemptedAt.Format(time.RFC3339), "", "collected Proxmox data failed validation")
		return PreviewResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}

	status := "preview-ready"
	lastError := ""
	if !snapshot.Complete {
		status = "degraded"
		lastError = strings.Join(snapshot.CollectionErrors, "; ")
	}
	s.recordLegacyStatus(hypervisorMac, status, attemptedAt.Format(time.RFC3339), "", lastError)

	return PreviewResult{
		Snapshot:       snapshot,
		Preview:        preview,
		ConfigRevision: config.ConfigRevision,
		AttemptedAt:    attemptedAt.Format(time.RFC3339),
	}, nil
}

func (s *Service) ApplyPreview(ctx context.Context, hypervisorMac string, snapshot proxmoxsnapshot.Snapshot, previewToken string, options ApplyOptions) (ApplyResult, error) {
	_ = ctx

	if strings.TrimSpace(snapshot.Source) != proxmoxsnapshot.SourceProxmoxAPI {
		return ApplyResult{}, &ServiceError{
			Kind: ErrorValidation,
			Err:  errors.New("Proxmox API sync requires a proxmox-api snapshot"),
		}
	}
	previewToken = strings.TrimSpace(previewToken)

	if err := s.ensureConfigRevision(hypervisorMac, options.ExpectedConfigRevision); err != nil {
		return ApplyResult{}, err
	}

	normalized, err := proxmoximport.ValidateAndNormalize(snapshot)
	if err != nil {
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}
	current, err := loadCurrentState(hypervisorMac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil {
		return ApplyResult{}, &ServiceError{Kind: ErrorState, Err: err}
	}
	preview, err := proxmoximport.BuildPreview(hypervisorMac, normalized, current)
	if err != nil {
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}
	if previewToken != preview.PreviewToken {
		return ApplyResult{}, &ServiceError{
			Kind: ErrorStalePreview,
			Err:  errors.New("preview is stale"),
		}
	}
	if !preview.ApplyAllowed {
		return ApplyResult{}, &ServiceError{
			Kind: ErrorApplyBlocked,
			Err:  errors.New("preview cannot be applied until blocking issues are resolved"),
		}
	}

	inputs, err := proxmoximport.SnapshotWorkloadInputs(normalized)
	if err != nil {
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}

	importedAt := s.nowUTC().Format(time.RFC3339)
	state, err := proxmoximport.SourceStateFromSnapshot(hypervisorMac, normalized, preview.SnapshotDigest, importedAt)
	if err != nil {
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}

	if err := s.ensureConfigRevision(hypervisorMac, options.ExpectedConfigRevision); err != nil {
		return ApplyResult{}, err
	}

	if err := gdb.ApplyProxmoxImport(hypervisorMac, state, inputs); err != nil {
		if errors.Is(err, gdb.ErrInfrastructureWorkloadSourceConflict) {
			return ApplyResult{}, &ServiceError{Kind: ErrorConflict, Err: err}
		}
		return ApplyResult{}, &ServiceError{Kind: ErrorPersistence, Err: err}
	}

	s.recordLegacyStatus(hypervisorMac, "connected", importedAt, importedAt, "")
	return ApplyResult{
		ImportedAt: importedAt,
		Summary:    preview.Summary,
	}, nil
}

func (s *Service) configuredClient(hypervisorMac string, requireEnabled bool) (*proxmoxapi.Client, models.ProxmoxAPIConfig, error) {
	config, found, err := gdb.SelectProxmoxAPIConfig(hypervisorMac)
	if err != nil {
		return nil, models.ProxmoxAPIConfig{}, &ServiceError{
			Kind: ErrorConfig,
			Err:  errors.New("failed to load Proxmox API configuration"),
		}
	}
	if !found {
		return nil, models.ProxmoxAPIConfig{}, &ServiceError{
			Kind: ErrorConfig,
			Err:  errors.New("Proxmox API is not configured"),
		}
	}
	if requireEnabled && !config.Enabled {
		return nil, config, &ServiceError{
			Kind: ErrorConfig,
			Err:  errors.New("Proxmox API integration is disabled"),
		}
	}
	if strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.TokenID) == "" || config.TokenSecret == "" {
		return nil, config, &ServiceError{
			Kind: ErrorConfig,
			Err:  errors.New("Proxmox API credentials are incomplete"),
		}
	}

	client, err := proxmoxapi.New(proxmoxapi.Config{
		BaseURL:     config.BaseURL,
		TokenID:     config.TokenID,
		TokenSecret: config.TokenSecret,
		VerifyTLS:   config.VerifyTLS,
		Timeout:     time.Duration(config.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, config, &ServiceError{Kind: ErrorConfig, Err: err}
	}
	return client, config, nil
}

func (s *Service) ensureConfigRevision(hypervisorMac string, expected *uint64) error {
	if expected == nil {
		return nil
	}
	config, found, err := gdb.SelectProxmoxAPIConfig(hypervisorMac)
	if err != nil {
		return &ServiceError{Kind: ErrorState, Err: err}
	}
	if !found || config.ConfigRevision != *expected {
		return &ServiceError{
			Kind: ErrorStaleConfig,
			Err:  errors.New("Proxmox API configuration changed during sync"),
		}
	}
	return nil
}

func (s *Service) recordLegacyStatus(mac, status, lastAttempt, lastSuccess, lastError string) {
	if err := gdb.UpdateProxmoxAPIStatus(mac, status, lastAttempt, lastSuccess, lastError); err != nil {
		slog.Warn("Failed to persist Proxmox API source status", "mac", mac, "status", status, "err", err)
	}
}

func (s *Service) nowUTC() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func loadCurrentState(mac, source string) (proxmoximport.CurrentState, error) {
	var current proxmoximport.CurrentState

	state, found, err := gdb.SelectProxmoxSourceState(mac, source)
	if err != nil {
		return current, err
	}
	if found {
		current.SourceState = &state
	}

	managed, found, err := gdb.SelectHypervisorProfileByMAC(mac)
	if err != nil {
		return current, err
	}
	if found {
		current.ManagedHypervisor = &managed
	}

	workloads, err := gdb.SelectInfrastructureWorkloadsByHypervisorMAC(mac)
	if err != nil {
		return current, err
	}
	current.Workloads = workloads
	return current, nil
}
