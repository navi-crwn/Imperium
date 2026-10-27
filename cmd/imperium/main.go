// Imperium: the control center of the HOP family. One binary, SQLite, no build step.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/navi-crwn/imperium/cmd/imperium/webfs"
	"github.com/navi-crwn/imperium/internal/app"
	"github.com/navi-crwn/imperium/internal/db"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "-version") {
		fmt.Println("imperium", version)
		return
	}
	addr := flag.String("addr", "", "listen address (default IMPERIUM_ADDR or 127.0.0.1:8100)")
	flag.Parse()
	cfg := app.LoadConfig(version)
	if *addr != "" {
		cfg.Addr = *addr
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	a, err := app.New(cfg, store, webfs.FS)
	if err != nil {
		log.Fatalf("app: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go a.Sweep(ctx)
	srv := &http.Server{Addr: cfg.Addr, Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("imperium %s listening on %s (HOP ID: %s)", version, cfg.Addr, cfg.HopURL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
