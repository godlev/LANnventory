package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/updater"
)

func TestSaveUpdateSettingsPersistsAndReturnsStatus(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0-beta.2-SNAPSHOT-deadbee")
	stubUpdateService(t, `[{"tag_name":"v0.1.0-beta.2","prerelease":true,"draft":false,"published_at":"2026-09-01T00:00:00Z","html_url":"https://github.com/godlev/LANnventory/releases/tag/v0.1.0-beta.2","assets":[]}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/update/settings", strings.NewReader(`{"channel":"stable","automaticCheck":true,"automatic":true,"intervalHours":12}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	config := conf.GetAppConfig()
	if config.UpdateChannel != "stable" || !config.UpdateCheckAuto || !config.UpdateAuto || config.UpdateCheckIntervalHours != 12 {
		t.Fatalf("config update settings = channel %q check %v auto %v interval %d", config.UpdateChannel, config.UpdateCheckAuto, config.UpdateAuto, config.UpdateCheckIntervalHours)
	}
	var status updater.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if status.Channel != "stable" || !status.AutomaticCheck || !status.Automatic || status.IntervalHours != 12 {
		t.Fatalf("response update settings = channel %q check %v auto %v interval %d", status.Channel, status.AutomaticCheck, status.Automatic, status.IntervalHours)
	}
}

func TestSaveUpdateSettingsRejectsInvalidInterval(t *testing.T) {
	router := setupConfigRouter(t)
	original := conf.GetAppConfig()

	req := httptest.NewRequest(http.MethodPost, "/api/update/settings", strings.NewReader(`{"channel":"beta","automaticCheck":true,"automatic":true,"intervalHours":5}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	config := conf.GetAppConfig()
	if config.UpdateCheckAuto != original.UpdateCheckAuto || config.UpdateAuto != original.UpdateAuto || config.UpdateCheckIntervalHours != original.UpdateCheckIntervalHours || config.UpdateChannel != original.UpdateChannel {
		t.Fatalf("invalid request changed config: got %+v want %+v", config, original)
	}
}

func TestSaveUpdateChannelPreservesAutomaticSettings(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0")
	config := conf.GetAppConfig()
	config.UpdateChannel = "beta"
	config.UpdateCheckAuto = true
	config.UpdateAuto = true
	config.UpdateCheckIntervalHours = 6
	conf.SetAppConfigForTest(config)
	stubUpdateService(t, `[{"tag_name":"v0.1.0","prerelease":false,"draft":false,"assets":[]}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/update/channel", strings.NewReader(`{"channel":"stable"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	config = conf.GetAppConfig()
	if config.UpdateChannel != "stable" || !config.UpdateCheckAuto || !config.UpdateAuto || config.UpdateCheckIntervalHours != 6 {
		t.Fatalf("channel update settings = channel %q check %v auto %v interval %d", config.UpdateChannel, config.UpdateCheckAuto, config.UpdateAuto, config.UpdateCheckIntervalHours)
	}
	var status updater.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if status.Channel != "stable" || !status.AutomaticCheck || !status.Automatic || status.IntervalHours != 6 {
		t.Fatalf("response update settings = channel %q check %v auto %v interval %d", status.Channel, status.AutomaticCheck, status.Automatic, status.IntervalHours)
	}
}

func TestGetUpdateStatusOnlyRefreshContactsReleaseSource(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0-beta.2")
	calls := stubCountingUpdateService(t, `[{"tag_name":"v0.1.0-beta.3","prerelease":true,"draft":false,"assets":[]}]`)

	req := httptest.NewRequest(http.MethodGet, "/api/update/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cached status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("passive status contacted release source %d times, want 0", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/update/status?refresh=1", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refreshed status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("explicit refresh contacted release source %d times, want 1", got)
	}
}

func TestSaveUpdateSettingsDoesNotContactReleaseSource(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0-beta.2")
	calls := stubCountingUpdateService(t, `[{"tag_name":"v0.1.0-beta.3","prerelease":true,"draft":false,"assets":[]}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/update/settings", strings.NewReader(`{"channel":"beta","automaticCheck":true,"automatic":false,"intervalHours":24}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("saving update settings contacted release source %d times, want 0", got)
	}
}

func TestSaveUpdateChannelDoesNotContactReleaseSource(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0-beta.2")
	calls := stubCountingUpdateService(t, `[{"tag_name":"v0.1.0","prerelease":false,"draft":false,"assets":[]}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/update/channel", strings.NewReader(`{"channel":"stable"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("saving update channel contacted release source %d times, want 0", got)
	}
}

func stubUpdateService(t *testing.T, body string) {
	t.Helper()
	_ = stubCountingUpdateService(t, body)
}

func stubCountingUpdateService(t *testing.T, body string) *atomic.Int32 {
	t.Helper()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	oldService := updateService
	updateService = updater.NewServiceWithURL(server.Client(), server.URL)
	t.Cleanup(func() {
		updateService = oldService
		server.Close()
	})
	return &calls
}

func TestUpdateHealthURL(t *testing.T) {
	tests := map[string]struct {
		host string
		port string
		want string
	}{
		"configured IPv4": {host: "10.4.1.29", port: "8840", want: "http://10.4.1.29:8840/api/health"},
		"wildcard IPv4":   {host: "0.0.0.0", port: "8840", want: "http://127.0.0.1:8840/api/health"},
		"wildcard IPv6":   {host: "::", port: "8840", want: "http://[::1]:8840/api/health"},
		"default port":    {host: "127.0.0.1", port: "", want: "http://127.0.0.1:8840/api/health"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := updateHealthURL(tc.host, tc.port); got != tc.want {
				t.Fatalf("updateHealthURL(%q, %q) = %q, want %q", tc.host, tc.port, got, tc.want)
			}
		})
	}
}

func TestSaveUpdateSettingsSupportsCheckOnlyMode(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0-beta.3")
	stubUpdateService(t, `[{"tag_name":"v0.1.0-beta.3","prerelease":true,"draft":false,"assets":[]}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/update/settings", strings.NewReader(`{"channel":"beta","automaticCheck":true,"automatic":false,"intervalHours":24}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	config := conf.GetAppConfig()
	if !config.UpdateCheckAuto || config.UpdateAuto {
		t.Fatalf("check-only settings not persisted: %+v", config)
	}

	var status updater.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !status.AutomaticCheck || status.Automatic {
		t.Fatalf("check-only response = %+v", status)
	}
}

func TestSaveUpdateSettingsAutomaticInstallImpliesChecks(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0-beta.3")
	stubUpdateService(t, `[{"tag_name":"v0.1.0-beta.3","prerelease":true,"draft":false,"assets":[]}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/update/settings", strings.NewReader(`{"channel":"beta","automaticCheck":false,"automatic":true,"intervalHours":24}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	config := conf.GetAppConfig()
	if !config.UpdateCheckAuto || !config.UpdateAuto {
		t.Fatalf("automatic install should imply automatic checks: %+v", config)
	}
}
