package portscan

import (
	"net"
	"testing"
	"time"
)

func TestIsOpenDetectsLocalIPv4Listener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	accepted := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
		close(accepted)
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}

	if !IsOpen(host, port) {
		t.Fatalf("IsOpen(%q, %q) = false, want true", host, port)
	}

	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("listener did not accept the connection")
	}
}

func TestIsOpenReturnsFalseForClosedIPv4Port(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		listener.Close()
		t.Fatalf("SplitHostPort: %v", err)
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close: %v", err)
	}

	if IsOpen(host, port) {
		t.Fatalf("IsOpen(%q, %q) = true, want false", host, port)
	}
}
