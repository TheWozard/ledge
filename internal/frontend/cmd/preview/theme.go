package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheWozard/ledge-frontend/theme"
)

// builtinDir is where the built-in themes are, from the repository's root.
const builtinDir = "theme/themes"

// loadTheme is the theme name: a built-in one, or the one whose manifest is the json
// file name. With fromDisk, a built-in one is read from builtinDir when it's there.
func loadTheme(name string, fromDisk bool) (theme.Theme, error) {
	if strings.HasSuffix(name, ".json") {
		// From the root, so its stylesheets and fonts can be anywhere.
		file, err := filepath.Abs(name)
		if err != nil {
			return theme.Theme{}, err
		}
		return theme.Load(os.DirFS("/"), filepath.ToSlash(strings.TrimPrefix(file, "/")))
	}
	set := theme.Builtins
	if _, err := os.Stat(builtinDir); fromDisk && err == nil {
		if set, err = theme.ReadSet(os.DirFS(builtinDir)); err != nil {
			return theme.Theme{}, err
		}
	}
	th, err := set.Theme(name)
	if err != nil {
		err = fmt.Errorf("%w; the themes are %s, or give the path to your own .json", err, strings.Join(set.Names(), ", "))
	}
	return th, err
}
