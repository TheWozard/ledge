package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the ledge started with `ledge start`",
	Long: `Stop the ledge started with ` + "`ledge start`" + `: ask it to exit, and kill it if it hasn't
after 5 seconds. A ledge started with ` + "`ledge run`" + ` in a terminal is left alone.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		process, err := backgroundProcess(0)
		if err != nil {
			return err
		}
		stopped, err := process.Stop()
		if err != nil {
			return err
		}
		if !stopped {
			fmt.Println("ledge isn't running in the background")
			return nil
		}
		fmt.Println("ledge stopped")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
