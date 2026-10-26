package cli

import (
	"fmt"

	"github.com/TheWozard/ledge/internal/store"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Print the current events as JSON",
	Long:  `Print the current events as JSON, as the bar shows them: highest priority first, then newest first.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := client()
		if err != nil {
			return err
		}
		var reply struct {
			Events []any `json:"events"`
		}
		if err := client.Call("GET", "/events", nil, &reply); err != nil {
			return err
		}
		data, err := store.MarshalIndent(reply.Events)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
