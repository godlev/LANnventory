package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestFrontendEntryRoutesUseRevalidationHeaders(t *testing.T) {
	router := NewRouter()

	for _, path := range []string{"/", "/config", "/history", "/activity", "/host/1"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Header().Get("Cache-Control") != "no-cache, max-age=0, must-revalidate" {
				t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestFrontendAssetsUseRevalidationHeaders(t *testing.T) {
	router := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/fs/public/assets/index.js", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Cache-Control") != "no-cache, max-age=0, must-revalidate" {
		t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

func TestPublicImagesDoNotUseFrontendRevalidationHeaders(t *testing.T) {
	router := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/fs/public/favicon.png", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Cache-Control") == "no-cache, max-age=0, must-revalidate" {
		t.Fatal("favicon unexpectedly received frontend asset revalidation headers")
	}
}
