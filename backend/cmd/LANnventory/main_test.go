package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunReturnsWebGUIErrorAndRunsCleanup(t *testing.T) {
	wantErr := errors.New("listen tcp 127.0.0.1:8840: bind: cannot assign requested address")
	calls := []string{}

	err := run(context.Background(), []string{"-d", "/tmp/lannventory", "-n", "/tmp/node"}, appRuntime{
		startConfig: func(dirPath, nodePath string) {
			calls = append(calls, "startConfig")
			if dirPath != "/tmp/lannventory" {
				t.Fatalf("dirPath = %q, want /tmp/lannventory", dirPath)
			}
			if nodePath != "/tmp/node" {
				t.Fatalf("nodePath = %q, want /tmp/node", nodePath)
			}
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
		historyTrim: func(context.Context) {
			calls = append(calls, "historyTrim")
		},
		webGUI: func(context.Context) error {
			calls = append(calls, "webGUI")
			return wantErr
		},
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("run() error = %v, want %v", err, wantErr)
	}

	wantCalls := []string{"startConfig", "startDB", "scanRestart", "historyTrim", "webGUI", "scanStop", "closeDB"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunReturnsDatabaseStartErrorWithoutStartingRoutines(t *testing.T) {
	wantErr := errors.New("database start failed")
	calls := []string{}

	err := run(context.Background(), nil, appRuntime{
		startConfig: func(string, string) {
			calls = append(calls, "startConfig")
		},
		startDB: func() error {
			calls = append(calls, "startDB")
			return wantErr
		},
		closeDB: func() error {
			t.Fatal("closeDB should not be called when startDB fails")
			return nil
		},
		scanRestart: func() {
			t.Fatal("scanRestart should not be called when startDB fails")
		},
		scanStop: func() {
			t.Fatal("scanStop should not be called when startDB fails")
		},
		historyTrim: func(context.Context) {
			t.Fatal("historyTrim should not be called when startDB fails")
		},
		webGUI: func(context.Context) error {
			t.Fatal("webGUI should not be called when startDB fails")
			return nil
		},
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("run() error = %v, want %v", err, wantErr)
	}

	wantCalls := []string{"startConfig", "startDB"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunReturnsNilAfterGracefulWebShutdown(t *testing.T) {
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

	wantCalls := []string{"startConfig", "startDB", "scanRestart", "historyTrim", "webGUI", "scanStop", "closeDB"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestPackagedSystemdServiceRetriesStartupFailures(t *testing.T) {
	servicePath := filepath.Join("..", "..", "configs", "watchyourlan.service")
	service, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", servicePath, err)
	}

	content := string(service)
	for _, directive := range []string{
		"After=network-online.target",
		"Wants=network-online.target",
		"Restart=on-failure",
		"RestartSec=5s",
	} {
		if !strings.Contains(content, directive) {
			t.Fatalf("systemd service missing %q", directive)
		}
	}
}

func TestGoReleaserPackagesSystemdServiceAsLannventoryService(t *testing.T) {
	configPath := filepath.Join("..", "..", ".goreleaser.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", configPath, err)
	}

	content := string(config)
	for _, expected := range []string{
		"src: ./configs/watchyourlan.service",
		"dst: /lib/systemd/system/lannventory.service",
		"dst: lannventory.service",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("GoReleaser config missing %q", expected)
		}
	}
}
