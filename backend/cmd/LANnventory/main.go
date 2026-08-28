// @title LANnventory API
// @version 0.1.0-beta.1
// @description Local network inventory and monitoring application
// @contact.url   https://github.com/godlev/LANnventory
// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT
// @BasePath /api/

package main

import (
	"context"
	"flag"
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

func main() {
	dirPtr := flag.String("d", dirPath, "Path to config dir")
	nodePtr := flag.String("n", nodePath, "Path to node modules")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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
	conf.Start(*dirPtr, *nodePtr)

	gdb.Start()
	defer func() {
		check.IfError(gdb.Close())
	}()

	routines.ScanRestart()
	defer routines.ScanStop()

	routines.HistoryTrimContext(ctx)

	check.IfError(web.GuiContext(ctx))
}
