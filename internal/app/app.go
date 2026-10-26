// Package app runs a ledge: the store of a state dir, its API on the dir's socket, and
// unless it's headless, the bar.
package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/TheWozard/ledge-frontend/ui"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/bar"
	"github.com/TheWozard/ledge/internal/config"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/TheWozard/ledge/internal/store"
)

// PruneEvery is how often expired events are dropped.
var PruneEvery = 30 * time.Second

// Run runs the ledge of dir, the state dir, with options, until ctx is done, POST
// /shutdown asks, or the bar quits. With a bar, call it from the main goroutine:
// AppKit's main loop needs it.
func Run(ctx context.Context, dir string, options api.Options) error {
	cfg, err := config.Load(options.Config)
	if err != nil {
		return err
	}
	if err := state.Make(dir); err != nil {
		return err
	}
	events, err := store.Open(filepath.Join(dir, "events.json"))
	if err != nil {
		return err
	}
	if _, err := events.Prune(time.Now()); err != nil {
		return err
	}
	listener, err := api.Listen(state.Socket(dir))
	if err != nil {
		return err
	}
	// POST /shutdown stops it as ctx would.
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	server := &api.Server{Store: events, Shutdown: stop, Commands: cfg.Commands,
		Started: time.Now().UTC().Truncate(time.Second), Options: options}
	if options.Headless {
		return serve(ctx, listener, server)
	}

	// AppKit's main loop takes the main goroutine; the API runs beside it, and
	// stopping either stops both.
	ui.Init()
	b, err := bar.Open(events, cfg, dir)
	if err != nil {
		listener.Close()
		return err
	}
	server.Changed = b.Render
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, listener, server)
		ui.Stop()
	}()
	ui.Run()
	stop()
	return <-done
}

// serve runs the API until ctx is done, dropping expired events as it goes.
func serve(ctx context.Context, listener net.Listener, server *api.Server) error {
	httpServer := &http.Server{Handler: server, ReadHeaderTimeout: 5 * time.Second}
	failed := make(chan error, 1)
	go func() { failed <- httpServer.Serve(listener) }()
	log.Printf("API on %s", listener.Addr())

	ticker := time.NewTicker(PruneEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pruned, err := server.Store.Prune(time.Now())
			if err != nil {
				log.Printf("dropping expired events: %v", err)
			} else if pruned && server.Changed != nil {
				server.Changed()
			}
		case err := <-failed:
			return err
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return httpServer.Shutdown(shutdown)
		}
	}
}
