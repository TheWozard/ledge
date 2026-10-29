// Package theme is how the bar and report windows look: a Theme is a stylesheet for
// the bar and one for the report window, with their fonts inlined, and how the report
// window's frame looks. Load makes one from a Manifest and the files it names, in any
// fs.FS such as a directory of the app's or its user's; Builtin is one of the themes
// built in.
package theme

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
)

//go:embed themes
var files embed.FS

// Files is the built-in themes: a manifest for each, and the files they name.
var Files, _ = fs.Sub(files, "themes")

// Builtins is the built-in themes. They're built in, so a mistake in them panics here.
var Builtins = func() *Set {
	set, err := ReadSet(Files)
	if err != nil {
		panic(err)
	}
	return set
}()

// Theme is a theme, ready to use. Its stylesheets go on top of the pages' layout
// styles; the comments at the top of those (in components) say what they can hook
// into. A page can't fetch files, so any font a stylesheet uses must be in it, as a
// data: url; Load and InlineFonts do that.
type Theme struct {
	Name   string // for errors and logs
	Bar    string // the bar's stylesheet
	Report string // the report window's stylesheet
	// Menu is the stylesheet of a card's menu of actions, drawn as a page in a pop-up
	// of its own; "" for the native macOS menu instead.
	Menu  string
	Frame Frame
	Icons Icons
}

// Icons is a theme's pictures for the bar's and report's buttons, in place of their
// text, such as icons from a set like Font Awesome or Boxicons. In a Manifest each is
// the path of an SVG file, relative to it; in a Theme, that file as a data: url. An
// icon left out is "", and its button shows its text. An icon is drawn as a mask in
// its button's colour, so only its shape counts, not its own colours.
type Icons struct {
	Menu    string `json:"menu,omitempty"`    // a card's ⋯, for its menu
	Dismiss string `json:"dismiss,omitempty"` // a card's ×
	Copy    string `json:"copy,omitempty"`    // a report's Code block's Copy
}

// Frame is how a theme's report window frame looks. Its page draws the title bar.
type Frame struct {
	// Appearance is "light" or "dark" for the window's own buttons and scroll bars, or
	// "" to follow the system, as the default theme's colours do.
	Appearance string `json:"appearance,omitempty"`
	// Frameless is true for a theme that draws the whole window: the window has no
	// macOS frame, so square corners and no buttons of its own, and the theme's report
	// stylesheet shows the page's close, minimise and zoom buttons and its border.
	Frameless bool `json:"frameless,omitempty"`
}

// Manifest is a theme's json file: its name, the stylesheets for each page, joined
// in order, and its Frame. A stylesheet's path is relative to the manifest, so files
// such as a font's @font-face rules can be shared between pages and themes:
//
//	{
//	  "name": "neon",
//	  "bar": ["css/neon-font.css", "css/neon.css"],
//	  "report": ["css/neon-font.css", "css/neon.report.css"],
//	  "menu": ["css/neon-font.css", "css/neon.menu.css"],
//	  "frame": {"appearance": "dark"},
//	  "icons": {"menu": "icons/ellipsis.svg", "dismiss": "icons/xmark.svg", "copy": "icons/copy.svg"}
//	}
//
// "menu" is optional: without it, a card's menu is the native one. So are "icons",
// and each of them; see Icons. One theme in a Set can say it's the Default, the one
// to use when none is chosen.
type Manifest struct {
	Name    string   `json:"name"`
	Default bool     `json:"default,omitempty"`
	Bar     []string `json:"bar"`
	Report  []string `json:"report"`
	Menu    []string `json:"menu,omitempty"`
	Frame   Frame    `json:"frame"`
	Icons   Icons    `json:"icons"`
}

// Load reads the theme whose manifest is file in fsys: each page's stylesheets, joined
// in order, with every font they name by a relative url, such as
// url(../fonts/neon.woff2), read from fsys and inlined, and its icons, inlined too.
func Load(fsys fs.FS, file string) (Theme, error) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return Theme{}, fmt.Errorf("theme: %w", err)
	}
	manifest, err := readManifest(data)
	if err != nil {
		return Theme{}, fmt.Errorf("theme %s: %w", file, err)
	}
	return load(fsys, file, manifest)
}

// load is the theme manifest describes, read from file in fsys.
func load(fsys fs.FS, file string, manifest Manifest) (Theme, error) {
	t := Theme{Name: manifest.Name, Frame: manifest.Frame}
	dir := path.Dir(file)
	var err error
	if t.Bar, err = joinStylesheets(fsys, dir, manifest.Bar); err != nil {
		return t, fmt.Errorf("theme %s: %w", t.Name, err)
	}
	if t.Report, err = joinStylesheets(fsys, dir, manifest.Report); err != nil {
		return t, fmt.Errorf("theme %s: %w", t.Name, err)
	}
	if t.Menu, err = joinStylesheets(fsys, dir, manifest.Menu); err != nil {
		return t, fmt.Errorf("theme %s: %w", t.Name, err)
	}
	t.Icons = manifest.Icons
	for _, icon := range []*string{&t.Icons.Menu, &t.Icons.Dismiss, &t.Icons.Copy} {
		if *icon == "" {
			continue
		}
		if *icon, err = loadIcon(fsys, path.Join(dir, *icon)); err != nil {
			return t, fmt.Errorf("theme %s: %w", t.Name, err)
		}
	}
	return t, nil
}

// loadIcon is the SVG file in fsys as a data: url.
func loadIcon(fsys fs.FS, file string) (string, error) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return "", err
	}
	if !isSVG(data) {
		return "", fmt.Errorf("%s isn't an SVG", file)
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(data), nil
}

// isSVG is whether data is XML whose first element is an <svg>.
func isSVG(data []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		if start, ok := token.(xml.StartElement); ok {
			return start.Name.Local == "svg"
		}
	}
}

// joinStylesheets is the stylesheets files, relative to dir in fsys, each with its
// fonts inlined, joined in order.
func joinStylesheets(fsys fs.FS, dir string, files []string) (string, error) {
	var css []string
	for _, name := range files {
		sheet, err := loadStylesheet(fsys, path.Join(dir, name))
		if err != nil {
			return "", err
		}
		css = append(css, sheet)
	}
	return strings.Join(css, "\n"), nil
}

// loadStylesheet is the stylesheet file in fsys with its fonts inlined.
func loadStylesheet(fsys fs.FS, file string) (string, error) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return "", err
	}
	css, err := inlineFonts(string(data), fsys, path.Dir(file))
	if err != nil {
		return "", fmt.Errorf("%s: %w", file, err)
	}
	return css, nil
}

// readManifest decodes a Manifest, refusing anything it doesn't know and one with no name.
func readManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	switch {
	case decoder.More():
		return manifest, errors.New("more than one value")
	case manifest.Name == "":
		return manifest, errors.New("no name")
	case !slices.Contains([]string{"", "light", "dark"}, manifest.Frame.Appearance):
		return manifest, fmt.Errorf("appearance %q isn't light, dark or unset", manifest.Frame.Appearance)
	}
	return manifest, nil
}

// Set is the themes in an fs.FS: every .json file in it, at any depth, is a theme's
// Manifest. Their names are unique, and at most one is the default.
type Set struct {
	fsys      fs.FS
	names     []string            // the default first, then the rest in order
	manifests map[string]Manifest // by name
	files     map[string]string   // each manifest's file, by name
	themes    sync.Map            // each theme Theme has loaded, by name
	def       string
}

// ReadSet reads the manifests in fsys. Their stylesheets and fonts are read when a
// theme is first asked for, and kept.
func ReadSet(fsys fs.FS) (*Set, error) {
	s := &Set{fsys: fsys, manifests: map[string]Manifest{}, files: map[string]string{}}
	err := fs.WalkDir(fsys, ".", func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || path.Ext(file) != ".json" {
			return err
		}
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return err
		}
		manifest, err := readManifest(data)
		if err != nil {
			return fmt.Errorf("theme %s: %w", file, err)
		}
		if other, ok := s.files[manifest.Name]; ok {
			return fmt.Errorf("themes %s and %s are both named %q", other, file, manifest.Name)
		}
		if manifest.Default {
			if s.def != "" {
				return fmt.Errorf("themes %s and %s are both the default", s.files[s.def], file)
			}
			s.def = manifest.Name
		}
		s.manifests[manifest.Name], s.files[manifest.Name] = manifest, file
		s.names = append(s.names, manifest.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(s.names, func(a, b string) int {
		switch {
		case a == s.def:
			return -1
		case b == s.def:
			return 1
		}
		return strings.Compare(a, b)
	})
	return s, nil
}

// Names is the names of the themes, the default first and the rest in order.
func (s *Set) Names() []string { return slices.Clone(s.names) }

// Default is the name of the theme whose manifest says it's the default, or "" if none does.
func (s *Set) Default() string { return s.def }

// Theme is the theme name.
func (s *Set) Theme(name string) (Theme, error) {
	if t, ok := s.themes.Load(name); ok {
		return t.(Theme), nil
	}
	manifest, ok := s.manifests[name]
	if !ok {
		return Theme{}, fmt.Errorf("no theme %q", name)
	}
	t, err := load(s.fsys, s.files[name], manifest)
	if err != nil {
		return t, err
	}
	s.themes.Store(name, t)
	return t, nil
}

// Builtin is the built-in theme name, one of Builtins.Names().
func Builtin(name string) (Theme, error) { return Builtins.Theme(name) }

// fontURL is how a stylesheet names a font file: url(<path>), its url quoted or not,
// for a relative path (no scheme, not starting with /) to a woff2, woff, ttf or otf.
var fontURL = regexp.MustCompile(`url\(\s*["']?([^"'()\s:/][^"'()\s:]*\.(woff2|woff|ttf|otf))["']?\s*\)`)

// fontTypes is the media type of each kind of font file fontURL matches.
var fontTypes = map[string]string{"woff2": "font/woff2", "woff": "font/woff", "ttf": "font/ttf", "otf": "font/otf"}

// InlineFonts replaces each font url in css with the file it names in fonts, as a
// data: url; a path is relative to fonts, as it would be to the stylesheet's own
// directory. A font that isn't in fonts is an error.
func InlineFonts(css string, fonts fs.FS) (string, error) {
	return inlineFonts(css, fonts, ".")
}

// inlineFonts is InlineFonts with paths relative to dir in fsys, so they can go up
// out of it with .. but not out of fsys.
func inlineFonts(css string, fsys fs.FS, dir string) (string, error) {
	var missing error
	inlined := fontURL.ReplaceAllStringFunc(css, func(url string) string {
		match := fontURL.FindStringSubmatch(url)
		data, err := fs.ReadFile(fsys, path.Join(dir, match[1]))
		if err != nil {
			missing = err
			return url
		}
		return "url(data:" + fontTypes[match[2]] + ";base64," + base64.StdEncoding.EncodeToString(data) + ")"
	})
	return inlined, missing
}
