// Command clear-cache deletes everything cache-theme downloaded, for every theme,
// from theme.Cache. Each theme then shows its icons' text and its fonts' fallbacks
// until cache-theme downloads them again.
//
//	go run ./cmd/clear-cache
package main

import (
	"fmt"
	"os"

	"github.com/TheWozard/ledge-frontend/theme"
)

func main() {
	cleared, err := theme.ClearCache()
	switch {
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	case cleared:
		fmt.Println("cleared", theme.Cache)
	default:
		fmt.Println("nothing to clear")
	}
}
