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
