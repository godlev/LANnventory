package gdb

import (
	"path/filepath"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestRecordHostServiceObservationRollsBackWhenEventInsertFails(t *testing.T) {
	oldConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		UseDB:  "sqlite",
		DBPath: filepath.Join(t.TempDir(), "service-event-rollback.db"),
	})
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		conf.SetAppConfigForTest(oldConfig)
	})

	if err := StartErr(); err != nil {
		t.Fatalf("StartErr: %v", err)
	}
	if err := db.Migrator().DropTable("events"); err != nil {
		t.Fatalf("DropTable(events): %v", err)
	}

	host := models.Host{
		ID:         7,
		Name:       "server",
		Mac:        "AA:BB:CC:DD:EE:57",
		IP:         "192.168.1.57",
		Iface:      "eth0",
		DeviceType: "server",
	}
	_, _, _, err := RecordHostServiceObservation(host, models.Service{
		Mac:            host.Mac,
		Address:        host.IP,
		Protocol:       "tcp",
		Port:           443,
		State:          "open",
		LastScanSource: "manual",
	}, "2026-09-18 11:00:00")
	if err == nil {
		t.Fatal("RecordHostServiceObservation returned nil error with events table removed")
	}

	_, ok, selectErr := SelectServiceByIdentity(host.Mac, host.IP, "tcp", 443)
	if selectErr != nil {
		t.Fatalf("SelectServiceByIdentity: %v", selectErr)
	}
	if ok {
		t.Fatal("service summary persisted even though lifecycle event transaction failed")
	}
}
