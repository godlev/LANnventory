package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
)

func TestDiagnosticsEndpointIsReadOnlyAndReturnsChecks(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		Version: "0.1.0-beta.3.1",
		UseDB:   "sqlite",
		DBPath:  filepath.Join(t.TempDir(), "diagnostics-api.db"),
	})
	t.Cleanup(func() {
		_ = gdb.Close()
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := gdb.StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	Routes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		OK     bool `json:"ok"`
		Checks []struct {
			Check   string `json:"check"`
			Status  string `json:"status"`
			Details string `json:"details"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(response.Checks) < 8 {
		t.Fatalf("checks len = %d, want diagnostics set; body=%s", len(response.Checks), rec.Body.String())
	}

	foundRuntime := false
	foundDatabase := false
	for _, item := range response.Checks {
		if item.Check == "LANnventory" {
			foundRuntime = true
		}
		if item.Check == "Database" {
			foundDatabase = true
			if item.Status != "ok" {
				t.Fatalf("Database status = %q, want ok; details=%q", item.Status, item.Details)
			}
		}
	}
	if !foundRuntime || !foundDatabase {
		t.Fatalf("required diagnostics missing: %+v", response.Checks)
	}
}
