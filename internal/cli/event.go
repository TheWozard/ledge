package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/spf13/cobra"
)

var eventCmd = &cobra.Command{
	Use:   "event [JSON]",
	Short: "Send an event, or a section of a source's events",
	Long: `Send JSON to ledge: an event, or a section ({"source": ..., "items": [...]})
that replaces all of a source's events. Without an argument, the JSON is read from
stdin. Prints the id to update or dismiss it by (one per item for a section).

What can be sent is defined by JSON Schemas; ` + "`ledge schema event`" + ` prints the event's.`,
	Example: `  ledge event '{"title": "Deploy finished", "priority": "high", "expires": "4h"}'
  ledge event '{"kind": "timer", "id": "aws-auth", "title": "AWS", "end": "1h", "color": "green",
                "command": "aws sso login",
                "schedule": [{"at": "50m", "color": "yellow"}, {"at": "1h", "color": "red"}]}'
  ledge event '{"source": "ci", "items": [{"id": "build", "title": "Build passed"}]}'
  some-feed | ledge event`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := client()
		if err != nil {
			return err
		}
		var text []byte
		if len(args) == 1 {
			text = []byte(args[0])
		} else if text, err = io.ReadAll(cmd.InOrStdin()); err != nil {
			return err
		}
		// A section is {"source", "items"}: its items go to PUT /sources/{source}.
		var section struct {
			Source *string            `json:"source"`
			Items  *[]json.RawMessage `json:"items"`
		}
		if json.Unmarshal(text, &section) == nil && section.Items != nil {
			if section.Source == nil || *section.Source == "" {
				return errors.New(`a section needs a "source" to replace`)
			}
			body, err := json.Marshal(map[string]any{"items": *section.Items})
			if err != nil {
				return err
			}
			var reply struct {
				IDs []string `json:"ids"`
			}
			if err := client.Call("PUT", "/sources/"+url.PathEscape(*section.Source), body, &reply); err != nil {
				return err
			}
			for _, id := range reply.IDs {
				fmt.Fprintln(cmd.OutOrStdout(), id)
			}
			return nil
		}
		var reply struct {
			ID string `json:"id"`
		}
		if err := client.Call("POST", "/events", text, &reply); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), reply.ID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventCmd)
}
