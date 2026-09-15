package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/updater"
)

func TestSaveUpdateSettingsPersistsAndReturnsStatus(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0-beta.2-SNAPSHOT-deadbee")
	stubUpdateService(t, `[{"tag_name":"v0.1.0-beta.2","prerelease":true,"draft":false,"published_at":"2026-09-01T00:00:00Z","html_url":"https://github.com/godlev/LANnventory/releases/tag/v0.1.0-beta.2","assets":[]}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/update/settings", strings.NewReader(`{"channel":"stable","automatic":true,"intervalHours":12}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	config := conf.GetAppConfig()
	if config.UpdateChannel != "stable" || !config.UpdateAuto || config.UpdateCheckIntervalHours != 12 {
		t.Fatalf("config update settings = channel %q auto %v interval %d", config.UpdateChannel, config.UpdateAuto, config.UpdateCheckIntervalHours)
	}
	var status updater.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if status.Channel != "stable" || !status.Automatic || status.IntervalHours != 12 {
		t.Fatalf("response update settings = channel %q auto %v interval %d", status.Channel, status.Automatic, status.IntervalHours)
	}
}

func TestSaveUpdateSettingsRejectsInvalidInterval(t *testing.T) {
	router := setupConfigRouter(t)
	original := conf.GetAppConfig()

	req := httptest.NewRequest(http.MethodPost, "/api/update/settings", strings.NewReader(`{"channel":"beta","automatic":true,"intervalHours":5}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	config := conf.GetAppConfig()
	if config.UpdateAuto != original.UpdateAuto || config.UpdateCheckIntervalHours != original.UpdateCheckIntervalHours || config.UpdateChannel != original.UpdateChannel {
		t.Fatalf("invalid request changed config: got %+v want %+v", config, original)
	}
}

func TestSaveUpdateChannelPreservesAutomaticSettings(t *testing.T) {
	router := setupConfigRouter(t)
	conf.SetVersion("0.1.0")
	config := conf.GetAppConfig()
	config.UpdateChannel = "beta"
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
	if config.UpdateChannel != "stable" || !config.UpdateAuto || config.UpdateCheckIntervalHours != 6 {
		t.Fatalf("channel update settings = channel %q auto %v interval %d", config.UpdateChannel, config.UpdateAuto, config.UpdateCheckIntervalHours)
	}
	var status updater.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if status.Channel != "stable" || !status.Automatic || status.IntervalHours != 6 {
		t.Fatalf("response update settings = channel %q auto %v interval %d", status.Channel, status.Automatic, status.IntervalHours)
	}
}

func stubUpdateService(t *testing.T, body string) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	oldService := updateService
	updateService = updater.NewServiceWithURL(server.Client(), server.URL)
	t.Cleanup(func() {
		updateService = oldService
		server.Close()
	})
}
