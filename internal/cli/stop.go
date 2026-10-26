package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running ledge",
	Long: `Stop the ledge that answers on the state dir's socket, however it was started: ask it
to shut down through POST /shutdown, and kill it, by the pid it gave, if it still
answers after 5 seconds.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		process, err := backgroundProcess()
		if err != nil {
			return err
		}
		stopped, err := process.Stop()
		if err != nil {
			return err
		}
		if !stopped {
			fmt.Println("ledge isn't running")
			return nil
		}
		fmt.Println("ledge stopped")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
