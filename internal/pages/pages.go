// Package pages holds the HTML pages ledge shows, built into the binary. They decide
// how everything looks; Go passes data in and takes back actions that carry ids only.
package pages

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

//go:embed bar.html
var bar string

//go:embed report.html
var report string

//go:embed themes/*.css themes/fonts/*.woff2
var themeFiles embed.FS

// Themes is the names of the themes. Each has two stylesheets in themes/: <name>.css
// for the bar and <name>.report.css for the report window. config.schema.json lists
// the same names.
var Themes = []string{"default", "platinum", "win95", "handheld", "synthwave", "hifi"}

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

// Bar is the bar's page in theme, one of Themes. It panics on an unknown theme: the
// config schema only allows the names in Themes.
func Bar(theme string) string {
	return withTheme(bar, theme, "css")
}

// Report is the report window's page in theme, showing title and blocks: an event's
// report, as checked by event.schema.json. The page draws the blocks itself, as text.
func Report(theme, title string, blocks any) (string, error) {
	// json.Marshal escapes <, > and &, so nothing in the data can end the <script>.
	data, err := json.Marshal(map[string]any{"title": title, "blocks": blocks})
	if err != nil {
		return "", err
	}
	page := withTheme(report, theme, "report.css")
	return strings.Replace(page, "<!-- report -->", `<script id="report" type="application/json">`+string(data)+`</script>`, 1), nil
}

// withTheme puts theme's stylesheet with this suffix at the end of page's <head>.
func withTheme(page, theme, suffix string) string {
	css, err := Theme(theme, suffix)
	if err != nil {
		panic(err)
	}
	return strings.Replace(page, "</head>", "<style>\n"+css+"</style>\n</head>", 1)
}

// fontURL is how a stylesheet names one of themes/fonts: url(fonts/<file>).
var fontURL = regexp.MustCompile(`url\(fonts/([a-z0-9-]+\.woff2)\)`)

// Theme is theme's stylesheet with this suffix ("css" for the bar, "report.css" for
// the report window), with its fonts inlined as data: urls, since the pages are
// loaded from strings and can't fetch files.
func Theme(theme, suffix string) (string, error) {
	if !slices.Contains(Themes, theme) {
		return "", fmt.Errorf("no theme %q", theme)
	}
	css, err := themeFiles.ReadFile(path.Join("themes", theme+"."+suffix))
	if err != nil {
		return "", fmt.Errorf("theme %s: %w", theme, err)
	}
	var missing error
	inlined := fontURL.ReplaceAllStringFunc(string(css), func(url string) string {
		file := fontURL.FindStringSubmatch(url)[1]
		data, err := themeFiles.ReadFile(path.Join("themes/fonts", file))
		if err != nil {
			missing = fmt.Errorf("theme %s: %w", theme, err)
			return url
		}
		return "url(data:font/woff2;base64," + base64.StdEncoding.EncodeToString(data) + ")"
	})
	return inlined, missing
}
