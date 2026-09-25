package proxmoxsync

import (
	"context"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestManualAndAutomaticSyncCoordinateOnSameHostLease(t *testing.T) {
	service := NewService()
	mac := "AA:BB:CC:DD:EE:F4"

	releaseAutomatic, ok := service.tryBegin(mac)
	if !ok {
		t.Fatal("could not simulate automatic ownership")
	}
	_, err := service.CollectPreview(context.Background(), mac, CollectOptions{Trigger: TriggerManual})
	if KindOf(err) != ErrorBusy {
		releaseAutomatic()
		t.Fatalf("manual sync while automatic owns host error=%v kind=%q, want busy", err, KindOf(err))
	}
	releaseAutomatic()

	releaseManual, ok := service.tryBegin(mac)
	if !ok {
		t.Fatal("could not simulate manual ownership")
	}
	err = service.RunAutomatic(context.Background(), models.ProxmoxAPIConfig{HypervisorMac: mac})
	if KindOf(err) != ErrorBusy {
		releaseManual()
		t.Fatalf("automatic sync while manual owns host error=%v kind=%q, want busy", err, KindOf(err))
	}
	releaseManual()
}
