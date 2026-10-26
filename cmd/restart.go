package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var restartFlags runFlags

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Stop the running ledge and start it again in the background",
	Long: `Stop the running ledge, as ` + "`ledge stop`" + ` does, and start it again, reading its config file
afresh. Without flags it starts with the flags the last ` + "`ledge start`" + ` used; with
any flag, it uses only the flags given. Starts it if it wasn't running. If the config
is invalid, the running ledge is left running.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		process, err := backgroundProcess()
		if err != nil {
			return err
		}
		args, err := restartFlags.args()
		if err != nil {
			return err
		}
		if cmd.Flags().NFlag() == 0 {
			last, ok, err := process.LastArgs()
			if err != nil {
				return err
			}
			if ok {
				args = last
				if len(args) > 0 {
					fmt.Printf("Using the last start's flags: %s\n", strings.Join(args, " "))
				}
			}
		}
		// A bad config leaves the running ledge running.
		if err := checkConfig(args); err != nil {
			return err
		}
		stopped, err := process.Stop()
		if err != nil {
			return err
		}
		if stopped {
			fmt.Println("ledge stopped")
		}
		return startWith(args)
	},
}

func init() {
	restartFlags.register(restartCmd)
	rootCmd.AddCommand(restartCmd)
}
