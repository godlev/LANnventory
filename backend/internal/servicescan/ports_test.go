package servicescan

import (
	"errors"
	"fmt"
	"testing"
)

func TestNormalizePortsSortsAndDeduplicates(t *testing.T) {
	ports, err := NormalizePorts([]int{443, 22, 443, 80})
	if err != nil {
		t.Fatalf("NormalizePorts: %v", err)
	}
	want := []int{22, 80, 443}
	if fmt.Sprint(ports) != fmt.Sprint(want) {
		t.Fatalf("ports = %v, want %v", ports, want)
	}
}

func TestNormalizePortsRejectsInvalidAndOversizedLists(t *testing.T) {
	if _, err := NormalizePorts([]int{0, 443}); !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("invalid port error = %v, want %v", err, ErrInvalidPort)
	}

	ports := make([]int, MaxScheduledPorts+1)
	for i := range ports {
		ports[i] = i + 1
	}
	if _, err := NormalizePorts(ports); !errors.Is(err, ErrTooManyPorts) {
		t.Fatalf("oversized error = %v, want %v", err, ErrTooManyPorts)
	}
}

func TestPortsJSONCanonicalization(t *testing.T) {
	encoded, ports, err := CanonicalPortsJSON(" [443, 22, 443] ")
	if err != nil {
		t.Fatalf("CanonicalPortsJSON: %v", err)
	}
	if encoded != "[22,443]" || fmt.Sprint(ports) != "[22 443]" {
		t.Fatalf("encoded=%q ports=%v", encoded, ports)
	}

	encoded, ports, err = CanonicalPortsJSON("")
	if err != nil {
		t.Fatalf("CanonicalPortsJSON empty: %v", err)
	}
	if encoded != "[]" || len(ports) != 0 {
		t.Fatalf("empty encoded=%q ports=%v", encoded, ports)
	}
}
