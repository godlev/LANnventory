package portscan

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestScanPortsBoundsConcurrencyAndPreservesNormalizedOrder(t *testing.T) {
	ports := []int{22, 80, 443, 8080, 8443, 32400, 22, 0, 65536}
	var active atomic.Int32
	var maximum atomic.Int32

	probe := func(ctx context.Context, host, port string) Result {
		current := active.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		defer active.Add(-1)

		select {
		case <-ctx.Done():
			return Result{State: ProbeCanceled, Err: ctx.Err()}
		case <-time.After(10 * time.Millisecond):
		}
		if _, err := strconv.Atoi(port); err != nil {
			t.Errorf("probe received invalid port %q", port)
		}
		return Result{State: ProbeOpen}
	}

	results := scanPortsWithProbe(context.Background(), "192.168.1.5", ports, 3, probe)
	wantPorts := []int{22, 80, 443, 8080, 8443, 32400}
	if len(results) != len(wantPorts) {
		t.Fatalf("results len = %d, want %d: %+v", len(results), len(wantPorts), results)
	}
	for i, wantPort := range wantPorts {
		if results[i].Port != wantPort || results[i].Result.State != ProbeOpen {
			t.Fatalf("result[%d] = %+v, want port %d open", i, results[i], wantPort)
		}
	}
	if got := maximum.Load(); got < 2 || got > 3 {
		t.Fatalf("maximum concurrency = %d, want between 2 and 3", got)
	}
}

func TestScanPortsHonorsCanceledContextBeforeScheduling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls atomic.Int32
	probe := func(context.Context, string, string) Result {
		calls.Add(1)
		return Result{State: ProbeOpen}
	}

	results := scanPortsWithProbe(ctx, "192.168.1.5", []int{22, 80, 443}, 2, probe)
	if calls.Load() != 0 {
		t.Fatalf("probe calls = %d, want 0", calls.Load())
	}
	if len(results) != 3 {
		t.Fatalf("results len = %d, want 3", len(results))
	}
	for _, result := range results {
		if result.Result.State != ProbeCanceled {
			t.Fatalf("result = %+v, want canceled", result)
		}
	}
}

func TestNormalizeWorkerCountUsesSafeBounds(t *testing.T) {
	if got := normalizeWorkerCount(0, 100); got != defaultMaxWorkers {
		t.Fatalf("default workers = %d, want %d", got, defaultMaxWorkers)
	}
	if got := normalizeWorkerCount(1000, 1000); got != absoluteMaxWorkers {
		t.Fatalf("capped workers = %d, want %d", got, absoluteMaxWorkers)
	}
	if got := normalizeWorkerCount(20, 3); got != 3 {
		t.Fatalf("workers for 3 jobs = %d, want 3", got)
	}
}
