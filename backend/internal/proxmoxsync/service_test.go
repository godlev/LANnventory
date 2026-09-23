package proxmoxsync

import (
	"errors"
	"testing"
)

func TestServiceErrorKindAndUnwrap(t *testing.T) {
	cause := errors.New("state failed")
	err := &ServiceError{Kind: ErrorState, Err: cause}

	if KindOf(err) != ErrorState {
		t.Fatalf("KindOf = %q, want %q", KindOf(err), ErrorState)
	}
	if !errors.Is(err, cause) {
		t.Fatal("ServiceError does not unwrap its cause")
	}
	if KindOf(cause) != "" {
		t.Fatalf("plain error unexpectedly classified as %q", KindOf(cause))
	}
}

func TestTriggerValuesAreStable(t *testing.T) {
	if TriggerManual != "manual" || TriggerAutomatic != "automatic" {
		t.Fatalf("unexpected trigger values manual=%q automatic=%q", TriggerManual, TriggerAutomatic)
	}
}


func TestServiceSameHostOwnershipIsExclusive(t *testing.T) {
	service := NewService()
	release, ok := service.tryBegin("AA:BB:CC:DD:EE:F1")
	if !ok {
		t.Fatal("first ownership acquisition failed")
	}
	if !service.IsRunning("aa:bb:cc:dd:ee:f1") {
		t.Fatal("IsRunning did not normalize host identity")
	}
	if secondRelease, ok := service.tryBegin("AA:BB:CC:DD:EE:F1"); ok {
		secondRelease()
		t.Fatal("second same-host ownership acquisition unexpectedly succeeded")
	}
	release()
	if service.IsRunning("AA:BB:CC:DD:EE:F1") {
		t.Fatal("ownership remained after release")
	}
	if releaseAgain, ok := service.tryBegin("AA:BB:CC:DD:EE:F1"); !ok {
		t.Fatal("ownership could not be reacquired after release")
	} else {
		releaseAgain()
	}
}

func TestServiceDifferentHostsMayRunConcurrently(t *testing.T) {
	service := NewService()
	releaseA, ok := service.tryBegin("AA:BB:CC:DD:EE:F2")
	if !ok {
		t.Fatal("host A acquisition failed")
	}
	defer releaseA()
	releaseB, ok := service.tryBegin("AA:BB:CC:DD:EE:F3")
	if !ok {
		t.Fatal("host B acquisition failed")
	}
	defer releaseB()
}
