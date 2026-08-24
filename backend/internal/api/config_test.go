package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aceberg/WatchYourLAN/internal/conf"
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
	confPath := filepath.Join(t.TempDir(), "config_v2.yaml")
	if err := os.WriteFile(confPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	conf.AppConfig = models.Conf{
		Host:     "127.0.0.1",
		Port:     "8840",
		Theme:    "sand",
		Color:    "dark",
		ConfPath: confPath,
		Timeout:  600,
		TrimHist: 48,
		UseDB:    "sqlite",
		LogLevel: "info",
	}

	t.Cleanup(func() {
		conf.AppConfig = oldConfig
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	Routes(router)

	return router
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
