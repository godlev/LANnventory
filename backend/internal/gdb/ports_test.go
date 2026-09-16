package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestReconcileHostPortObservationsRecordsOnlyTransitions(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:         9,
		Name:       "server",
		IP:         "192.168.1.90",
		Mac:        "AA:BB:CC:DD:EE:90",
		Iface:      "eth0",
		DeviceType: "server",
		Known:      1,
		Now:        1,
	}
	seedExportCurrentHost(t, host)

	t1 := "2026-09-16 10:00:00"
	events, err := ReconcileHostPortObservations(host, map[int]bool{22: true, 80: false}, t1)
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if len(events) != 1 || events[0].EventType != string(models.EventPortOpen) || events[0].NewValue != "22" {
		t.Fatalf("first events = %+v, want one port-open 22", events)
	}

	states, err := SelectHostPorts(host.ID)
	if err != nil {
		t.Fatalf("SelectHostPorts first: %v", err)
	}
	if len(states) != 1 || states[0].Port != 22 || !states[0].Open {
		t.Fatalf("first states = %+v, want only open 22", states)
	}
	if states[0].FirstSeen != t1 || states[0].LastScanned != t1 || states[0].LastChanged != t1 {
		t.Fatalf("first timestamps = %+v", states[0])
	}

	t2 := "2026-09-16 10:05:00"
	events, err = ReconcileHostPortObservations(host, map[int]bool{22: true}, t2)
	if err != nil {
		t.Fatalf("same-state reconcile: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("same open state created events: %+v", events)
	}

	states, err = SelectHostPorts(host.ID)
	if err != nil {
		t.Fatalf("SelectHostPorts second: %v", err)
	}
	if states[0].FirstSeen != t1 || states[0].LastChanged != t1 || states[0].LastScanned != t2 {
		t.Fatalf("same-state timestamps = %+v", states[0])
	}

	t3 := "2026-09-16 10:10:00"
	events, err = ReconcileHostPortObservations(host, map[int]bool{22: false}, t3)
	if err != nil {
		t.Fatalf("close reconcile: %v", err)
	}
	if len(events) != 1 || events[0].EventType != string(models.EventPortClosed) || events[0].NewValue != "22" {
		t.Fatalf("close events = %+v, want one port-closed 22", events)
	}

	t4 := "2026-09-16 10:15:00"
	events, err = ReconcileHostPortObservations(host, map[int]bool{22: true}, t4)
	if err != nil {
		t.Fatalf("reopen reconcile: %v", err)
	}
	if len(events) != 1 || events[0].EventType != string(models.EventPortOpen) || events[0].NewValue != "22" {
		t.Fatalf("reopen events = %+v, want one port-open 22", events)
	}

	states, err = SelectHostPorts(host.ID)
	if err != nil {
		t.Fatalf("SelectHostPorts final: %v", err)
	}
	if len(states) != 1 || !states[0].Open || states[0].FirstSeen != t1 || states[0].LastChanged != t4 || states[0].LastScanned != t4 {
		t.Fatalf("final state = %+v", states)
	}

	var storedEvents []models.HostEvent
	if err := db.Table("events").Order("\"ID\" ASC").Find(&storedEvents).Error; err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(storedEvents) != 3 {
		t.Fatalf("stored events len = %d, want 3: %+v", len(storedEvents), storedEvents)
	}
	wantTypes := []models.HostEventType{models.EventPortOpen, models.EventPortClosed, models.EventPortOpen}
	for i, want := range wantTypes {
		if storedEvents[i].EventType != string(want) {
			t.Fatalf("storedEvents[%d] = %q, want %q", i, storedEvents[i].EventType, want)
		}
	}
}

func TestDeleteCurrentHostRemovesPortStates(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:    4,
		Name:  "nas",
		IP:    "192.168.1.20",
		Mac:   "AA:BB:CC:DD:EE:20",
		Known: 1,
		Now:   1,
	}
	seedExportCurrentHost(t, host)

	if _, err := ReconcileHostPortObservations(host, map[int]bool{445: true}, "2026-09-16 11:00:00"); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if err := DeleteCurrentHostWithMetadata(host); err != nil {
		t.Fatalf("DeleteCurrentHostWithMetadata: %v", err)
	}

	states, err := SelectHostPorts(host.ID)
	if err != nil {
		t.Fatalf("SelectHostPorts after delete: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("port states remain after host delete: %+v", states)
	}
}


func TestReconcileHostPortObservationsRejectsMissingHost(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:   99,
		Name: "deleted",
		IP:   "192.168.1.99",
		Mac:  "AA:BB:CC:DD:EE:99",
	}

	if _, err := ReconcileHostPortObservations(host, map[int]bool{22: true}, "2026-09-16 12:00:00"); err == nil {
		t.Fatal("missing host reconcile returned nil error")
	}

	states, err := SelectHostPorts(host.ID)
	if err != nil {
		t.Fatalf("SelectHostPorts missing host: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("missing host created port state: %+v", states)
	}

	var events []models.HostEvent
	if err := db.Table("events").Find(&events).Error; err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("missing host created events: %+v", events)
	}
}
