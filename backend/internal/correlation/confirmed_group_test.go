package correlation

import (
	"reflect"
	"testing"
)

func TestConfirmedGroupFollowsConfirmedRelationshipsTransitively(t *testing.T) {
	decisions := []DecisionPair{
		{MacA: "02:AA:BB:CC:DD:01", MacB: "06:AA:BB:CC:DD:02", Decision: DecisionConfirmed},
		{MacA: "06:AA:BB:CC:DD:02", MacB: "0A:AA:BB:CC:DD:03", Decision: DecisionConfirmed},
		{MacA: "0E:AA:BB:CC:DD:04", MacB: "12:AA:BB:CC:DD:05", Decision: DecisionConfirmed},
	}

	got := ConfirmedGroup("02-aa-bb-cc-dd-01", decisions)
	want := []string{
		"02:AA:BB:CC:DD:01",
		"06:AA:BB:CC:DD:02",
		"0A:AA:BB:CC:DD:03",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfirmedGroup() = %v, want %v", got, want)
	}
}

func TestConfirmedGroupIgnoresRejectedRelationships(t *testing.T) {
	decisions := []DecisionPair{
		{MacA: "02:AA:BB:CC:DD:11", MacB: "06:AA:BB:CC:DD:12", Decision: DecisionRejected},
	}
	got := ConfirmedGroup("02:AA:BB:CC:DD:11", decisions)
	want := []string{"02:AA:BB:CC:DD:11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfirmedGroup() = %v, want %v", got, want)
	}
}
