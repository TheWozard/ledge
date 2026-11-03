// Command cache-theme downloads the files a theme names by url, its icons and its
// stylesheets' fonts, for this user, into theme.Cache, where the theme reads them
// from as it loads. They're files the theme can't ship itself, such as ones whose
// licence doesn't allow it; until they're downloaded, an icon's button shows its
// text and a font's stylesheet uses its next font.
//
//	go run ./cmd/cache-theme [theme ...]
//
// Each theme is a built-in theme's name, or the path to a theme's json manifest of
// your own, such as ~/themes/neon.json; with none, it's every built-in theme. A file
// already downloaded is left as it is. One theme's failing doesn't stop the rest.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheWozard/ledge-frontend/theme"
)

func main() {
	names := os.Args[1:]
	if len(names) == 0 {
		names = theme.Builtins.Names()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Second}
	failed := false
	for _, name := range names {
		downloaded, err := download(ctx, client, name)
		for _, url := range downloaded {
			fmt.Printf("%s: downloaded %s\n", name, url)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		} else if len(downloaded) == 0 {
			fmt.Printf("%s: nothing to download\n", name)
		}
	}
	if failed {
		os.Exit(1)
	}
}

// download downloads the files of name: a built-in theme, or the theme whose
// manifest is the json file name.
func download(ctx context.Context, client *http.Client, name string) ([]string, error) {
	if !strings.HasSuffix(name, ".json") {
		return theme.Builtins.Download(ctx, client, name)
	}
	// From the root, as the preview loads one, so its files can be anywhere.
	file, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return theme.Download(ctx, client, os.DirFS("/"), filepath.ToSlash(strings.TrimPrefix(file, "/")))
}
