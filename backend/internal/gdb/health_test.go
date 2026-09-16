package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestGetDatabaseHealthReportsActiveSQLiteConnection(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "health-test.db"),
	})
	t.Cleanup(func() {
		_ = Close()
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}

	health := GetDatabaseHealth()
	if !health.Connected {
		t.Fatalf("Connected = false, error=%q", health.Error)
	}
	if health.Backend != "sqlite" {
		t.Fatalf("Backend = %q, want sqlite", health.Backend)
	}
	if health.Error != "" {
		t.Fatalf("Error = %q, want empty", health.Error)
	}
}

func TestGetDatabaseHealthWithoutDatabaseIsDisconnected(t *testing.T) {
	if err := Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	health := GetDatabaseHealth()
	if health.Connected {
		t.Fatal("Connected = true with no active database")
	}
	if health.Error == "" {
		t.Fatal("Error is empty with no active database")
	}
}
