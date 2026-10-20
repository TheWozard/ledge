package theme

import (
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"
)

// Every built-in theme loads from its manifest, named for it, with its fonts inlined,
// and every file in themes/ is used.
func TestBuiltin(t *testing.T) {
	used := map[string]bool{"fonts/OFL.txt": true}
	for _, name := range Builtins.Names() {
		theme, err := Builtin(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		file := Builtins.files[name]
		used[file] = true
		data, _ := fs.ReadFile(Files, file)
		manifest, _ := readManifest(data)
		for _, page := range []struct {
			files         []string
			css, variable string
		}{{manifest.Bar, theme.Bar, "--bar:"}, {manifest.Report, theme.Report, "--page:"}} {
			if fontURL.MatchString(page.css) {
				t.Errorf("%s: a font wasn't inlined", name)
			}
			// Each page uses one variable for the opaque colour behind everything.
			if !strings.Contains(page.css, page.variable) {
				t.Errorf("%s doesn't set %s", name, page.variable)
			}
			// Each page's stylesheet has a comment naming its theme.
			if !strings.Contains(page.css, "/* "+name+":") {
				t.Errorf("%s: a stylesheet doesn't start /* %s:", name, name)
			}
			for _, file := range page.files {
				used[file] = true
				data, _ := fs.ReadFile(Files, file)
				for _, match := range fontURL.FindAllStringSubmatch(string(data), -1) {
					used[path.Join(path.Dir(file), match[1])] = true
				}
			}
		}
	}
	fs.WalkDir(Files, ".", func(file string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !used[file] {
			t.Errorf("%s isn't used by any theme", file)
		}
		return err
	})
	if _, err := Builtin("neon"); err == nil {
		t.Error("an unknown theme should be an error")
	}
	if Builtins.Default() != "default" || Builtins.Names()[0] != "default" {
		t.Errorf("the default is %q, and Names starts with %q", Builtins.Default(), Builtins.Names()[0])
	}
	if theme, _ := Builtin("win95"); theme.Frame != (Frame{Appearance: "light", Frameless: true}) {
		t.Errorf("win95's frame is %+v", theme.Frame)
	}
}

// A theme loads from its manifest in any files: each page's stylesheets joined in
// order, paths relative to the manifest, fonts relative to their stylesheet. Anything
// missing or wrong is an error.
func TestLoad(t *testing.T) {
	file := func(data string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(data)} }
	fsys := fstest.MapFS{
		"common/fonts.css":   file(`@font-face { src: url("../fonts/neon.ttf") }`),
		"common/base.css":    file(`:root { --page: #000 }`),
		"fonts/neon.ttf":     file("font"),
		"neon/theme.json":    file(`{"name": "Neon", "bar": ["../common/fonts.css", "bar.css"], "report": ["../common/base.css"], "frame": {"appearance": "dark", "frameless": true}}`),
		"neon/bar.css":       file(`:root { --bar: #000 }`),
		"plain.json":         file(`{"name": "plain", "bar": ["a.css"], "report": []}`),
		"a.css":              file(`a {}`),
		"noname.json":        file(`{"bar": ["a.css"]}`),
		"nofile.json":        file(`{"name": "x", "bar": ["gone.css"]}`),
		"nofont.json":        file(`{"name": "x", "bar": ["nofont.css"]}`),
		"nofont.css":         file(`src: url(fonts/gone.woff2)`),
		"outside.json":       file(`{"name": "x", "bar": ["outside.css"]}`),
		"outside.css":        file(`src: url(../fonts/neon.ttf)`),
		"badappearance.json": file(`{"name": "x", "frame": {"appearance": "sepia"}}`),
		"unknown.json":       file(`{"name": "x", "frameless": true}`),
		"two.json":           file(`{"name": "x"} {"name": "y"}`),
	}
	theme, err := Load(fsys, "neon/theme.json")
	if err != nil {
		t.Fatal(err)
	}
	want := Theme{Name: "Neon", Bar: "@font-face { src: url(data:font/ttf;base64,Zm9udA==) }\n:root { --bar: #000 }",
		Report: ":root { --page: #000 }", Frame: Frame{Appearance: "dark", Frameless: true}}
	if theme != want {
		t.Errorf("Load(neon) = %+v, want %+v", theme, want)
	}
	if theme, err := Load(fsys, "plain.json"); err != nil || theme != (Theme{Name: "plain", Bar: "a {}"}) {
		t.Errorf("Load(plain) = %+v, %v", theme, err)
	}
	for _, name := range []string{"noname", "nofile", "nofont", "outside", "badappearance", "unknown", "two", "missing"} {
		if _, err := Load(fsys, name+".json"); err == nil {
			t.Errorf("Load(%s.json) should be an error", name)
		}
	}
}

// A set is every manifest in its files, at any depth, by name, the default first; names
// can't repeat, and only one can be the default.
func TestReadSet(t *testing.T) {
	file := func(data string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(data)} }
	fsys := fstest.MapFS{
		"zebra.json":      file(`{"name": "zebra", "bar": ["a.css"]}`),
		"neon/theme.json": file(`{"name": "neon", "default": true, "bar": ["../a.css"]}`),
		"apple.json":      file(`{"name": "apple"}`),
		"a.css":           file(`a {}`),
		"notes.txt":       file(`not a manifest`),
	}
	set, err := ReadSet(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(set.Names(), " "); got != "neon apple zebra" || set.Default() != "neon" {
		t.Errorf("names %s, default %q", got, set.Default())
	}
	if theme, err := set.Theme("neon"); err != nil || theme.Bar != "a {}" {
		t.Errorf("Theme(neon) = %+v, %v", theme, err)
	}
	if _, err := set.Theme("missing"); err == nil {
		t.Error("an unknown theme should be an error")
	}
	delete(fsys, "neon/theme.json")
	if set, err := ReadSet(fsys); err != nil || set.Default() != "" || set.Names()[0] != "apple" {
		t.Errorf("with no default: %v, %v", set, err)
	}
	for name, extra := range map[string]*fstest.MapFile{
		"same name":   file(`{"name": "apple"}`),
		"two default": file(`{"name": "x", "default": true}`),
		"bad json":    file(`{"name": "x",`),
	} {
		bad := fstest.MapFS{"one.json": file(`{"name": "apple", "default": true}`), "two.json": extra}
		if _, err := ReadSet(bad); err == nil {
			t.Errorf("%s should be an error", name)
		}
	}
}

// A font's url, relative to the files given, is inlined whether it's quoted or not,
// with its type; a missing font is an error, and other urls are left alone.
func TestInlineFonts(t *testing.T) {
	for _, css := range []string{
		`src: url(fonts/silkscreen.woff2)`,
		`src: url("fonts/silkscreen.woff2")`,
		`src: url( 'fonts/silkscreen.woff2' )`,
	} {
		got, err := InlineFonts(css, Files)
		if err != nil || !strings.HasPrefix(got, "src: url(data:font/woff2;base64,") || strings.Contains(got, "fonts/") {
			t.Errorf("InlineFonts(%s) = %.60s…, %v", css, got, err)
		}
	}
	for _, css := range []string{`url(https://example.com/a.woff2)`, `url(/fonts/silkscreen.woff2)`, `url(data:font/woff2;base64,AA==)`, `url(a.png)`} {
		if got, err := InlineFonts(css, Files); got != css || err != nil {
			t.Errorf("InlineFonts(%s) = %s, %v", css, got, err)
		}
	}
	if _, err := InlineFonts(`url(fonts/missing.woff2)`, Files); err == nil {
		t.Error("a missing font should be an error")
	}
}
