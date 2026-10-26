package cli

import (
	"fmt"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/TheWozard/ledge/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ledge version, and the running ledge's",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ledge %s (%s, built %s)\n", version.Tag(), version.Commit, version.Date)
		dir, err := state.Dir()
		if err != nil {
			return
		}
		var status api.Status
		if (api.Client{Socket: state.Socket(dir)}).Call("GET", "/status", nil, &status) == nil {
			fmt.Printf("running: ledge %s (pid %d, started %s)\n",
				status.Version, status.PID, status.Started.Local().Format("2006-01-02 15:04:05"))
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
