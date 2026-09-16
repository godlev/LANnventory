package routines

import (
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/arp"
)

func TestGetScannerStateReturnsIndependentSnapshot(t *testing.T) {
	original := ScannerState{
		Status:     ScannerStatusProblem,
		Interfaces: []string{"eth0"},
		LastErrors: []arp.ScanError{{Source: "eth0", Message: "timeout"}},
	}
	setScannerStateForTest(original)
	t.Cleanup(func() {
		setScannerStateForTest(ScannerState{})
	})

	snapshot := GetScannerState()
	snapshot.Interfaces[0] = "mutated0"
	snapshot.LastErrors[0].Message = "mutated"

	again := GetScannerState()
	if again.Interfaces[0] != "eth0" {
		t.Fatalf("Interfaces mutated through snapshot: %v", again.Interfaces)
	}
	if again.LastErrors[0].Message != "timeout" {
		t.Fatalf("LastErrors mutated through snapshot: %+v", again.LastErrors)
	}
}

func TestMarkScanCompletedFailurePreservesLastSuccessfulScan(t *testing.T) {
	lastSuccess := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	started := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)
	completed := started.Add(3 * time.Second)
	next := completed.Add(2 * time.Minute)

	setScannerStateForTest(ScannerState{
		Status:               ScannerStatusHealthy,
		LastSuccessfulScanAt: lastSuccess,
	})
	t.Cleanup(func() {
		setScannerStateForTest(ScannerState{})
	})

	result := arp.ScanResult{
		Success:    false,
		Interfaces: []string{"eth0"},
		Errors: []arp.ScanError{{
			Source:  "eth0",
			Kind:    arp.ScanErrorTimeout,
			Message: "command timed out",
		}},
	}
	markScanCompleted(result, started, completed, next, false)

	state := GetScannerState()
	if state.Status != ScannerStatusProblem {
		t.Fatalf("Status = %q, want %q", state.Status, ScannerStatusProblem)
	}
	if !state.LastSuccessfulScanAt.Equal(lastSuccess) {
		t.Fatalf("LastSuccessfulScanAt = %v, want %v", state.LastSuccessfulScanAt, lastSuccess)
	}
	if !state.LastScanAt.Equal(completed) || !state.NextScanAt.Equal(next) {
		t.Fatalf("scan timestamps not recorded: %+v", state)
	}
	if state.Duration != 3*time.Second {
		t.Fatalf("Duration = %s, want 3s", state.Duration)
	}
	if len(state.LastErrors) != 1 || state.LastErrors[0].Source != "eth0" {
		t.Fatalf("LastErrors = %+v, want one eth0 error", state.LastErrors)
	}
}
