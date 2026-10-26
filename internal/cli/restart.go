package cli

import (
	"fmt"
	"strings"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/config"
	"github.com/spf13/cobra"
)

var restartOptions api.Options

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Stop the running ledge and start it again in the background",
	Long: `Stop the running ledge, as ` + "`ledge stop`" + ` does, and start it again, reading its config file
afresh. Without flags it starts with the running ledge's flags, or if none is running,
the last ` + "`ledge start`" + `'s; with any flag, it uses only the flags given. Starts it if it
wasn't running. If the config is invalid, the running ledge is left running.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		process, err := backgroundProcess()
		if err != nil {
			return err
		}
		options, err := absolute(restartOptions)
		if err != nil {
			return err
		}
		if !cmd.Flags().Changed("config") && !cmd.Flags().Changed("headless") {
			if status, ok := process.Status(); ok {
				options = status.Options
			} else if last, ok, err := process.LastOptions(); err != nil {
				return err
			} else if ok {
				options = last
			}
			if args := options.Args(); len(args) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Using the flags: %s\n", strings.Join(args, " "))
			}
		}
		// A bad config leaves the running ledge running.
		if _, err := config.Load(options.Config); err != nil {
			return err
		}
		stopped, err := process.Stop()
		if err != nil {
			return err
		}
		if stopped {
			fmt.Fprintln(cmd.OutOrStdout(), "ledge stopped")
		}
		return startWith(cmd, options)
	},
}

func init() {
	registerRunFlags(restartCmd, &restartOptions)
	rootCmd.AddCommand(restartCmd)
}
