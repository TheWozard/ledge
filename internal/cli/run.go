package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/app"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/spf13/cobra"
)

var runOptions api.Options

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the bar and the JSON API",
	Long: `Run the bar and the JSON API, on the socket ledge.sock in the state dir.

--headless runs only the API, with no windows.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		options, err := absolute(runOptions)
		if err != nil {
			return err
		}
		dir, err := state.Dir()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return app.Run(ctx, dir, options)
	},
}

func init() {
	registerRunFlags(runCmd, &runOptions)
	rootCmd.AddCommand(runCmd)
}
