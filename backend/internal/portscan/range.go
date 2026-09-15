package portscan

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultWorkers = 128
	DefaultTimeout = 500 * time.Millisecond
)

type Result struct {
	Port int
	Open bool
}

// IsOpenContext checks one TCP port with cancellation support.
func IsOpenContext(ctx context.Context, host string, port int, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	dialer := net.Dialer{Timeout: timeout}
	target := targetAddress(host, strconv.Itoa(port))
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ScanRange scans a TCP range with bounded concurrency. Results are reported as
// probes finish, so callers can maintain progress without issuing one HTTP
// request per port.
func ScanRange(
	ctx context.Context,
	host string,
	start int,
	end int,
	workers int,
	timeout time.Duration,
	onResult func(Result),
) error {
	if start < 1 {
		start = 1
	}
	if end > 65535 {
		end = 65535
	}
	if start > end {
		start, end = end, start
	}
	if workers < 1 {
		workers = DefaultWorkers
	}
	if workers > 512 {
		workers = 512
	}

	ports := make(chan int)
	results := make(chan Result, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range ports {
				if ctx.Err() != nil {
					return
				}
				open := IsOpenContext(ctx, host, port, timeout)
				if ctx.Err() != nil {
					return
				}
				results <- Result{Port: port, Open: open}
			}
		}()
	}

	go func() {
		defer close(ports)
		for port := start; port <= end; port++ {
			select {
			case ports <- port:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if onResult != nil {
			onResult(result)
		}
	}

	return ctx.Err()
}
