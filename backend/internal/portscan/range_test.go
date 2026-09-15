package portscan

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestScanRangeFindsOpenListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	results := []Result{}
	if err := ScanRange(context.Background(), "127.0.0.1", port, port, 4, time.Second, func(result Result) {
		results = append(results, result)
	}); err != nil {
		t.Fatalf("ScanRange: %v", err)
	}

	if len(results) != 1 || results[0].Port != port || !results[0].Open {
		t.Fatalf("results = %+v, want open port %d", results, port)
	}
}

func TestScanRangeNormalizesReversedRange(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	seen := map[int]bool{}
	if err := ScanRange(ctx, "127.0.0.1", port, port-1, 2, 100*time.Millisecond, func(result Result) {
		seen[result.Port] = result.Open
	}); err != nil {
		t.Fatalf("ScanRange reversed: %v", err)
	}

	if _, ok := seen[port-1]; !ok {
		t.Fatalf("missing lower port %d in %+v", port-1, seen)
	}
	if !seen[port] {
		t.Fatalf("open port %d not detected in %+v", port, seen)
	}
}

func TestServiceName(t *testing.T) {
	tests := map[int]string{
		22:    "SSH",
		443:   "HTTPS",
		445:   "SMB",
		8840:  "LANnventory",
		32400: "Plex",
		65000: "",
	}

	for port, want := range tests {
		if got := ServiceName(port); got != want {
			t.Fatalf("ServiceName(%s) = %q, want %q", strconv.Itoa(port), got, want)
		}
	}
}


func TestScanRangeCancelledContextEmitsNoResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := []Result{}
	err := ScanRange(ctx, "192.0.2.1", 1, 100, 8, time.Second, func(result Result) {
		results = append(results, result)
	})
	if err != context.Canceled {
		t.Fatalf("ScanRange cancelled error = %v, want context.Canceled", err)
	}
	if len(results) != 0 {
		t.Fatalf("cancelled scan emitted results: %+v", results)
	}
}
