package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Download or delete the icons and fonts themes name by url",
	Long: `A theme can name icons and fonts it can't ship itself by their https urls. Each user
downloads them, with ` + "`ledge theme cache`" + `, into ` + theme.Cache + `, and a theme
reads them from there as it loads. Until then, a button shows its text instead of its
icon, and text uses the theme's fallback font.`,
}

var themeCacheCmd = &cobra.Command{
	Use:   "cache [THEME ...]",
	Short: "Download the icons and fonts of themes",
	Long: `Download the icons and fonts the themes named (` + strings.Join(theme.Builtins.Names(), ", ") + `) give by url, or
every theme's with none named. A file already downloaded is left as it is, and one
theme's failing doesn't stop the rest. A running ledge loaded its theme when it
started, so ` + "`ledge restart`" + ` to use what's downloaded.`,
	Example:   `  ledge theme cache default`,
	ValidArgs: theme.Builtins.Names(),
	RunE: func(cmd *cobra.Command, args []string) error {
		names := args
		if len(names) == 0 {
			names = theme.Builtins.Names()
		}
		for _, name := range names {
			if !slices.Contains(theme.Builtins.Names(), name) {
				return fmt.Errorf("no theme %q; there are %s", name, strings.Join(theme.Builtins.Names(), ", "))
			}
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
		defer cancel()
		client := &http.Client{Timeout: 20 * time.Second}
		var errs []error
		for _, name := range names {
			downloaded, err := theme.Builtins.Download(ctx, client, name)
			for _, url := range downloaded {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: downloaded %s\n", name, url)
			}
			if err != nil {
				errs = append(errs, err)
			} else if len(downloaded) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: nothing to download\n", name)
			}
		}
		return errors.Join(errs...)
	},
}

var themeClearCacheCmd = &cobra.Command{
	Use:   "clear-cache",
	Short: "Delete every theme's downloaded icons and fonts",
	Long: `Delete everything ` + "`ledge theme cache`" + ` downloaded, for every theme. Themes then show
their buttons' text and their fallback fonts until it downloads them again; a
running ledge keeps what it loaded until ` + "`ledge restart`" + `.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cleared, err := theme.ClearCache()
		if err != nil {
			return err
		}
		if !cleared {
			fmt.Fprintln(cmd.OutOrStdout(), "nothing to clear")
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "cleared", theme.Cache)
		return nil
	},
}

func init() {
	themeCmd.AddCommand(themeCacheCmd, themeClearCacheCmd)
	rootCmd.AddCommand(themeCmd)
}
