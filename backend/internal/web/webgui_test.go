package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/models"
)

func TestGuiContextReturnsListenerStartupError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("net.SplitHostPort: %v", err)
	}

	previousConfig := conf.GetAppConfig()
	conf.SetAppConfigForTest(models.Conf{
		Host:     "127.0.0.1",
		Port:     port,
		DirPath:  t.TempDir(),
		UseDB:    "sqlite",
		LogLevel: "info",
	})
	t.Cleanup(func() {
		conf.SetAppConfigForTest(previousConfig)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = GuiContext(ctx)
	if err == nil {
		t.Fatal("GuiContext() error = nil, want listener startup error")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("GuiContext() error = %v, want listener startup error", err)
	}
}

func TestProxmoxCollectorDownload(t *testing.T) {
	router := NewRouter()
	request := httptest.NewRequest(http.MethodGet, "/lannventory-proxmox-collector.py", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("collector download status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Disposition"); got != `attachment; filename="lannventory-proxmox-collector.py"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/x-python") {
		t.Fatalf("Content-Type = %q, want text/x-python", got)
	}
	body := response.Body.String()
	if !strings.HasPrefix(body, "#!/usr/bin/env python3") {
		t.Fatalf("collector download missing Python shebang: %q", body[:min(len(body), 80)])
	}
	if !strings.Contains(body, `SOURCE = "script-import"`) {
		t.Fatal("collector download missing script-import source contract")
	}
	if strings.Contains(body, "requests.") || strings.Contains(body, "urllib.") || strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatal("collector download unexpectedly contains network-transfer code or URLs")
	}
}
