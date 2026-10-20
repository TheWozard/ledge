package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/TheWozard/ledge/internal/update"
	"github.com/TheWozard/ledge/internal/version"
	"github.com/spf13/cobra"
)

var updateCheck bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Replace this ledge with the latest release",
	Long: `Download the latest release from GitHub, check it against the release's checksums,
and replace this binary with it. A ledge that's running keeps the old version until
it's stopped and run again.

--check only says whether there's a newer version.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if version.Version == "dev" {
			return errors.New("update: this is a development build; install a release with scripts/install.sh")
		}
		client := update.NewClient()
		release, err := client.Latest()
		if err != nil {
			return err
		}
		newer, err := update.Newer(release.Tag, version.Tag())
		if err != nil {
			return err
		}
		if !newer {
			fmt.Printf("ledge %s is up to date.\n", version.Tag())
			return nil
		}
		fmt.Printf("ledge %s is installed; %s is the latest.\n", version.Tag(), release.Tag)
		if updateCheck {
			fmt.Println("Run `ledge update` to install it.")
			return nil
		}
		binary, err := client.Download(release)
		if err != nil {
			return err
		}
		path, err := os.Executable()
		if err != nil {
			return err
		}
		if err := update.Replace(path, binary); err != nil {
			return fmt.Errorf("installing %s: %w", release.Tag, err)
		}
		fmt.Printf("Updated to %s. A ledge that's running keeps %s until it's restarted (ledge restart).\n",
			release.Tag, version.Tag())
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "only say whether there's a newer version")
	rootCmd.AddCommand(updateCmd)
}
