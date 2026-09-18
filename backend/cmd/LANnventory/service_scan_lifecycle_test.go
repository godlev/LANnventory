package main

import (
	"context"
	"reflect"
	"testing"
)

func TestRunServiceScannerLifecycleStaysInsideDatabaseLifetime(t *testing.T) {
	calls := []string{}

	err := run(context.Background(), nil, appRuntime{
		startConfig: func(string, string) {
			calls = append(calls, "startConfig")
		},
		startDB: func() error {
			calls = append(calls, "startDB")
			return nil
		},
		closeDB: func() error {
			calls = append(calls, "closeDB")
			return nil
		},
		scanRestart: func() {
			calls = append(calls, "scanRestart")
		},
		scanStop: func() {
			calls = append(calls, "scanStop")
		},
		serviceScanRestart: func() {
			calls = append(calls, "serviceScanRestart")
		},
		serviceScanStop: func() {
			calls = append(calls, "serviceScanStop")
		},
		historyTrim: func(context.Context) {
			calls = append(calls, "historyTrim")
		},
		webGUI: func(context.Context) error {
			calls = append(calls, "webGUI")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	want := []string{
		"startConfig",
		"startDB",
		"scanRestart",
		"serviceScanRestart",
		"historyTrim",
		"webGUI",
		"serviceScanStop",
		"scanStop",
		"closeDB",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}
