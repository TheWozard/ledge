package cmd

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/TheWozard/ledge-frontend/ui"
	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/bar"
	"github.com/TheWozard/ledge/internal/config"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/TheWozard/ledge/internal/store"
	"github.com/spf13/cobra"
)

// pruneEvery is how often expired events are dropped.
const pruneEvery = 30 * time.Second

var (
	runConfig   string
	runHeadless bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the bar and the JSON API",
	Long: `Run the bar and the JSON API, on the socket ledge.sock in the state dir.

--headless runs only the API, with no windows.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(runConfig)
		if err != nil {
			return err
		}
		dir, err := state.Dir()
		if err != nil {
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
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		// POST /shutdown stops it as a signal would.
		ctx, shutdown := context.WithCancel(ctx)
		defer shutdown()
		server := &api.Server{Store: events, Shutdown: shutdown}
		if runHeadless {
			return serve(ctx, listener, server)
		}

		// AppKit's main loop takes the main goroutine; the API runs beside it, and
		// stopping either stops both.
		ui.Init()
		b, err := bar.Open(events, cfg, dir)
		if err != nil {
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
	},
}

// serve runs the API until ctx is done, dropping expired events as it goes.
func serve(ctx context.Context, listener net.Listener, server *api.Server) error {
	httpServer := &http.Server{Handler: server, ReadHeaderTimeout: 5 * time.Second}
	failed := make(chan error, 1)
	go func() { failed <- httpServer.Serve(listener) }()
	log.Printf("ledge: API on %s", listener.Addr())

	ticker := time.NewTicker(pruneEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pruned, err := server.Store.Prune(time.Now())
			if err != nil {
				log.Printf("ledge: dropping expired events: %v", err)
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

func init() {
	runCmd.Flags().StringVar(&runConfig, "config", "", "window settings `PATH` (JSON; default: built-in defaults)")
	runCmd.Flags().BoolVar(&runHeadless, "headless", false, "run only the API, with no windows")
	rootCmd.AddCommand(runCmd)
}
