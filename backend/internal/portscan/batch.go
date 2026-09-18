package portscan

import (
	"context"
	"strconv"
	"sync"
)

const (
	defaultMaxWorkers = 32
	absoluteMaxWorkers = 64
)

// PortResult pairs a requested TCP port with its structured probe outcome.
type PortResult struct {
	Port   int
	Result Result
}

// ScanPorts probes a set of TCP ports with bounded concurrency. Results are
// returned in the same normalized order as the requested ports.
func ScanPorts(ctx context.Context, host string, ports []int, maxWorkers int) []PortResult {
	return scanPortsWithProbe(ctx, host, ports, maxWorkers, Probe)
}

type probeFunc func(context.Context, string, string) Result

func scanPortsWithProbe(ctx context.Context, host string, ports []int, maxWorkers int, probe probeFunc) []PortResult {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized := normalizePorts(ports)
	if len(normalized) == 0 {
		return []PortResult{}
	}
	if probe == nil {
		probe = Probe
	}

	workers := normalizeWorkerCount(maxWorkers, len(normalized))
	type job struct {
		index int
		port  int
	}
	jobs := make(chan job)
	results := make([]PortResult, len(normalized))
	for i, port := range normalized {
		results[i] = PortResult{Port: port, Result: Result{State: ProbeCanceled, Err: context.Canceled}}
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wg.Done()
			for item := range jobs {
				if ctx.Err() != nil {
					results[item.index].Result = Result{State: ProbeCanceled, Err: ctx.Err()}
					continue
				}
				results[item.index].Result = probe(ctx, host, strconv.Itoa(item.port))
			}
		}()
	}

sendLoop:
	for index, port := range normalized {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- job{index: index, port: port}:
		}
	}
	close(jobs)
	wg.Wait()

	return results
}

func normalizePorts(ports []int) []int {
	normalized := make([]int, 0, len(ports))
	seen := make(map[int]struct{}, len(ports))
	for _, port := range ports {
		if port < 1 || port > 65535 {
			continue
		}
		if _, exists := seen[port]; exists {
			continue
		}
		seen[port] = struct{}{}
		normalized = append(normalized, port)
	}
	return normalized
}

func normalizeWorkerCount(maxWorkers, jobs int) int {
	if maxWorkers <= 0 {
		maxWorkers = defaultMaxWorkers
	}
	if maxWorkers > absoluteMaxWorkers {
		maxWorkers = absoluteMaxWorkers
	}
	if maxWorkers > jobs {
		maxWorkers = jobs
	}
	if maxWorkers < 1 {
		return 1
	}
	return maxWorkers
}
