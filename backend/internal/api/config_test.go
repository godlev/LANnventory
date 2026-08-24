package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aceberg/WatchYourLAN/internal/conf"
	"github.com/aceberg/WatchYourLAN/internal/gdb"
	"github.com/aceberg/WatchYourLAN/internal/models"
	"github.com/gin-gonic/gin"
)

func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "positive", value: "120", want: 120},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "not number", value: "abc", wantErr: true},
		{name: "empty", value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePositiveInt(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePositiveInt(%q) returned nil error, want error", tt.value)
				}
				return
			}

			if err != nil {
				t.Fatalf("parsePositiveInt(%q) returned error: %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("parsePositiveInt(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func setupConfigRouter(t *testing.T) *gin.Engine {
	t.Helper()

	oldConfig := conf.AppConfig
	tempDir := t.TempDir()
	confPath := filepath.Join(tempDir, "config_v2.yaml")
	if err := os.WriteFile(confPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	conf.AppConfig = models.Conf{
		Host:                  "127.0.0.1",
		Port:                  "8840",
		Theme:                 "sand",
		Color:                 "dark",
		ConfPath:              confPath,
		DBPath:                filepath.Join(tempDir, "config-test.db"),
		Timeout:               600,
		TrimHist:              48,
		ConnectivityRetention: 72,
		UseDB:                 "sqlite",
		LogLevel:              "info",
		ArpArgs:               "-r 1",
		ArpStrs:               []string{"scan-a"},
		Ifaces:                "eth0",
	}

	t.Cleanup(func() {
		if err := gdb.Close(); err != nil {
			t.Errorf("gdb.Close: %v", err)
		}
		conf.AppConfig = oldConfig
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	Routes(router)

	return router
}

func stubScannerRestart(t *testing.T) *int {
	t.Helper()

	oldRestartScanner := restartScanner
	restartCalls := 0
	restartScanner = func() {
		restartCalls++
	}

	t.Cleanup(func() {
		restartScanner = oldRestartScanner
	})

	return &restartCalls
}

func TestSaveSettingsRejectsInvalidConnectivityRetention(t *testing.T) {
	router := setupConfigRouter(t)
	original := conf.AppConfig
	confPath := conf.AppConfig.ConfPath
	restartCalls := stubScannerRestart(t)

	body := strings.NewReader("log=debug&arpargs=-q&ifaces=wlan0&timeout=600&trim=48&connectivity_retention=-1&usedb=sqlite&pgconnect=&arpstrs=scan-b")
	req := httptest.NewRequest(http.MethodPost, "/api/config_settings/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !reflect.DeepEqual(conf.AppConfig, original) {
		t.Fatalf("AppConfig changed after rejected settings save:\ngot  %+v\nwant %+v", conf.AppConfig, original)
	}
	if *restartCalls != 0 {
		t.Fatalf("restart calls = %d, want 0 after rejected settings save", *restartCalls)
	}

	written, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if strings.Contains(strings.ToLower(string(written)), "connectivity_retention: -1") {
		t.Fatalf("config file persisted invalid connectivity retention: %s", string(written))
	}
}

func TestSaveSettingsRejectsInvalidInputAtomically(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantError string
	}{
		{
			name:      "invalid timeout",
			overrides: map[string]string{"timeout": "abc"},
			wantError: "invalid timeout",
		},
		{
			name:      "invalid trim",
			overrides: map[string]string{"trim": "0"},
			wantError: "invalid trim",
		},
		{
			name:      "invalid usedb",
			overrides: map[string]string{"usedb": "mysql"},
			wantError: "invalid usedb",
		},
		{
			name:      "invalid log",
			overrides: map[string]string{"log": "verbose"},
			wantError: "invalid log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupConfigRouter(t)
			original := conf.AppConfig
			restartCalls := stubScannerRestart(t)

			req := httptest.NewRequest(http.MethodPost, "/api/config_settings/", strings.NewReader(settingsForm(tt.overrides)))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantError) {
				t.Fatalf("body = %q, want to contain %q", rec.Body.String(), tt.wantError)
			}
			if !reflect.DeepEqual(conf.AppConfig, original) {
				t.Fatalf("AppConfig changed after rejected settings save:\ngot  %+v\nwant %+v", conf.AppConfig, original)
			}
			if *restartCalls != 0 {
				t.Fatalf("restart calls = %d, want 0 after rejected settings save", *restartCalls)
			}
		})
	}
}

func TestSaveSettingsPersistsAfterValidationAndRestartsScanner(t *testing.T) {
	router := setupConfigRouter(t)
	original := conf.AppConfig
	restartCalls := stubScannerRestart(t)

	req := httptest.NewRequest(http.MethodPost, "/api/config_settings/", strings.NewReader(settingsForm(nil)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if *restartCalls != 1 {
		t.Fatalf("restart calls = %d, want 1", *restartCalls)
	}
	if conf.AppConfig.LogLevel != "debug" ||
		conf.AppConfig.ArpArgs != "-q" ||
		conf.AppConfig.Ifaces != "wlan0" ||
		conf.AppConfig.Timeout != 300 ||
		conf.AppConfig.TrimHist != 96 ||
		conf.AppConfig.ConnectivityRetention != 168 ||
		!reflect.DeepEqual(conf.AppConfig.ArpStrs, []string{"scan-b"}) {
		t.Fatalf("scan settings were not updated correctly: %+v", conf.AppConfig)
	}
	if conf.AppConfig.Host != original.Host ||
		conf.AppConfig.Port != original.Port ||
		conf.AppConfig.Theme != original.Theme ||
		conf.AppConfig.Color != original.Color ||
		conf.AppConfig.ShoutURL != original.ShoutURL {
		t.Fatalf("unrelated config changed: got %+v, original %+v", conf.AppConfig, original)
	}
}

func TestSaveSettingsMigratesDatabaseWhenDBConfigChanges(t *testing.T) {
	router := setupConfigRouter(t)
	dbPath := conf.AppConfig.DBPath
	restartCalls := stubScannerRestart(t)

	req := httptest.NewRequest(http.MethodPost, "/api/config_settings/", strings.NewReader(settingsForm(map[string]string{
		"pgconnect": "changed-but-still-sqlite",
	})))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if *restartCalls != 1 {
		t.Fatalf("restart calls = %d, want 1", *restartCalls)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("settings DB was not created at %s: %v", dbPath, err)
	}

	host := models.Host{
		Name:       "migration-check",
		Mac:        "AA:BB:CC:DD:EE:90",
		DeviceType: "router",
	}
	if err := gdb.UpdateWithError("now", host); err != nil {
		t.Fatalf("UpdateWithError now after settings DB migration: %v", err)
	}
	hosts := gdb.SelectByMAC("now", host.Mac)
	if len(hosts) != 1 || hosts[0].DeviceType != "router" {
		t.Fatalf("migrated now table did not persist DeviceType: %+v", hosts)
	}
	if err := gdb.UpdateWithError("history", hosts[0]); err != nil {
		t.Fatalf("UpdateWithError history after settings DB migration: %v", err)
	}
	if err := gdb.AddEvent(models.NewHostEvent(hosts[0], models.EventDiscovered, "", "")); err != nil {
		t.Fatalf("AddEvent after settings DB migration: %v", err)
	}
}

func settingsForm(overrides map[string]string) string {
	values := url.Values{
		"log":                    {"debug"},
		"arpargs":                {"-q"},
		"ifaces":                 {"wlan0"},
		"timeout":                {"300"},
		"trim":                   {"96"},
		"connectivity_retention": {"168"},
		"usedb":                  {"sqlite"},
		"pgconnect":              {""},
		"arpstrs":                {"scan-b", ""},
	}

	for key, value := range overrides {
		values.Set(key, value)
	}

	return values.Encode()
}

func TestSaveRetentionHandlerPersistsOnlyRetentionFields(t *testing.T) {
	router := setupConfigRouter(t)
	original := conf.AppConfig

	body := bytes.NewBufferString(`{"presenceRetention":96,"connectivityRetention":168}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/retention", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got models.Conf
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.TrimHist != 96 {
		t.Fatalf("response TrimHist = %d, want 96", got.TrimHist)
	}
	if got.ConnectivityRetention != 168 {
		t.Fatalf("response ConnectivityRetention = %d, want 168", got.ConnectivityRetention)
	}
	if conf.AppConfig.TrimHist != 96 {
		t.Fatalf("conf.AppConfig.TrimHist = %d, want 96", conf.AppConfig.TrimHist)
	}
	if conf.AppConfig.ConnectivityRetention != 168 {
		t.Fatalf("conf.AppConfig.ConnectivityRetention = %d, want 168", conf.AppConfig.ConnectivityRetention)
	}
	if conf.AppConfig.Timeout != original.Timeout ||
		conf.AppConfig.Ifaces != original.Ifaces ||
		conf.AppConfig.UseDB != original.UseDB ||
		conf.AppConfig.PGConnect != original.PGConnect ||
		conf.AppConfig.ArpArgs != original.ArpArgs {
		t.Fatalf("unrelated config changed: got %+v, original %+v", conf.AppConfig, original)
	}

	written, err := os.ReadFile(conf.AppConfig.ConfPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	lowerWritten := strings.ToLower(string(written))
	if !strings.Contains(lowerWritten, "trim_hist: 96") {
		t.Fatalf("config file did not persist trim hist: %s", string(written))
	}
	if !strings.Contains(lowerWritten, "connectivity_retention: 168") {
		t.Fatalf("config file did not persist connectivity retention: %s", string(written))
	}
}

func TestSaveRetentionHandlerRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "zero presence",
			body: `{"presenceRetention":0,"connectivityRetention":168}`,
			want: "invalid presenceRetention",
		},
		{
			name: "negative connectivity",
			body: `{"presenceRetention":96,"connectivityRetention":-1}`,
			want: "invalid connectivityRetention",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupConfigRouter(t)
			confPath := conf.AppConfig.ConfPath

			req := httptest.NewRequest(http.MethodPost, "/api/config/retention", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.want) {
				t.Fatalf("body = %q, want to contain %q", rec.Body.String(), tt.want)
			}
			if conf.AppConfig.TrimHist != 48 {
				t.Fatalf("TrimHist = %d, want unchanged 48", conf.AppConfig.TrimHist)
			}
			if conf.AppConfig.ConnectivityRetention != 72 {
				t.Fatalf("ConnectivityRetention = %d, want unchanged 72", conf.AppConfig.ConnectivityRetention)
			}

			written, err := os.ReadFile(confPath)
			if err != nil {
				t.Fatalf("os.ReadFile: %v", err)
			}
			lowerWritten := strings.ToLower(string(written))
			if strings.Contains(lowerWritten, "trim_hist: 0") || strings.Contains(lowerWritten, "connectivity_retention: -1") {
				t.Fatalf("config file persisted invalid retention values: %s", string(written))
			}
		})
	}
}

func TestSaveColorHandlerPersistsValidColor(t *testing.T) {
	router := setupConfigRouter(t)

	body := bytes.NewBufferString(`{"color":"light"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/color", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got models.Conf
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.Color != "light" {
		t.Fatalf("response Color = %q, want %q", got.Color, "light")
	}
	if conf.AppConfig.Color != "light" {
		t.Fatalf("conf.AppConfig.Color = %q, want %q", conf.AppConfig.Color, "light")
	}

	written, err := os.ReadFile(conf.AppConfig.ConfPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(written)), "color: light") {
		t.Fatalf("config file did not persist color: %s", string(written))
	}
}

func TestSaveColorHandlerRejectsInvalidColor(t *testing.T) {
	router := setupConfigRouter(t)
	confPath := conf.AppConfig.ConfPath

	body := bytes.NewBufferString(`{"color":"blue"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/color", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if conf.AppConfig.Color != "dark" {
		t.Fatalf("conf.AppConfig.Color = %q, want %q", conf.AppConfig.Color, "dark")
	}

	written, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if strings.Contains(strings.ToLower(string(written)), "color: blue") {
		t.Fatalf("config file persisted invalid color: %s", string(written))
	}
}
