package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/TheWozard/ledge/internal/store"
	"github.com/spf13/cobra"
)

var (
	eventList    bool
	eventDismiss string
	eventSchema  string
)

var eventCmd = &cobra.Command{
	Use:   "event [JSON]",
	Short: "Send an event, or list, dismiss or describe them",
	Long: `Send JSON to ledge: an event, or a section ({"source": ..., "items": [...]})
that replaces all of a source's events. Without an argument, the JSON is read from
stdin. Prints the id to update or dismiss it by (one per item for a section).

What can be sent is defined by JSON Schemas; print one with --schema.`,
	Example: `  ledge event '{"title": "Deploy finished", "priority": "high", "expires": "4h"}'
  ledge event '{"kind": "timer", "id": "aws-auth", "title": "AWS", "end": "1h", "color": "green",
                "command": "aws sso login",
                "schedule": [{"at": "50m", "color": "yellow"}, {"at": "1h", "color": "red"}]}'
  some-feed | ledge event
  ledge event --list
  ledge event --dismiss aws-auth
  ledge event --schema event`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if (eventList || eventDismiss != "" || eventSchema != "") && len(args) > 0 {
			return fmt.Errorf("event: JSON can't be combined with --list, --dismiss or --schema")
		}
		if eventSchema != "" {
			return printSchema(cmd.OutOrStdout(), eventSchema)
		}
		dir, err := state.Dir()
		if err != nil {
			return err
		}
		client := api.Client{Socket: state.Socket(dir)}
		switch {
		case eventList:
			var reply struct {
				Events []any `json:"events"`
			}
			if err := client.Call("GET", "/events", nil, &reply); err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), reply.Events)
		case eventDismiss != "":
			var reply struct{}
			return client.Call("DELETE", "/events/"+url.PathEscape(eventDismiss), nil, &reply)
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

func printSchema(out io.Writer, name string) error {
	source, ok := store.Schemas.Source(name)
	if !ok {
		return fmt.Errorf("no schema %q; there are %s", name, strings.Join(store.Schemas.Names(), ", "))
	}
	_, err := out.Write(source)
	return err
}

func printJSON(out io.Writer, value any) error {
	data, err := store.MarshalIndent(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func init() {
	eventCmd.Flags().BoolVar(&eventList, "list", false, "print the current events as JSON")
	eventCmd.Flags().StringVar(&eventDismiss, "dismiss", "", "remove the event with this `ID`")
	eventCmd.Flags().StringVar(&eventSchema, "schema", "",
		"print the JSON schema called `NAME` ("+strings.Join(store.Schemas.Names(), ", ")+")")
	eventCmd.MarkFlagsMutuallyExclusive("list", "dismiss", "schema")
	eventCmd.RegisterFlagCompletionFunc("schema", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return slices.Clone(store.Schemas.Names()), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(eventCmd)
}
