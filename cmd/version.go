package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"ledge/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ledge version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ledge %s (%s, built %s)\n", version.Tag(), version.Commit, version.Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
