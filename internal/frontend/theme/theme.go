// Package theme holds the themes: for each, a stylesheet for the bar and one for the
// report window, the fonts they need, and how its report window's frame looks.
package theme

import (
	"embed"
	"encoding/base64"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sync"
)

//go:embed themes/*.css themes/fonts/*.woff2
var files embed.FS

// Names is the themes, "default" first. Each has two stylesheets in themes/:
// <name>.css for the bar and <name>.report.css for the report window.
var Names = []string{"default", "platinum", "win95", "handheld", "synthwave", "hifi"}

// Default is the theme to use when none is chosen.
const Default = "default"

// Page is which page a stylesheet styles.
type Page string

const (
	Bar    Page = "css"        // the bar
	Report Page = "report.css" // the report window
)

// Frame is how a theme's report window frame looks. Its page draws the title bar.
type Frame struct {
	// Appearance is "light" or "dark" for the window's own buttons and scroll bars, or
	// "" to follow the system, as the default theme's colours do.
	Appearance string
	// Frameless is true for a theme that draws the whole window: the window has no
	// macOS frame, so square corners and no buttons of its own, and the theme's report
	// stylesheet shows the page's close, minimise and zoom buttons and its border.
	Frameless bool
}

// Frames is each theme's report window frame; a theme that isn't here has the zero Frame.
var Frames = map[string]Frame{
	"platinum":  {Appearance: "light", Frameless: true},
	"win95":     {Appearance: "light", Frameless: true},
	"handheld":  {Appearance: "light"},
	"synthwave": {Appearance: "dark"},
	"hifi":      {Appearance: "light"},
}

// fontURL is how a stylesheet names one of themes/fonts: url(fonts/<file>), its url
// quoted or not.
var fontURL = regexp.MustCompile(`url\(\s*["']?fonts/([a-z0-9-]+\.woff2)["']?\s*\)`)

// stylesheets is each stylesheet CSS has made, by theme and page: they're built in,
// so they never change.
var stylesheets sync.Map

// CSS is theme's stylesheet for page, with its fonts inlined as data: urls, since
// ledge's pages are loaded from strings and can't fetch files.
func CSS(name string, page Page) (string, error) {
	if !slices.Contains(Names, name) {
		return "", fmt.Errorf("no theme %q", name)
	}
	file := name + "." + string(page)
	if css, ok := stylesheets.Load(file); ok {
		return css.(string), nil
	}
	css, err := files.ReadFile(path.Join("themes", file))
	if err != nil {
		return "", fmt.Errorf("theme %s: %w", name, err)
	}
	inlined, err := inlineFonts(string(css))
	if err != nil {
		return "", fmt.Errorf("theme %s: %w", name, err)
	}
	stylesheets.Store(file, inlined)
	return inlined, nil
}

// inlineFonts replaces each of css's urls for themes/fonts with the font as a data: url.
func inlineFonts(css string) (string, error) {
	var missing error
	inlined := fontURL.ReplaceAllStringFunc(css, func(url string) string {
		data, err := files.ReadFile(path.Join("themes/fonts", fontURL.FindStringSubmatch(url)[1]))
		if err != nil {
			missing = err
			return url
		}
		return "url(data:font/woff2;base64," + base64.StdEncoding.EncodeToString(data) + ")"
	})
	return inlined, missing
}
