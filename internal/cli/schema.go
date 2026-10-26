package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/TheWozard/ledge/internal/store"
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema NAME",
	Short: "Print a JSON schema of what can be sent",
	Long: `Print the JSON schema called NAME: ` + strings.Join(store.Schemas.Names(), " or ") + `. Every event sent is
checked against the event schema, and every section against the section schema.`,
	Example:   `  ledge schema event`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: slices.Clone(store.Schemas.Names()),
	RunE: func(cmd *cobra.Command, args []string) error {
		source, ok := store.Schemas.Source(args[0])
		if !ok {
			return fmt.Errorf("no schema %q; there are %s", args[0], strings.Join(store.Schemas.Names(), ", "))
		}
		_, err := cmd.OutOrStdout().Write(source)
		return err
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
