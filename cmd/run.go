package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ledge/internal/api"
	"ledge/internal/bar"
	"ledge/internal/config"
	"ledge/internal/shim"
	"ledge/internal/state"
	"ledge/internal/store"
)

// pruneEvery is how often expired events are dropped.
const pruneEvery = 30 * time.Second

var (
	runConfig   string
	runPort     int
	runHeadless bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the bar and the JSON API",
	Long: `Run the bar and the JSON API on 127.0.0.1.

--headless runs only the API, with no windows.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(runConfig)
		if err != nil {
			return err
		}
		port := runPort
		if port == 0 {
			if port, err = state.Port(); err != nil {
				return err
			}
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
		listener, err := api.Listen(port)
		if err != nil {
			return err
		}
		server := &api.Server{Store: events}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if runHeadless {
			return serve(ctx, listener, server)
		}

		// AppKit's main loop takes the main goroutine; the API runs beside it, and
		// stopping either stops both.
		shim.Init()
		server.Changed = bar.Open(events, cfg, dir).Render
		done := make(chan error, 1)
		go func() {
			done <- serve(ctx, listener, server)
			shim.Stop()
		}()
		shim.Run()
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
	runCmd.Flags().IntVar(&runPort, "port", 0, fmt.Sprintf("API port (default: $LEDGE_PORT or %d)", state.DefaultPort))
	runCmd.Flags().BoolVar(&runHeadless, "headless", false, "run only the API, with no windows")
	rootCmd.AddCommand(runCmd)
}
