// Package pages holds the HTML pages ledge shows, built into the binary. They decide
// how everything looks; Go passes data in and takes back actions that carry ids only.
package pages

import (
	"embed"
	"encoding/base64"
	"fmt"
	"path"
	"regexp"
	"strings"
)

//go:embed bar.html
var bar string

//go:embed themes/*.css themes/fonts/*.woff2
var themeFiles embed.FS

// Themes maps each theme's name to the stylesheets in themes/ that make it, in order.
// config.schema.json lists the same names.
var Themes = map[string][]string{
	"default":   {"default.css"},
	"platinum":  {"platinum.css"},
	"win95":     {"win95.css"},
	"handheld":  {"handheld.css"},
	"synthwave": {"synthwave.css"},
	"hifi":      {"hifi.css"},
}

// Bar is the bar's page in theme, one of Themes. It panics on an unknown theme: the
// config schema only allows the names in Themes.
func Bar(theme string) string {
	css, err := Theme(theme)
	if err != nil {
		panic(err)
	}
	return strings.Replace(bar, "</head>", "<style>\n"+css+"</style>\n</head>", 1)
}

// fontURL is how a stylesheet names one of themes/fonts: url(fonts/<file>).
var fontURL = regexp.MustCompile(`url\(fonts/([a-z0-9-]+\.woff2?)\)`)

// Theme is theme's stylesheets joined, with their fonts inlined as data: urls, since
// the bar's page is loaded from a string and can't fetch files.
func Theme(theme string) (string, error) {
	files, ok := Themes[theme]
	if !ok {
		return "", fmt.Errorf("no theme %q", theme)
	}
	var css strings.Builder
	for _, name := range files {
		data, err := themeFiles.ReadFile(path.Join("themes", name))
		if err != nil {
			return "", fmt.Errorf("theme %s: %w", theme, err)
		}
		css.Write(data)
	}
	var missing error
	inlined := fontURL.ReplaceAllStringFunc(css.String(), func(url string) string {
		file := fontURL.FindStringSubmatch(url)[1]
		data, err := themeFiles.ReadFile(path.Join("themes/fonts", file))
		if err != nil {
			missing = fmt.Errorf("theme %s: %w", theme, err)
			return url
		}
		kind := strings.TrimPrefix(path.Ext(file), ".")
		return "url(data:font/" + kind + ";base64," + base64.StdEncoding.EncodeToString(data) + ")"
	})
	return inlined, missing
}
