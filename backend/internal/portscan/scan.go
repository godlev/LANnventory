package portscan

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"
)

const defaultProbeTimeout = 3 * time.Second

// ProbeState distinguishes definitive TCP service state from transport failures.
type ProbeState string

const (
	ProbeOpen          ProbeState = "open"
	ProbeClosed        ProbeState = "closed"
	ProbeIndeterminate ProbeState = "indeterminate"
	ProbeCanceled      ProbeState = "canceled"
)

// Result is the structured outcome of one TCP service probe.
type Result struct {
	State ProbeState
	Err   error
}

// Probe checks one TCP port without converting timeout/unreachable failures into "closed".
func Probe(ctx context.Context, host, port string) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{State: ProbeCanceled, Err: err}
	}

	target := targetAddress(host, port)
	dialer := net.Dialer{Timeout: defaultProbeTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err == nil {
		_ = conn.Close()
		return Result{State: ProbeOpen}
	}

	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return Result{State: ProbeCanceled, Err: err}
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return Result{State: ProbeClosed, Err: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Result{State: ProbeIndeterminate, Err: err}
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return Result{State: ProbeIndeterminate, Err: err}
	}

	return Result{State: ProbeIndeterminate, Err: err}
}

// IsOpen preserves the legacy boolean contract for callers that only need a best-effort check.
func IsOpen(host, port string) bool {
	return Probe(context.Background(), host, port).State == ProbeOpen
}

func targetAddress(host, port string) string {
	return net.JoinHostPort(host, port)
}
