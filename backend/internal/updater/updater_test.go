package updater

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestComparableVersionNormalizesSnapshots(t *testing.T) {
	tests := map[string]string{
		"0.1.0-beta.2":                  "v0.1.0-beta.2",
		"v0.1.0-beta.2":                 "v0.1.0-beta.2",
		"0.1.0-beta.2-SNAPSHOT-deadbee": "v0.1.0-beta.2",
		"not-a-version":                 "",
	}
	for input, want := range tests {
		if got := comparableVersion(input); got != want {
			t.Fatalf("comparableVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSelectLatestReleaseHonorsChannel(t *testing.T) {
	releases := []release{
		{TagName: "v0.2.0-beta.1", Prerelease: true},
		{TagName: "v0.1.5", Prerelease: false},
		{TagName: "v0.1.0-beta.2", Prerelease: true},
	}
	stable, ok := selectLatestRelease(releases, StableChannel)
	if !ok || stable.TagName != "v0.1.5" {
		t.Fatalf("stable = %+v, ok=%v", stable, ok)
	}
	beta, ok := selectLatestRelease(releases, BetaChannel)
	if !ok || beta.TagName != "v0.2.0-beta.1" {
		t.Fatalf("beta = %+v, ok=%v", beta, ok)
	}
}

func TestCheckDoesNotOfferSameSnapshotBaseRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v0.1.0-beta.2","prerelease":true,"draft":false,"published_at":"2026-09-01T00:00:00Z","assets":[]}]`))
	}))
	defer server.Close()

	service := NewServiceWithURL(server.Client(), server.URL)
	status, err := service.Check(context.Background(), "0.1.0-beta.2-SNAPSHOT-deadbee", BetaChannel, true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status.Available {
		t.Fatalf("snapshot base should compare equal to released beta: %+v", status)
	}
	if status.LatestVersion != "0.1.0-beta.2" {
		t.Fatalf("LatestVersion = %q", status.LatestVersion)
	}
}

func TestCheckFindsNewerBetaRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"tag_name":"v0.1.0-beta.3","prerelease":true,"draft":false,"published_at":"2026-09-15T00:00:00Z","assets":[]},
			{"tag_name":"v0.1.0-beta.2","prerelease":true,"draft":false,"assets":[]}
		]`))
	}))
	defer server.Close()

	service := NewServiceWithURL(server.Client(), server.URL)
	status, err := service.Check(context.Background(), "0.1.0-beta.2-SNAPSHOT-deadbee", BetaChannel, true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !status.Available || status.LatestVersion != "0.1.0-beta.3" {
		t.Fatalf("status = %+v", status)
	}
}

func TestChecksumForFile(t *testing.T) {
	hash := strings.Repeat("a", 64)
	data := []byte(hash + "  lannventory_0.2.0_linux_amd64.deb\n")
	got, err := checksumForFile(data, "lannventory_0.2.0_linux_amd64.deb")
	if err != nil {
		t.Fatalf("checksumForFile: %v", err)
	}
	if got != hash {
		t.Fatalf("hash = %q, want %q", got, hash)
	}
}

func TestSafePathComponent(t *testing.T) {
	tests := map[string]string{
		"0.1.0-beta.2-SNAPSHOT-0251fa4": "0.1.0-beta.2-SNAPSHOT-0251fa4",
		" v0.1.0 beta/3 ":               "v0.1.0_beta_3",
		"":                              "unknown",
	}
	for input, want := range tests {
		if got := safePathComponent(input); got != want {
			t.Fatalf("safePathComponent(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAutoSchedulerDisabledDoesNotCheckOrInstall(t *testing.T) {
	scheduler := &AutoScheduler{
		Config: func() AutoConfig {
			return AutoConfig{CurrentVersion: "0.1.0", Channel: BetaChannel, AutomaticCheck: false, AutomaticInstall: false, IntervalHours: 24}
		},
		Check: func(context.Context, string, string, bool) (Status, error) {
			t.Fatal("Check called while automatic updates disabled")
			return Status{}, nil
		},
		Schedule: func(context.Context, string, string, string, string) (ApplyResult, error) {
			t.Fatal("Schedule called while automatic updates disabled")
			return ApplyResult{}, nil
		},
	}

	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
}

func TestAutoSchedulerCheckOnlyNeverInstalls(t *testing.T) {
	checks := 0
	scheduler := &AutoScheduler{
		Config: func() AutoConfig {
			return AutoConfig{
				CurrentVersion:   "0.1.0",
				Channel:          BetaChannel,
				AutomaticCheck:   true,
				AutomaticInstall: false,
				IntervalHours:    24,
			}
		},
		Check: func(context.Context, string, string, bool) (Status, error) {
			checks++
			return Status{Available: true, InstallSupported: true, LatestVersion: "0.1.1"}, nil
		},
		Schedule: func(context.Context, string, string, string, string) (ApplyResult, error) {
			t.Fatal("Schedule called while automatic installation is disabled")
			return ApplyResult{}, nil
		},
	}

	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if checks != 1 {
		t.Fatalf("checks = %d, want 1", checks)
	}
}

func TestAutoSchedulerNoNewerVersionDoesNotInstall(t *testing.T) {
	checks := 0
	scheduler := &AutoScheduler{
		Config: func() AutoConfig {
			return AutoConfig{CurrentVersion: "0.1.0", Channel: BetaChannel, AutomaticCheck: true, AutomaticInstall: true, IntervalHours: 24}
		},
		Check: func(context.Context, string, string, bool) (Status, error) {
			checks++
			return Status{Available: false}, nil
		},
		Schedule: func(context.Context, string, string, string, string) (ApplyResult, error) {
			t.Fatal("Schedule called despite no newer version")
			return ApplyResult{}, nil
		},
	}

	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if checks != 1 {
		t.Fatalf("checks = %d, want 1", checks)
	}
}

func TestAutoSchedulerSchedulesSupportedUpdate(t *testing.T) {
	var scheduled struct {
		current string
		channel string
		version string
	}
	scheduler := &AutoScheduler{
		Config: func() AutoConfig {
			return AutoConfig{CurrentVersion: "0.1.0", Channel: BetaChannel, AutomaticCheck: true, AutomaticInstall: true, IntervalHours: 24}
		},
		Check: func(context.Context, string, string, bool) (Status, error) {
			return Status{Available: true, InstallSupported: true, LatestVersion: "0.1.1"}, nil
		},
		Schedule: func(_ context.Context, current, channel, version, healthURL string) (ApplyResult, error) {
			scheduled.current = current
			scheduled.channel = channel
			scheduled.version = version
			return ApplyResult{Version: version, Scheduled: true}, nil
		},
	}

	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if scheduled.current != "0.1.0" || scheduled.channel != BetaChannel || scheduled.version != "0.1.1" {
		t.Fatalf("scheduled = %+v", scheduled)
	}
}

func TestAutoSchedulerFailureDoesNotStopFutureRuns(t *testing.T) {
	checks := 0
	wantErr := errors.New("temporary")
	scheduler := &AutoScheduler{
		Config: func() AutoConfig {
			return AutoConfig{CurrentVersion: "0.1.0", Channel: BetaChannel, AutomaticCheck: true, AutomaticInstall: true, IntervalHours: 24}
		},
		Check: func(context.Context, string, string, bool) (Status, error) {
			checks++
			if checks == 1 {
				return Status{}, wantErr
			}
			return Status{Available: false}, nil
		},
		Schedule: func(context.Context, string, string, string, string) (ApplyResult, error) {
			t.Fatal("Schedule called")
			return ApplyResult{}, nil
		},
	}

	if err := scheduler.RunOnce(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("first RunOnce error = %v, want %v", err, wantErr)
	}
	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("second RunOnce: %v", err)
	}
	if checks != 2 {
		t.Fatalf("checks = %d, want 2", checks)
	}
}

func TestAutoSchedulerSkipsUnsupportedInstall(t *testing.T) {
	scheduler := &AutoScheduler{
		Config: func() AutoConfig {
			return AutoConfig{CurrentVersion: "0.1.0", Channel: StableChannel, AutomaticCheck: true, AutomaticInstall: true, IntervalHours: 24}
		},
		Check: func(context.Context, string, string, bool) (Status, error) {
			return Status{Available: true, InstallSupported: false, InstallReason: "not a package install"}, nil
		},
		Schedule: func(context.Context, string, string, string, string) (ApplyResult, error) {
			t.Fatal("Schedule called for unsupported install")
			return ApplyResult{}, nil
		},
	}

	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
}

func TestAutoSchedulerNormalizesInvalidInterval(t *testing.T) {
	scheduler := &AutoScheduler{
		Config: func() AutoConfig {
			return AutoConfig{IntervalHours: -1}
		},
	}

	if got := scheduler.NextInterval(); got != 24*time.Hour {
		t.Fatalf("NextInterval = %v, want 24h", got)
	}
}

func TestAutoSchedulerNotifyConfigChangedCoalesces(t *testing.T) {
	scheduler := &AutoScheduler{configChanged: make(chan struct{}, 1)}

	scheduler.NotifyConfigChanged()
	scheduler.NotifyConfigChanged()

	select {
	case <-scheduler.configChanged:
	default:
		t.Fatal("expected config-change notification")
	}

	select {
	case <-scheduler.configChanged:
		t.Fatal("duplicate config-change notification was queued")
	default:
	}
}

func TestExtractReleaseSummaryPrefersSummarySection(t *testing.T) {
	markdown := "# Release\n\n## Summary\n- First item\n- Second item\n\n## Details\n- Not included\n"
	got := extractReleaseSummary(markdown)
	want := "- First item\n- Second item"
	if got != want {
		t.Fatalf("extractReleaseSummary() = %q, want %q", got, want)
	}
}

func TestExtractReleaseSummaryFallsBackToHighlights(t *testing.T) {
	markdown := "# Release\n\n## Highlights\n### Updates\n- Automatic checks\n- Local notes\n\n## Validation\n- Not included\n"
	got := extractReleaseSummary(markdown)
	want := "Updates:\n- Automatic checks\n- Local notes"
	if got != want {
		t.Fatalf("extractReleaseSummary() = %q, want %q", got, want)
	}
}

func TestCheckCachedDoesNotContactReleaseServer(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`[{"tag_name":"v0.1.1","prerelease":true,"draft":false,"assets":[]}]`))
	}))
	defer server.Close()

	service := NewServiceWithURL(server.Client(), server.URL)
	status := service.CheckCached("0.1.0", BetaChannel)
	if requests != 0 {
		t.Fatalf("cached status performed %d HTTP requests, want 0", requests)
	}
	if status.Available {
		t.Fatalf("empty cache unexpectedly reported update: %+v", status)
	}

	if _, err := service.Check(context.Background(), "0.1.0", BetaChannel, true); err != nil {
		t.Fatalf("Check: %v", err)
	}
	requests = 0
	status = service.CheckCached("0.1.0", BetaChannel)
	if requests != 0 {
		t.Fatalf("cached status performed %d HTTP requests after cache fill, want 0", requests)
	}
	if !status.Available || status.LatestVersion != "0.1.1" {
		t.Fatalf("cached status = %+v", status)
	}
}

func TestPhase34UATReleaseSelectionFromBeta5(t *testing.T) {
	releases := []release{
		{
			TagName:     "v0.1.0-beta.5.uat.1",
			Prerelease:  true,
			Draft:       false,
			PublishedAt: "2026-09-18T00:00:00Z",
			HTMLURL:     "https://github.com/godlev/LANnventory/releases/tag/v0.1.0-beta.5.uat.1",
		},
		{
			TagName:    "v0.1.0-beta.5",
			Prerelease: true,
			Draft:      false,
		},
	}

	betaStatus, selected, ok := buildStatus(
		"v0.1.0-beta.5",
		BetaChannel,
		releases,
		time.Time{},
		false,
	)
	if !ok {
		t.Fatal("Beta channel did not select a release")
	}
	if selected.TagName != "v0.1.0-beta.5.uat.1" {
		t.Fatalf("Beta selected %q, want Phase 34 UAT", selected.TagName)
	}
	if !betaStatus.Available || betaStatus.LatestVersion != "0.1.0-beta.5.uat.1" {
		t.Fatalf("Beta status = %+v, want available Phase 34 UAT", betaStatus)
	}

	stableStatus, selected, ok := buildStatus(
		"v0.1.0-beta.5",
		StableChannel,
		releases,
		time.Time{},
		false,
	)
	if ok || selected.TagName != "" {
		t.Fatalf("Stable channel selected prerelease: selected=%+v ok=%v", selected, ok)
	}
	if stableStatus.Available || stableStatus.LatestVersion != "" {
		t.Fatalf("Stable status exposed UAT prerelease: %+v", stableStatus)
	}

	beta6 := []release{
		{TagName: "v0.1.0-beta.6", Prerelease: true},
		{TagName: "v0.1.0-beta.5.uat.1", Prerelease: true},
	}
	future, ok := selectLatestRelease(beta6, BetaChannel)
	if !ok || future.TagName != "v0.1.0-beta.6" {
		t.Fatalf("future beta.6 should supersede UAT, got %+v ok=%v", future, ok)
	}
}

func TestPhase34HostUXUAT2SelectionFromUAT1(t *testing.T) {
	releases := []release{
		{
			TagName:     "v0.1.0-beta.5.uat.2",
			Prerelease:  true,
			Draft:       false,
			PublishedAt: "2026-09-18T00:00:00Z",
			HTMLURL:     "https://github.com/godlev/LANnventory/releases/tag/v0.1.0-beta.5.uat.2",
		},
		{
			TagName:    "v0.1.0-beta.5.uat.1",
			Prerelease: true,
			Draft:      false,
		},
		{
			TagName:    "v0.1.0-beta.5",
			Prerelease: true,
			Draft:      false,
		},
	}

	betaStatus, selected, ok := buildStatus(
		"v0.1.0-beta.5.uat.1",
		BetaChannel,
		releases,
		time.Time{},
		false,
	)
	if !ok {
		t.Fatal("Beta channel did not select a release")
	}
	if selected.TagName != "v0.1.0-beta.5.uat.2" {
		t.Fatalf("Beta selected %q, want Host UX UAT2", selected.TagName)
	}
	if !betaStatus.Available || betaStatus.LatestVersion != "0.1.0-beta.5.uat.2" {
		t.Fatalf("Beta status = %+v, want available Host UX UAT2", betaStatus)
	}

	stableStatus, selected, ok := buildStatus(
		"v0.1.0-beta.5.uat.1",
		StableChannel,
		releases,
		time.Time{},
		false,
	)
	if ok || selected.TagName != "" {
		t.Fatalf("Stable channel selected prerelease: selected=%+v ok=%v", selected, ok)
	}
	if stableStatus.Available || stableStatus.LatestVersion != "" {
		t.Fatalf("Stable status exposed Host UX UAT2 prerelease: %+v", stableStatus)
	}

	future := []release{
		{TagName: "v0.1.0-beta.6", Prerelease: true},
		{TagName: "v0.1.0-beta.5.uat.2", Prerelease: true},
		{TagName: "v0.1.0-beta.5.uat.1", Prerelease: true},
	}
	next, ok := selectLatestRelease(future, BetaChannel)
	if !ok || next.TagName != "v0.1.0-beta.6" {
		t.Fatalf("future beta.6 should supersede UAT2, got %+v ok=%v", next, ok)
	}
}
