package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

const (
	DefaultChannel                   = "beta"
	StableChannel                    = "stable"
	BetaChannel                      = "beta"
	defaultReleasesURL               = "https://api.github.com/repos/godlev/LANnventory/releases?per_page=50"
	defaultReleaseNotesBaseURL       = "https://raw.githubusercontent.com/godlev/LANnventory/"
	releaseDownloadPrefix            = "https://github.com/godlev/LANnventory/releases/download/"
	cacheTTL                         = 15 * time.Minute
	maxReleaseBody             int64 = 4 << 20
	maxReleaseNotesBody        int64 = 256 << 10
	maxChecksumBody            int64 = 2 << 20
	maxPackageBody             int64 = 256 << 20
)

var (
	ErrInvalidChannel     = errors.New("invalid update channel")
	ErrNoUpdate           = errors.New("no update available")
	ErrUpdateInProgress   = errors.New("update already in progress")
	ErrUnsupportedInstall = errors.New("automatic install is not supported")
)

var validAutoIntervalHours = map[int]struct{}{
	6:   {},
	12:  {},
	24:  {},
	168: {},
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type release struct {
	TagName     string         `json:"tag_name"`
	Prerelease  bool           `json:"prerelease"`
	Draft       bool           `json:"draft"`
	PublishedAt string         `json:"published_at"`
	HTMLURL     string         `json:"html_url"`
	Assets      []releaseAsset `json:"assets"`
}

type Status struct {
	CurrentVersion      string `json:"currentVersion"`
	Channel             string `json:"channel"`
	LatestVersion       string `json:"latestVersion"`
	Available           bool   `json:"available"`
	PublishedAt         string `json:"publishedAt"`
	ReleaseURL          string `json:"releaseUrl"`
	ReleaseSummary      string `json:"releaseSummary"`
	InstallSupported    bool   `json:"installSupported"`
	InstallReason       string `json:"installReason"`
	Message             string `json:"message"`
	Updating            bool   `json:"updating"`
	AutomaticCheck      bool   `json:"automaticCheck"`
	Automatic           bool   `json:"automatic"`
	IntervalHours       int    `json:"intervalHours"`
	LastChecked         string `json:"lastChecked"`
	SnapshotBaseVersion string `json:"snapshotBaseVersion"`
}

type ApplyResult struct {
	Version    string `json:"version"`
	Scheduled  bool   `json:"scheduled"`
	Message    string `json:"message"`
	BackupPath string `json:"backupPath"`
}

type Service struct {
	client              *http.Client
	releasesURL         string
	releaseNotesBaseURL string
	progressPath        string
	now                 func() time.Time
	unitActive          func(string) bool
	serviceActive       func() bool

	mu               sync.Mutex
	cached           []release
	releaseSummaries map[string]string
	cacheUntil       time.Time
	lastChecked      time.Time
	updating         bool
}

func NewService() *Service {
	service := NewServiceWithURLAndProgressPath(&http.Client{Timeout: 30 * time.Second}, defaultReleasesURL, defaultProgressPath)
	service.releaseNotesBaseURL = defaultReleaseNotesBaseURL
	return service
}

func NewServiceWithURL(client *http.Client, releasesURL string) *Service {
	return NewServiceWithURLAndProgressPath(client, releasesURL, defaultProgressPath)
}

func NewServiceWithURLAndProgressPath(client *http.Client, releasesURL, progressPath string) *Service {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Service{
		client:           client,
		releasesURL:      releasesURL,
		progressPath:     progressPath,
		releaseSummaries: make(map[string]string),
		now:              time.Now,
		unitActive:       systemdUnitActive,
		serviceActive:    lannventoryServiceActive,
	}
}

func ValidChannel(channel string) bool {
	return channel == StableChannel || channel == BetaChannel
}

func NormalizeChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if ValidChannel(channel) {
		return channel
	}
	return DefaultChannel
}

func ValidAutoIntervalHours(hours int) bool {
	_, ok := validAutoIntervalHours[hours]
	return ok
}

func NormalizeAutoIntervalHours(hours int) int {
	if ValidAutoIntervalHours(hours) {
		return hours
	}
	return 24
}

func (s *Service) Check(ctx context.Context, currentVersion, channel string, refresh bool) (Status, error) {
	channel = NormalizeChannel(channel)
	releases, err := s.loadReleases(ctx, refresh)
	if err != nil {
		return Status{}, err
	}

	status, selected, ok := buildStatus(currentVersion, channel, releases, s.LastChecked(), s.isUpdating())
	if ok {
		status.ReleaseSummary = s.loadReleaseSummary(ctx, selected.TagName, refresh)
	}
	return status, nil
}

// CheckCached returns only status derived from release data already held in memory.
// It never contacts GitHub and is intended for lightweight UI notification polling.
func (s *Service) CheckCached(currentVersion, channel string) Status {
	channel = NormalizeChannel(channel)

	s.mu.Lock()
	releases := append([]release(nil), s.cached...)
	lastChecked := s.lastChecked
	summaries := make(map[string]string, len(s.releaseSummaries))
	for tag, summary := range s.releaseSummaries {
		summaries[tag] = summary
	}
	s.mu.Unlock()

	status, selected, ok := buildStatus(currentVersion, channel, releases, lastChecked, s.isUpdating())
	if ok {
		status.ReleaseSummary = summaries[selected.TagName]
	}
	if len(releases) == 0 && lastChecked.IsZero() {
		status.Message = "No update check has been performed yet."
	}
	return status
}

func buildStatus(currentVersion, channel string, releases []release, lastChecked time.Time, updating bool) (Status, release, bool) {
	selected, ok := selectLatestRelease(releases, channel)
	status := Status{CurrentVersion: currentVersion, Channel: channel, Updating: updating}
	if !lastChecked.IsZero() {
		status.LastChecked = lastChecked.Format(time.RFC3339)
	}
	if !ok {
		if channel == StableChannel {
			status.Message = "No stable release is published yet."
		} else {
			status.Message = "No release is published for this channel yet."
		}
		status.InstallReason = "No installable release is available."
		return status, release{}, false
	}

	status.LatestVersion = displayVersion(selected.TagName)
	status.PublishedAt = selected.PublishedAt
	status.ReleaseURL = selected.HTMLURL

	currentSemver := comparableVersion(currentVersion)
	latestSemver := comparableVersion(selected.TagName)
	if snapshotBase := snapshotBaseVersion(currentVersion); snapshotBase != "" {
		status.SnapshotBaseVersion = snapshotBase
	}
	if currentSemver == "" || latestSemver == "" {
		status.Message = "The running version cannot be compared automatically."
		status.InstallReason = "Version comparison is unavailable."
		return status, selected, true
	}

	status.Available = semver.Compare(latestSemver, currentSemver) > 0
	supported, reason, _, _ := detectInstallSupport(selected)
	status.InstallSupported = supported
	status.InstallReason = reason

	if status.Updating {
		status.Message = "An update has been scheduled and LANnventory will restart."
	} else if status.Available {
		status.Message = "Update " + status.LatestVersion + " is available."
	} else {
		status.Message = "No newer " + channelDisplayLabel(channel) + " release is available."
	}
	return status, selected, true
}

func (s *Service) LastChecked() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastChecked
}

func (s *Service) loadReleaseSummary(ctx context.Context, tagName string, refresh bool) string {
	if s.releaseNotesBaseURL == "" || comparableVersion(tagName) == "" {
		return ""
	}

	s.mu.Lock()
	if summary, ok := s.releaseSummaries[tagName]; ok && !refresh {
		s.mu.Unlock()
		return summary
	}
	s.mu.Unlock()

	tag := "v" + displayVersion(tagName)
	rawURL := strings.TrimRight(s.releaseNotesBaseURL, "/") + "/" + tag + "/docs/releases/" + tag + ".md"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "LANnventory-release-notes")

	resp, err := s.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseNotesBody+1))
	if err != nil || int64(len(body)) > maxReleaseNotesBody {
		return ""
	}

	summary := extractReleaseSummary(string(body))
	s.mu.Lock()
	s.releaseSummaries[tagName] = summary
	s.mu.Unlock()
	return summary
}

func extractReleaseSummary(markdown string) string {
	lines := strings.Split(markdown, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Summary" {
			start = i + 1
			break
		}
	}
	if start == -1 {
		for i, line := range lines {
			if strings.TrimSpace(line) == "## Highlights" {
				start = i + 1
				break
			}
		}
	}
	if start == -1 {
		return ""
	}

	summaryLines := make([]string, 0, 16)
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			break
		}
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			if trimmed != "" {
				summaryLines = append(summaryLines, trimmed+":")
			}
			continue
		}
		summaryLines = append(summaryLines, trimmed)
		if len(summaryLines) >= 24 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(summaryLines, "\n"))
}

func (s *Service) Schedule(ctx context.Context, currentVersion, channel, expectedVersion, healthURL string) (ApplyResult, error) {
	s.mu.Lock()
	if s.updating {
		s.mu.Unlock()
		return ApplyResult{}, ErrUpdateInProgress
	}
	s.updating = true
	s.mu.Unlock()

	scheduled := false
	defer func() {
		if scheduled {
			return
		}
		s.mu.Lock()
		s.updating = false
		s.mu.Unlock()
	}()

	channel = NormalizeChannel(channel)
	releases, err := s.loadReleases(ctx, true)
	if err != nil {
		return ApplyResult{}, err
	}
	selected, ok := selectLatestRelease(releases, channel)
	if !ok {
		return ApplyResult{}, ErrNoUpdate
	}

	targetVersion := displayVersion(selected.TagName)
	if expectedVersion != "" && displayVersion(expectedVersion) != targetVersion {
		return ApplyResult{}, fmt.Errorf("requested version is no longer the latest %s release", channel)
	}

	currentSemver := comparableVersion(currentVersion)
	latestSemver := comparableVersion(selected.TagName)
	if currentSemver == "" || latestSemver == "" || semver.Compare(latestSemver, currentSemver) <= 0 {
		return ApplyResult{}, ErrNoUpdate
	}

	supported, reason, debAsset, checksumAsset := detectInstallSupport(selected)
	if !supported {
		return ApplyResult{}, fmt.Errorf("%w: %s", ErrUnsupportedInstall, reason)
	}

	startedAt := s.now().UTC()
	progress := Progress{
		AttemptID:       fmt.Sprintf("update-%d", startedAt.UnixNano()),
		Status:          ProgressStatusRunning,
		Stage:           UpdateStagePreparing,
		PreviousVersion: currentVersion,
		TargetVersion:   targetVersion,
		StartedAt:       startedAt.Format(time.RFC3339),
	}
	if err := s.persistProgress(progress); err != nil {
		return ApplyResult{}, fmt.Errorf("initialize update progress: %w", err)
	}
	progressActive := true

	fail := func(updateErr error) (ApplyResult, error) {
		if progressActive {
			progress.Status = ProgressStatusFailed
			progress.FailedStage = progress.Stage
			progress.Error = updateErr.Error()
			progress.CompletedAt = s.now().UTC().Format(time.RFC3339)
			_ = s.persistProgress(progress)
		}
		return ApplyResult{}, updateErr
	}
	setStage := func(stage string) error {
		progress.Status = ProgressStatusRunning
		progress.Stage = stage
		progress.FailedStage = ""
		progress.Error = ""
		progress.CompletedAt = ""
		return s.persistProgress(progress)
	}

	if err := setStage(UpdateStageDownloading); err != nil {
		return fail(fmt.Errorf("persist download progress: %w", err))
	}
	checksumData, err := s.download(ctx, checksumAsset.BrowserDownloadURL, maxChecksumBody)
	if err != nil {
		return fail(fmt.Errorf("download checksums: %w", err))
	}
	debData, err := s.download(ctx, debAsset.BrowserDownloadURL, maxPackageBody)
	if err != nil {
		return fail(fmt.Errorf("download package: %w", err))
	}

	if err := setStage(UpdateStageVerifying); err != nil {
		return fail(fmt.Errorf("persist verification progress: %w", err))
	}
	expectedHash, err := checksumForFile(checksumData, debAsset.Name)
	if err != nil {
		return fail(err)
	}
	actualHash := sha256.Sum256(debData)
	actualHashHex := hex.EncodeToString(actualHash[:])
	if !strings.EqualFold(expectedHash, actualHashHex) {
		return fail(errors.New("downloaded package checksum does not match checksums.txt"))
	}
	if digest := strings.TrimSpace(debAsset.Digest); digest != "" {
		digest = strings.TrimPrefix(strings.ToLower(digest), "sha256:")
		if len(digest) == 64 && !strings.EqualFold(digest, actualHashHex) {
			return fail(errors.New("downloaded package checksum does not match GitHub asset digest"))
		}
	}

	updateDir, err := os.MkdirTemp("/var/tmp", "lannventory-update-")
	if err != nil {
		return fail(fmt.Errorf("create update directory: %w", err))
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(updateDir)
		}
	}()

	debPath := filepath.Join(updateDir, filepath.Base(debAsset.Name))
	if err := os.WriteFile(debPath, debData, 0o600); err != nil {
		return fail(fmt.Errorf("write package: %w", err))
	}

	healthURL = strings.TrimSpace(healthURL)
	if healthURL == "" {
		healthURL = "http://127.0.0.1:8840/api/health"
	}
	backupDir := filepath.Join(
		"/var/lib/lannventory/update-backups",
		fmt.Sprintf("%s-to-%s-%d", safePathComponent(currentVersion), safePathComponent(targetVersion), s.now().Unix()),
	)

	unitName := fmt.Sprintf("lannventory-update-%d", s.now().UnixNano())
	progress.BackupPath = backupDir
	progress.SystemdUnit = unitName
	if err := s.persistProgress(progress); err != nil {
		return fail(fmt.Errorf("persist update job metadata: %w", err))
	}

	scriptPath := filepath.Join(updateDir, "apply-update.sh")
	script := buildApplyUpdateScript(applyScriptParams{
		ActualHashHex:  actualHashHex,
		DebPath:        debPath,
		BackupDir:      backupDir,
		CurrentVersion: currentVersion,
		TargetVersion:  targetVersion,
		HealthURL:      healthURL,
		UpdateDir:      updateDir,
		ProgressPath:   s.progressPath,
		AttemptID:      progress.AttemptID,
		StartedAt:      progress.StartedAt,
		SystemdUnit:    unitName,
	})
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		return fail(fmt.Errorf("write update helper: %w", err))
	}

	if err := setStage(UpdateStageBackup); err != nil {
		return fail(fmt.Errorf("persist backup progress: %w", err))
	}

	output, err := exec.Command("systemd-run", "--unit="+unitName, "--collect", "--no-block", "/bin/sh", scriptPath).CombinedOutput()
	if err != nil {
		return fail(fmt.Errorf("schedule update: %w: %s", err, strings.TrimSpace(string(output))))
	}

	scheduled = true
	cleanup = false
	return ApplyResult{
		Version:    targetVersion,
		Scheduled:  true,
		Message:    "Update scheduled. LANnventory will back up its current data, install the verified package, restart, and run a health check.",
		BackupPath: backupDir,
	}, nil
}

func (s *Service) isUpdating() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updating
}

func (s *Service) loadReleases(ctx context.Context, refresh bool) ([]release, error) {
	s.mu.Lock()
	if !refresh && len(s.cached) > 0 && s.now().Before(s.cacheUntil) {
		cached := append([]release(nil), s.cached...)
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.releasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LANnventory-update-check")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub releases API returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseBody+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxReleaseBody {
		return nil, errors.New("GitHub releases response is too large")
	}

	var releases []release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	checkedAt := s.now()
	s.mu.Lock()
	s.cached = append([]release(nil), releases...)
	s.lastChecked = checkedAt
	s.cacheUntil = checkedAt.Add(cacheTTL)
	s.mu.Unlock()
	return releases, nil
}

func (s *Service) download(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	if !strings.HasPrefix(rawURL, releaseDownloadPrefix) {
		return nil, errors.New("refusing non-LANnventory release download URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LANnventory-updater")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("release download returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("release download is too large")
	}
	return data, nil
}

func selectLatestRelease(releases []release, channel string) (release, bool) {
	var selected release
	selectedVersion := ""
	for _, candidate := range releases {
		if candidate.Draft || (channel == StableChannel && candidate.Prerelease) {
			continue
		}
		version := comparableVersion(candidate.TagName)
		if version == "" {
			continue
		}
		if selectedVersion == "" || semver.Compare(version, selectedVersion) > 0 {
			selected = candidate
			selectedVersion = version
		}
	}
	return selected, selectedVersion != ""
}

func comparableVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	if idx := strings.Index(value, "-SNAPSHOT-"); idx >= 0 {
		value = value[:idx]
	}
	if value == "" {
		return ""
	}
	value = "v" + value
	if !semver.IsValid(value) {
		return ""
	}
	return value
}

func snapshotBaseVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	if idx := strings.Index(value, "-SNAPSHOT-"); idx >= 0 {
		base := value[:idx]
		if semver.IsValid("v" + base) {
			return base
		}
	}
	return ""
}

func displayVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}

func channelDisplayLabel(channel string) string {
	switch channel {
	case StableChannel:
		return "Stable"
	case BetaChannel:
		return "Beta"
	default:
		return "Beta"
	}
}

func detectInstallSupport(selected release) (bool, string, releaseAsset, releaseAsset) {
	if runtime.GOOS != "linux" {
		return false, "Automatic install is supported only on Linux package installations.", releaseAsset{}, releaseAsset{}
	}
	if os.Geteuid() != 0 {
		return false, "LANnventory must run as root to install package updates.", releaseAsset{}, releaseAsset{}
	}
	for _, command := range []string{"curl", "dpkg", "dpkg-query", "systemctl", "systemd-run", "sha256sum"} {
		if _, err := exec.LookPath(command); err != nil {
			return false, command + " is not available on this system.", releaseAsset{}, releaseAsset{}
		}
	}
	statusOutput, err := exec.Command("dpkg-query", "-W", "-f=$"+"{Status}", "lannventory").Output()
	if err != nil || !strings.Contains(string(statusOutput), "install ok installed") {
		return false, "LANnventory is not installed as a Debian package.", releaseAsset{}, releaseAsset{}
	}
	archOutput, err := exec.Command("dpkg", "--print-architecture").Output()
	if err != nil {
		return false, "Could not determine the Debian package architecture.", releaseAsset{}, releaseAsset{}
	}
	debArch, ok := releaseArchitecture(strings.TrimSpace(string(archOutput)))
	if !ok {
		return false, "Automatic updates are not yet supported for this Debian architecture.", releaseAsset{}, releaseAsset{}
	}

	var debAsset releaseAsset
	var checksumAsset releaseAsset
	suffix := "_linux_" + debArch + ".deb"
	for _, asset := range selected.Assets {
		switch {
		case asset.Name == "checksums.txt":
			checksumAsset = asset
		case strings.HasSuffix(asset.Name, suffix):
			debAsset = asset
		}
	}
	if debAsset.Name == "" {
		return false, "The selected release has no matching Debian package.", releaseAsset{}, releaseAsset{}
	}
	if checksumAsset.Name == "" {
		return false, "The selected release has no checksums.txt file.", releaseAsset{}, releaseAsset{}
	}
	return true, "", debAsset, checksumAsset
}

func releaseArchitecture(debianArch string) (string, bool) {
	switch debianArch {
	case "amd64":
		return "amd64", true
	case "arm64":
		return "arm64", true
	case "i386":
		return "386", true
	default:
		return "", false
	}
}

func checksumForFile(data []byte, filename string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == filename {
			hash := strings.ToLower(fields[0])
			if len(hash) != 64 {
				return "", errors.New("invalid SHA256 checksum entry")
			}
			if _, err := hex.DecodeString(hash); err != nil {
				return "", errors.New("invalid SHA256 checksum entry")
			}
			return hash, nil
		}
	}
	return "", fmt.Errorf("checksums.txt does not contain %s", filename)
}

func safePathComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '.', r == '-', r == '_':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	return out.String()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
