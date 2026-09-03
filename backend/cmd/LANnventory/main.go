// @title LANnventory API
// @version 0.1.0-beta.2
// @description Local network inventory and monitoring application
// @contact.url   https://github.com/godlev/LANnventory
// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT
// @BasePath /api/

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	// "net/http"

	// _ "net/http/pprof"

	// Import Swagger docs
	_ "github.com/godlev/LANnventory/docs"

	"github.com/godlev/LANnventory/internal/check"
	"github.com/godlev/LANnventory/internal/conf"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/routines"
	"github.com/godlev/LANnventory/internal/web"
)

const dirPath = "/data/WatchYourLAN"
const nodePath = ""

type appRuntime struct {
	startConfig func(dirPath, nodePath string)
	startDB     func() error
	closeDB     func() error
	scanRestart func()
	scanStop    func()
	historyTrim func(context.Context)
	webGUI      func(context.Context) error
}

func main() {
	if err := run(context.Background(), os.Args[1:], defaultAppRuntime()); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		slog.Error("LANnventory stopped due to fatal runtime error", "err", err)
		os.Exit(1)
	}
}

func defaultAppRuntime() appRuntime {
	return appRuntime{
		startConfig: conf.Start,
		startDB:     gdb.StartErr,
		closeDB:     gdb.Close,
		scanRestart: routines.ScanRestart,
		scanStop:    routines.ScanStop,
		historyTrim: routines.HistoryTrimContext,
		webGUI:      web.GuiContext,
	}
}

func run(parentCtx context.Context, args []string, runtime appRuntime) error {
	flags := flag.NewFlagSet("LANnventory", flag.ContinueOnError)
	dirPtr := flags.String("d", dirPath, "Path to config dir")
	nodePtr := flags.String("n", nodePath, "Path to node modules")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// pprof - memory leak detect
	// go tool pprof -alloc_space http://localhost:8085/debug/pprof/heap
	// (pprof) web
	// (pprof) list db.Select
	//
	// go func() {
	// 	http.ListenAndServe("localhost:8085", nil)
	// }()

	// Make AppConfig
	runtime.startConfig(*dirPtr, *nodePtr)

	if err := runtime.startDB(); err != nil {
		return err
	}
	defer func() {
		check.IfError(runtime.closeDB())
	}()

	runtime.scanRestart()
	defer runtime.scanStop()

	runtime.historyTrim(ctx)

	return runtime.webGUI(ctx)
}
