package cli

import (
	"fmt"
	"path/filepath"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/background"
	"github.com/TheWozard/ledge/internal/config"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/spf13/cobra"
)

// registerRunFlags adds `ledge run`'s flags to cmd, setting options: run, start and
// restart take the same ones.
func registerRunFlags(cmd *cobra.Command, options *api.Options) {
	cmd.Flags().StringVar(&options.Config, "config", "", "settings `PATH` (JSON; default: built-in defaults)")
	cmd.Flags().BoolVar(&options.Headless, "headless", false, "run only the API, with no windows")
}

// absolute is options with the config's path made absolute: a background ledge runs
// in $HOME, and its status should say which file it read.
func absolute(options api.Options) (api.Options, error) {
	if options.Config != "" {
		path, err := filepath.Abs(options.Config)
		if err != nil {
			return options, err
		}
		options.Config = path
	}
	return options, nil
}

var startOptions api.Options

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Run ledge in the background",
	Long: `Run ledge in the background, as ` + "`ledge run`" + ` does in a terminal, and wait for its
API to answer. Its output goes to ledge.log in the state dir. Does nothing if a ledge
already answers on the state dir's socket; ` + "`ledge stop`" + ` stops it.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := absolute(startOptions)
		if err != nil {
			return err
		}
		return startWith(cmd, options)
	},
}

// startWith starts `ledge run` with options in the background, saying so on cmd's
// output.
func startWith(cmd *cobra.Command, options api.Options) error {
	process, err := backgroundProcess()
	if err != nil {
		return err
	}
	// A bad config fails straight away with its own message, before anything starts.
	if _, err := config.Load(options.Config); err != nil {
		return err
	}
	pid, started, err := process.Start(options)
	if err != nil {
		return err
	}
	if !started {
		fmt.Fprintf(cmd.OutOrStdout(), "ledge is already running (pid %d)\n", pid)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ledge started (pid %d, socket %s, log %s)\n", pid, process.Socket, process.LogFile)
	return nil
}

// backgroundProcess is the background ledge of the state dir.
func backgroundProcess() (*background.Process, error) {
	dir, err := state.Dir()
	if err != nil {
		return nil, err
	}
	return background.New(dir)
}

func init() {
	registerRunFlags(startCmd, &startOptions)
	rootCmd.AddCommand(startCmd)
}
