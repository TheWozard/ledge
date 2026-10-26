package cli

import (
	"net/url"

	"github.com/spf13/cobra"
)

var dismissCmd = &cobra.Command{
	Use:     "dismiss ID...",
	Short:   "Remove events by id",
	Long:    `Remove the events with these ids, as their cards' × does. An id that isn't an event is an error, after the others are removed.`,
	Example: `  ledge dismiss aws-auth`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := client()
		if err != nil {
			return err
		}
		var failed error
		for _, id := range args {
			var reply struct{}
			if err := client.Call("DELETE", "/events/"+url.PathEscape(id), nil, &reply); err != nil && failed == nil {
				failed = err
			}
		}
		return failed
	},
}

func init() {
	rootCmd.AddCommand(dismissCmd)
}
