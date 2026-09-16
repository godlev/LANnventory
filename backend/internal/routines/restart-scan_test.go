package routines

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/conf"
)

func setupRestartScanTest(t *testing.T, fn func(context.Context)) {
	t.Helper()

	scanRestartMu.Lock()
	oldCancel := scanCancel
	oldDone := scanDone
	oldStartScanFunc := startScanFunc
	oldLogLevel := conf.AppConfig.LogLevel
	scanCancel = nil
	scanDone = nil
	startScanFunc = fn
	conf.AppConfig.LogLevel = "info"
	scanRestartMu.Unlock()

	t.Cleanup(func() {
		ScanStop()

		scanRestartMu.Lock()
		scanCancel = oldCancel
		scanDone = oldDone
		startScanFunc = oldStartScanFunc
		conf.AppConfig.LogLevel = oldLogLevel
		scanRestartMu.Unlock()
	})
}

func TestScanRestartConcurrentDoesNotPanic(t *testing.T) {
	setupRestartScanTest(t, func(ctx context.Context) {
		<-ctx.Done()
	})

	var wg sync.WaitGroup
	panics := make(chan any, 32)

	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics <- r
				}
			}()

			ScanRestart()
		}()
	}

	wg.Wait()
	close(panics)

	for panicValue := range panics {
		t.Fatalf("ScanRestart panicked: %v", panicValue)
	}
}

func TestScanRestartStopsPreviousRoutineBeforeReplacement(t *testing.T) {
	firstStarted := make(chan struct{})
	firstStopped := make(chan struct{})
	secondStarted := make(chan struct{})
	var startCount int
	var countMu sync.Mutex

	setupRestartScanTest(t, func(ctx context.Context) {
		countMu.Lock()
		startCount++
		count := startCount
		countMu.Unlock()

		if count == 1 {
			close(firstStarted)
			<-ctx.Done()
			close(firstStopped)
			return
		}

		if count == 2 {
			close(secondStarted)
		}
		<-ctx.Done()
	})

	ScanRestart()
	waitForRoutineSignal(t, firstStarted, "first scanner start")

	ScanRestart()
	waitForRoutineSignal(t, firstStopped, "first scanner stop")
	waitForRoutineSignal(t, secondStarted, "replacement scanner start")
}

func waitForRoutineSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}
