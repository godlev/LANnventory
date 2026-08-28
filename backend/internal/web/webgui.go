package web

import (
	"context"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/api"
	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/prometheus"
	"github.com/godlev/LANnventory/internal/version"
)

// templFS - html templates
//
//go:embed templates/*
var templFS embed.FS

// pubFS - public folder
//
//go:embed public/*
var pubFS embed.FS

// NewRouter builds the LANnventory web/API router without starting a listener.
func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	templ := template.Must(template.New("").ParseFS(templFS, "templates/*"))
	router.SetHTMLTemplate(templ) // templates

	router.StaticFS("/fs/", http.FS(pubFS)) // public

	router.GET("/", indexHandler)          // index.go
	router.GET("/config", indexHandler)    // index.go
	router.GET("/history", indexHandler)   // index.go
	router.GET("/activity", indexHandler)  // index.go
	router.GET("/host/*any", indexHandler) // index.go
	router.GET("/metrics", prometheus.Handler())

	api.Routes(router)

	return router
}

// Gui - start web server
func Gui() {
	check.IfError(GuiContext(context.Background()))
}

// GuiContext starts the web server and shuts it down when ctx is cancelled.
func GuiContext(ctx context.Context) error {
	const (
		colorCyan  = "\033[36m"
		colorReset = "\033[0m"
	)

	conf.SetVersion(version.Version)

	config := conf.GetAppConfig()
	address := config.Host + ":" + config.Port

	slog.Info(colorCyan + "\n=================================== " +
		"\n  LANnventory Version: " + config.Version +
		"\n  Config dir: " + config.DirPath +
		"\n  Default DB: " + config.UseDB +
		"\n  Log level: " + config.LogLevel +
		"\n  Web GUI: http://" + address +
		"\n=================================== " + colorReset)

	server := &http.Server{
		Addr:    address,
		Handler: NewRouter(),
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Warn("Web server shutdown failed", "err", err)
		}
	}()

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
