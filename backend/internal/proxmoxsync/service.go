package proxmoxsync

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
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
	ErrorBusy         ErrorKind = "busy"
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
	RequireEnabled         bool
	Trigger                Trigger
	ExpectedConfigRevision *uint64
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

	inFlightMu sync.Mutex
	inFlight   map[string]struct{}
}

func NewService() *Service {
	return &Service{
		now:      time.Now,
		inFlight: make(map[string]struct{}),
	}
}

func (s *Service) TestConnection(ctx context.Context, hypervisorMac string) (proxmoxapi.ConnectionTestResult, error) {
	client, _, err := s.configuredClient(hypervisorMac, false)
	if err != nil {
		return proxmoxapi.ConnectionTestResult{}, err
	}

	attemptedAt := s.nowUTC().Format(time.RFC3339)
	result, err := proxmoxapi.TestConnection(ctx, client)
	if err != nil {
		s.recordLegacyStatus(hypervisorMac, "error", attemptedAt, "", err.Error(), nil)
		return result, err
	}

	s.recordLegacyStatus(hypervisorMac, "connected", attemptedAt, "", "", nil)
	return result, nil
}

func (s *Service) CollectPreview(ctx context.Context, hypervisorMac string, options CollectOptions) (PreviewResult, error) {
	release, ok := s.tryBegin(hypervisorMac)
	if !ok {
		return PreviewResult{}, &ServiceError{Kind: ErrorBusy, Err: errors.New("sync already in progress")}
	}
	defer release()
	return s.collectPreview(ctx, hypervisorMac, options)
}

func (s *Service) collectPreview(ctx context.Context, hypervisorMac string, options CollectOptions) (PreviewResult, error) {
	client, config, err := s.configuredClient(hypervisorMac, options.RequireEnabled)
	if err != nil {
		return PreviewResult{}, err
	}
	expectedRevision := options.ExpectedConfigRevision
	if expectedRevision == nil {
		revision := config.ConfigRevision
		expectedRevision = &revision
	}
	if config.ConfigRevision != *expectedRevision {
		return PreviewResult{}, staleConfigError()
	}

	attemptedAt := s.nowUTC()
	attemptedValue := attemptedAt.Format(time.RFC3339)
	trigger := normalizedTrigger(options.Trigger)
	s.recordRuntime(hypervisorMac, expectedRevision, gdb.ProxmoxAPISyncRuntimeUpdate{
		LastSyncAttemptAt: ptrString(attemptedValue),
		LastSyncTrigger:   ptrString(string(trigger)),
	})

	snapshot, err := proxmoxapi.CollectSnapshot(ctx, client, attemptedAt)
	if err != nil {
		s.recordLegacyStatus(hypervisorMac, "error", attemptedValue, "", err.Error(), expectedRevision)
		s.recordRuntime(hypervisorMac, expectedRevision, gdb.ProxmoxAPISyncRuntimeUpdate{
			LastSyncStatus:  ptrString("error"),
			LastSyncError:   ptrString(err.Error()),
			LastSyncTrigger: ptrString(string(trigger)),
		})
		return PreviewResult{}, err
	}
	if err := s.ensureConfigRevision(hypervisorMac, expectedRevision); err != nil {
		return PreviewResult{}, err
	}

	current, err := loadCurrentState(hypervisorMac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil {
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return PreviewResult{}, &ServiceError{Kind: ErrorState, Err: err}
	}

	preview, err := proxmoximport.BuildPreview(hypervisorMac, snapshot, current)
	if err != nil {
		s.recordLegacyStatus(hypervisorMac, "error", attemptedValue, "", "collected Proxmox data failed validation", expectedRevision)
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return PreviewResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}
	if err := s.ensureConfigRevision(hypervisorMac, expectedRevision); err != nil {
		return PreviewResult{}, err
	}

	legacyStatus := "preview-ready"
	legacyError := ""
	runtimeStatus := "healthy"
	runtimeError := ""
	var successfulCollection *string
	if !snapshot.Complete {
		legacyStatus = "degraded"
		legacyError = strings.Join(snapshot.CollectionErrors, "; ")
		runtimeStatus = "error"
		runtimeError = legacyError
	} else {
		value := snapshot.CollectedAt
		successfulCollection = &value
		if !preview.ApplyAllowed {
			runtimeStatus = "review-required"
		}
	}

	s.recordLegacyStatus(hypervisorMac, legacyStatus, attemptedValue, "", legacyError, expectedRevision)
	s.recordRuntime(hypervisorMac, expectedRevision, gdb.ProxmoxAPISyncRuntimeUpdate{
		LastSuccessfulCollectionAt: successfulCollection,
		LastSyncStatus:              ptrString(runtimeStatus),
		LastSyncError:               ptrString(runtimeError),
		LastSyncTrigger:             ptrString(string(trigger)),
	})

	return PreviewResult{
		Snapshot:       snapshot,
		Preview:        preview,
		ConfigRevision: config.ConfigRevision,
		AttemptedAt:    attemptedValue,
	}, nil
}

func (s *Service) ApplyPreview(ctx context.Context, hypervisorMac string, snapshot proxmoxsnapshot.Snapshot, previewToken string, options ApplyOptions) (ApplyResult, error) {
	release, ok := s.tryBegin(hypervisorMac)
	if !ok {
		return ApplyResult{}, &ServiceError{Kind: ErrorBusy, Err: errors.New("sync already in progress")}
	}
	defer release()
	return s.applyPreview(ctx, hypervisorMac, snapshot, previewToken, options)
}

func (s *Service) applyPreview(ctx context.Context, hypervisorMac string, snapshot proxmoxsnapshot.Snapshot, previewToken string, options ApplyOptions) (ApplyResult, error) {
	_ = ctx
	trigger := normalizedTrigger(options.Trigger)

	if strings.TrimSpace(snapshot.Source) != proxmoxsnapshot.SourceProxmoxAPI {
		err := &ServiceError{Kind: ErrorValidation, Err: errors.New("Proxmox API sync requires a proxmox-api snapshot")}
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return ApplyResult{}, err
	}
	previewToken = strings.TrimSpace(previewToken)

	if err := s.ensureConfigRevision(hypervisorMac, expectedRevision); err != nil {
		return ApplyResult{}, err
	}

	normalized, err := proxmoximport.ValidateAndNormalize(snapshot)
	if err != nil {
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}
	current, err := loadCurrentState(hypervisorMac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil {
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return ApplyResult{}, &ServiceError{Kind: ErrorState, Err: err}
	}
	preview, err := proxmoximport.BuildPreview(hypervisorMac, normalized, current)
	if err != nil {
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}
	if previewToken != preview.PreviewToken {
		s.recordRuntimeReview(hypervisorMac, options.ExpectedConfigRevision, trigger)
		return ApplyResult{}, &ServiceError{Kind: ErrorStalePreview, Err: errors.New("preview is stale")}
	}
	if !preview.ApplyAllowed {
		s.recordRuntimeReview(hypervisorMac, options.ExpectedConfigRevision, trigger)
		return ApplyResult{}, &ServiceError{Kind: ErrorApplyBlocked, Err: errors.New("preview cannot be applied until blocking issues are resolved")}
	}

	inputs, err := proxmoximport.SnapshotWorkloadInputs(normalized)
	if err != nil {
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}

	importedAt := s.nowUTC().Format(time.RFC3339)
	state, err := proxmoximport.SourceStateFromSnapshot(hypervisorMac, normalized, preview.SnapshotDigest, importedAt)
	if err != nil {
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, err)
		return ApplyResult{}, &ServiceError{Kind: ErrorValidation, Err: err}
	}

	if err := s.ensureConfigRevision(hypervisorMac, expectedRevision); err != nil {
		return ApplyResult{}, err
	}

	var applyErr error
	if options.ExpectedConfigRevision != nil {
		applyErr = gdb.ApplyProxmoxAPIImportIfRevision(
			hypervisorMac,
			state,
			inputs,
			*options.ExpectedConfigRevision,
			trigger == TriggerAutomatic,
		)
	} else {
		applyErr = gdb.ApplyProxmoxImport(hypervisorMac, state, inputs)
	}
	if applyErr != nil {
		if errors.Is(applyErr, gdb.ErrProxmoxAPIConfigChanged) {
			return ApplyResult{}, staleConfigError()
		}
		if errors.Is(applyErr, gdb.ErrInfrastructureWorkloadSourceConflict) {
			s.recordRuntimeReview(hypervisorMac, options.ExpectedConfigRevision, trigger)
			return ApplyResult{}, &ServiceError{Kind: ErrorConflict, Err: applyErr}
		}
		s.recordRuntimeError(hypervisorMac, options.ExpectedConfigRevision, trigger, applyErr)
		return ApplyResult{}, &ServiceError{Kind: ErrorPersistence, Err: applyErr}
	}

	s.recordLegacyStatus(hypervisorMac, "connected", importedAt, importedAt, "", expectedRevision)
	s.recordRuntime(hypervisorMac, expectedRevision, gdb.ProxmoxAPISyncRuntimeUpdate{
		LastSyncStatus:  ptrString("healthy"),
		LastSyncError:   ptrString(""),
		LastSyncTrigger: ptrString(string(trigger)),
	})
	return ApplyResult{
		ImportedAt: importedAt,
		Summary:    preview.Summary,
	}, nil
}

// RunAutomatic executes one complete automatic sync under one same-host
// ownership lease. It never applies partial, conflicting, or ambiguous data.
func (s *Service) RunAutomatic(ctx context.Context, config models.ProxmoxAPIConfig) error {
	release, ok := s.tryBegin(config.HypervisorMac)
	if !ok {
		return &ServiceError{Kind: ErrorBusy, Err: errors.New("sync already in progress")}
	}
	defer release()

	expected := config.ConfigRevision
	if err := s.ensureAutomaticConfig(config.HypervisorMac, expected); err != nil {
		return err
	}

	result, err := s.collectPreview(ctx, config.HypervisorMac, CollectOptions{
		RequireEnabled:         true,
		Trigger:                TriggerAutomatic,
		ExpectedConfigRevision: &expected,
	})
	if err != nil {
		return err
	}
	if !result.Snapshot.Complete {
		return &ServiceError{Kind: ErrorApplyBlocked, Err: errors.New("automatic sync collected an incomplete snapshot")}
	}

	safety, err := s.evaluateAutomaticSafety(config.HypervisorMac, result.Snapshot, result.Preview)
	if err != nil {
		s.recordRuntimeError(config.HypervisorMac, &expected, TriggerAutomatic, err)
		return &ServiceError{Kind: ErrorState, Err: err}
	}
	if !safety.Safe {
		s.recordRuntimeReview(config.HypervisorMac, &expected, TriggerAutomatic)
		return nil
	}

	_, err = s.applyPreview(ctx, config.HypervisorMac, result.Snapshot, result.Preview.PreviewToken, ApplyOptions{
		Trigger:                TriggerAutomatic,
		ExpectedConfigRevision: &expected,
	})
	if err == nil {
		return nil
	}
	switch KindOf(err) {
	case ErrorStalePreview, ErrorApplyBlocked, ErrorConflict:
		s.recordRuntimeReview(config.HypervisorMac, &expected, TriggerAutomatic)
		return nil
	default:
		return err
	}
}

func (s *Service) IsRunning(hypervisorMac string) bool {
	key := syncKey(hypervisorMac)
	s.inFlightMu.Lock()
	defer s.inFlightMu.Unlock()
	_, exists := s.inFlight[key]
	return exists
}

func (s *Service) tryBegin(hypervisorMac string) (func(), bool) {
	key := syncKey(hypervisorMac)
	s.inFlightMu.Lock()
	if s.inFlight == nil {
		s.inFlight = make(map[string]struct{})
	}
	if _, exists := s.inFlight[key]; exists {
		s.inFlightMu.Unlock()
		return nil, false
	}
	s.inFlight[key] = struct{}{}
	s.inFlightMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.inFlightMu.Lock()
			delete(s.inFlight, key)
			s.inFlightMu.Unlock()
		})
	}, true
}

func syncKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (s *Service) configuredClient(hypervisorMac string, requireEnabled bool) (*proxmoxapi.Client, models.ProxmoxAPIConfig, error) {
	config, found, err := gdb.SelectProxmoxAPIConfig(hypervisorMac)
	if err != nil {
		return nil, models.ProxmoxAPIConfig{}, &ServiceError{Kind: ErrorConfig, Err: errors.New("failed to load Proxmox API configuration")}
	}
	if !found {
		return nil, models.ProxmoxAPIConfig{}, &ServiceError{Kind: ErrorConfig, Err: errors.New("Proxmox API is not configured")}
	}
	if requireEnabled && !config.Enabled {
		return nil, config, &ServiceError{Kind: ErrorConfig, Err: errors.New("Proxmox API integration is disabled")}
	}
	if strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.TokenID) == "" || config.TokenSecret == "" {
		return nil, config, &ServiceError{Kind: ErrorConfig, Err: errors.New("Proxmox API credentials are incomplete")}
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
		return staleConfigError()
	}
	return nil
}

func (s *Service) ensureAutomaticConfig(hypervisorMac string, expected uint64) error {
	config, found, err := gdb.SelectProxmoxAPIConfig(hypervisorMac)
	if err != nil {
		return &ServiceError{Kind: ErrorState, Err: err}
	}
	if !found || config.ConfigRevision != expected || !config.Enabled || !config.AutomaticSync {
		return staleConfigError()
	}
	return nil
}

func staleConfigError() error {
	return &ServiceError{Kind: ErrorStaleConfig, Err: errors.New("Proxmox API configuration changed during sync")}
}

func (s *Service) recordLegacyStatus(mac, status, lastAttempt, lastSuccess, lastError string, expected *uint64) {
	if expected != nil {
		updated, err := gdb.UpdateProxmoxAPIStatusIfRevision(mac, *expected, status, lastAttempt, lastSuccess, lastError)
		if err != nil {
			slog.Warn("Failed to persist Proxmox API source status", "mac", mac, "status", status, "err", err)
		} else if !updated {
			slog.Debug("Skipped stale Proxmox API source status update", "mac", mac, "status", status)
		}
		return
	}
	if err := gdb.UpdateProxmoxAPIStatus(mac, status, lastAttempt, lastSuccess, lastError); err != nil {
		slog.Warn("Failed to persist Proxmox API source status", "mac", mac, "status", status, "err", err)
	}
}

func (s *Service) recordRuntime(mac string, expected *uint64, update gdb.ProxmoxAPISyncRuntimeUpdate) {
	if expected != nil {
		updated, err := gdb.UpdateProxmoxAPISyncRuntimeIfRevision(mac, *expected, update)
		if err != nil {
			slog.Warn("Failed to persist Proxmox automatic sync runtime", "mac", mac, "err", err)
		} else if !updated {
			slog.Debug("Skipped stale Proxmox automatic sync runtime update", "mac", mac)
		}
		return
	}
	if err := gdb.UpdateProxmoxAPISyncRuntime(mac, update); err != nil {
		slog.Warn("Failed to persist Proxmox sync runtime", "mac", mac, "err", err)
	}
}

func (s *Service) recordRuntimeError(mac string, expected *uint64, trigger Trigger, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	s.recordRuntime(mac, expected, gdb.ProxmoxAPISyncRuntimeUpdate{
		LastSyncStatus:  ptrString("error"),
		LastSyncError:   ptrString(message),
		LastSyncTrigger: ptrString(string(normalizedTrigger(trigger))),
	})
}

func (s *Service) recordRuntimeReview(mac string, expected *uint64, trigger Trigger) {
	s.recordRuntime(mac, expected, gdb.ProxmoxAPISyncRuntimeUpdate{
		LastSyncStatus:  ptrString("review-required"),
		LastSyncError:   ptrString(""),
		LastSyncTrigger: ptrString(string(normalizedTrigger(trigger))),
	})
}

func normalizedTrigger(trigger Trigger) Trigger {
	if trigger == TriggerAutomatic {
		return TriggerAutomatic
	}
	return TriggerManual
}

func ptrString(value string) *string {
	return &value
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
