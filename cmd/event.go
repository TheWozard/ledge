package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	Short: "Send an event or status, or list, dismiss or describe them",
	Long: `Send JSON to ledge: an event, a status, or a section ({"source": ..., "items": [...]})
that replaces all of a source's events. Without an argument, the JSON is read from
stdin. Prints the id to update or dismiss it by (one per item for a section).

What can be sent is defined by JSON Schemas; print one with --schema.`,
	Example: `  ledge event '{"title": "Deploy finished", "priority": "high", "expires": "4h"}'
  ledge event '{"kind": "status", "id": "aws-auth", "title": "AWS", "color": "green",
                "schedule": [{"at": "50m", "color": "yellow"}, {"at": "1h", "color": "red"}]}'
  some-feed | ledge event
  ledge event --list
  ledge event --dismiss aws-auth
  ledge event --schema status`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if (eventList || eventDismiss != "" || eventSchema != "") && len(args) > 0 {
			return fmt.Errorf("event: JSON can't be combined with --list, --dismiss or --schema")
		}
		if eventSchema != "" {
			return printSchema(eventSchema)
		}
		port, err := state.Port()
		if err != nil {
			return err
		}
		client := api.Client{Port: port}
		switch {
		case eventList:
			var reply struct {
				Events []any `json:"events"`
			}
			if err := client.Call("GET", "/events", nil, &reply); err != nil {
				return err
			}
			return printJSON(reply.Events)
		case eventDismiss != "":
			body, _ := json.Marshal(map[string]string{"id": eventDismiss})
			var reply struct {
				Dismissed bool `json:"dismissed"`
			}
			if err := client.Call("POST", "/dismiss", body, &reply); err != nil {
				return err
			}
			if !reply.Dismissed {
				return fmt.Errorf("no event %q", eventDismiss)
			}
			return nil
		}
		var text []byte
		if len(args) == 1 {
			text = []byte(args[0])
		} else if text, err = io.ReadAll(os.Stdin); err != nil {
			return err
		}
		var parsed any
		if err := json.Unmarshal(text, &parsed); err != nil {
			return fmt.Errorf("not JSON (%v)", err)
		}
		var reply struct {
			ID  string   `json:"id"`
			IDs []string `json:"ids"`
		}
		if err := client.Call("POST", "/event", text, &reply); err != nil {
			return err
		}
		if reply.IDs != nil {
			if len(reply.IDs) > 0 {
				fmt.Println(strings.Join(reply.IDs, "\n"))
			}
		} else {
			fmt.Println(reply.ID)
		}
		return nil
	},
}

func printSchema(name string) error {
	source, ok := store.Schemas.Source(name)
	if !ok {
		return fmt.Errorf("no schema %q; there are %s", name, strings.Join(store.Schemas.Names(), ", "))
	}
	_, err := os.Stdout.Write(source)
	return err
}

func printJSON(value any) error {
	data, err := store.MarshalIndent(value)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func init() {
	eventCmd.Flags().BoolVar(&eventList, "list", false, "print the current events as JSON")
	eventCmd.Flags().StringVar(&eventDismiss, "dismiss", "", "remove the event or status with this `ID`")
	eventCmd.Flags().StringVar(&eventSchema, "schema", "",
		"print the JSON schema called `NAME` ("+strings.Join(store.Schemas.Names(), ", ")+")")
	eventCmd.MarkFlagsMutuallyExclusive("list", "dismiss", "schema")
	eventCmd.RegisterFlagCompletionFunc("schema", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return slices.Clone(store.Schemas.Names()), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(eventCmd)
}
