package routines

import (
	"sync"
	"time"

	"github.com/godlev/LANnventory/internal/arp"
)

// ScannerStatus is the authoritative backend scanner lifecycle state.
type ScannerStatus string

const (
	ScannerStatusHealthy  ScannerStatus = "healthy"
	ScannerStatusProblem  ScannerStatus = "problem"
	ScannerStatusScanning ScannerStatus = "scanning"
)

// ScannerState is a read-only snapshot of the active scanner scheduler.
type ScannerState struct {
	Status               ScannerStatus
	LastScanStartedAt    time.Time
	LastScanAt           time.Time
	LastSuccessfulScanAt time.Time
	Duration             time.Duration
	DevicesFound         int
	Interfaces           []string
	LastErrors           []arp.ScanError
	NextScanAt           time.Time
}

var (
	scannerStateMu sync.RWMutex
	scannerState   ScannerState
)

// GetScannerState returns a copy safe for concurrent readers.
func GetScannerState() ScannerState {
	scannerStateMu.RLock()
	defer scannerStateMu.RUnlock()

	return cloneScannerState(scannerState)
}

func markScannerStarting() {
	scannerStateMu.Lock()
	defer scannerStateMu.Unlock()

	scannerState.Status = ScannerStatusScanning
	scannerState.NextScanAt = time.Time{}
}

func markScanStarted(started time.Time) {
	scannerStateMu.Lock()
	defer scannerStateMu.Unlock()

	scannerState.Status = ScannerStatusScanning
	scannerState.LastScanStartedAt = started
	scannerState.NextScanAt = time.Time{}
}

func markScanCompleted(result arp.ScanResult, started, completed, next time.Time, applied bool) {
	scannerStateMu.Lock()
	defer scannerStateMu.Unlock()

	success := result.Success && applied
	if success {
		scannerState.Status = ScannerStatusHealthy
		scannerState.LastSuccessfulScanAt = completed
	} else {
		scannerState.Status = ScannerStatusProblem
	}

	scannerState.LastScanStartedAt = started
	scannerState.LastScanAt = completed
	scannerState.Duration = completed.Sub(started)
	if scannerState.Duration < 0 {
		scannerState.Duration = 0
	}
	scannerState.DevicesFound = len(result.Hosts)
	scannerState.Interfaces = append([]string(nil), result.Interfaces...)
	scannerState.LastErrors = append([]arp.ScanError(nil), result.Errors...)
	scannerState.NextScanAt = next
}

func cloneScannerState(state ScannerState) ScannerState {
	state.Interfaces = append([]string(nil), state.Interfaces...)
	state.LastErrors = append([]arp.ScanError(nil), state.LastErrors...)
	return state
}

func setScannerStateForTest(state ScannerState) {
	scannerStateMu.Lock()
	defer scannerStateMu.Unlock()
	scannerState = cloneScannerState(state)
}
