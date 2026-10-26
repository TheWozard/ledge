package cmd

import (
	"fmt"
	"os"

	"github.com/TheWozard/ledge/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ledge",
	Short: "An always-on-top desktop bar fed through a local JSON API",
	Long: `ledge shows events and statuses in an always-on-top bar along one edge of the
screen. Scripts, hooks and CI push them to a JSON API on 127.0.0.1, which checks
everything it's sent against JSON Schemas.`,
	Version:       version.Tag(),
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ledge:", err)
		os.Exit(1)
	}
}
