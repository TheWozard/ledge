package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ledge/internal/version"
)

var rootCmd = &cobra.Command{
	Use:   "ledge",
	Short: "An always-on-top desktop bar fed through a local JSON API",
	Long: `ledge shows events and status pills in an always-on-top bar along one edge of the
screen. Scripts, hooks and CI push them to a JSON API on 127.0.0.1, which checks
everything it's sent against JSON Schemas.`,
	Version:       version.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ledge:", err)
		os.Exit(1)
	}
}
