package web

import (
	"context"
	"errors"
	"net"
	"net/http"
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
