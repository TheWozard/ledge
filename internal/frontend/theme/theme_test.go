package theme

import (
	"encoding/base64"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"
)

// Every built-in theme loads from its manifest, with its fonts and icons inlined, and every file
// in themes/ is used. Each stylesheet starts with a comment naming its theme, which
// the window tests wait for to know a theme has loaded.
func TestBuiltin(t *testing.T) {
	used := map[string]bool{}
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
		for _, icon := range []string{manifest.Icons.Menu, manifest.Icons.Dismiss, manifest.Icons.Copy, manifest.Icons.Run, manifest.Icons.Stop} {
			if icon != "" {
				used[path.Join(path.Dir(file), icon)] = true
			}
		}
		for _, page := range []struct {
			files []string
			css   string
		}{{manifest.Bar, theme.Bar}, {manifest.Report, theme.Report}, {manifest.Menu, theme.Menu}} {
			if len(page.files) == 0 {
				continue // a menu is optional
			}
			if fontURL.MatchString(page.css) {
				t.Errorf("%s: a font wasn't inlined", name)
			}
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
}

// A theme loads from its manifest in any files: each page's stylesheets joined in
// order, paths relative to the manifest, fonts relative to their stylesheet. Anything
// missing or wrong is an error. Icons are SVGs, relative to the manifest, inlined.
func TestLoad(t *testing.T) {
	file := func(data string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(data)} }
	svg := `<?xml version="1.0"?><!-- an icon --><svg viewBox="0 0 16 16"><path d="M0 0h16v16z"/></svg>`
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
		"icons/x.svg":        file(svg),
		"icons/icons.json":   file(`{"name": "icons", "icons": {"dismiss": "x.svg", "copy": "x.svg"}}`),
		"noicon.json":        file(`{"name": "x", "icons": {"menu": "icons/gone.svg"}}`),
		"notsvg.json":        file(`{"name": "x", "icons": {"menu": "a.css"}}`),
		"unknownicon.json":   file(`{"name": "x", "icons": {"close": "icons/x.svg"}}`),
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
	inlined := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
	want = Theme{Name: "icons", Icons: Icons{Dismiss: inlined, Copy: inlined}}
	if theme, err := Load(fsys, "icons/icons.json"); err != nil || theme != want {
		t.Errorf("Load(icons) = %+v, %v; want %+v", theme, err, want)
	}
	for _, name := range []string{"noname", "nofile", "nofont", "outside", "badappearance", "unknown", "two", "missing", "noicon", "notsvg", "unknownicon"} {
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
// with its type; a missing font is an error, and other urls, a font by https url not
// yet downloaded's included, are left alone.
func TestInlineFonts(t *testing.T) {
	withCache(t)
	fonts := fstest.MapFS{"fonts/silkscreen.woff2": &fstest.MapFile{Data: []byte("font")}}
	for _, css := range []string{
		`src: url(fonts/silkscreen.woff2)`,
		`src: url("fonts/silkscreen.woff2")`,
		`src: url( 'fonts/silkscreen.woff2' )`,
	} {
		got, err := InlineFonts(css, fonts)
		if err != nil || !strings.HasPrefix(got, "src: url(data:font/woff2;base64,") || strings.Contains(got, "fonts/") {
			t.Errorf("InlineFonts(%s) = %.60s…, %v", css, got, err)
		}
	}
	for _, css := range []string{`url(https://example.com/a.woff2)`, `url(/fonts/silkscreen.woff2)`, `url(data:font/woff2;base64,AA==)`, `url(a.png)`} {
		if got, err := InlineFonts(css, fonts); got != css || err != nil {
			t.Errorf("InlineFonts(%s) = %s, %v", css, got, err)
		}
	}
	if _, err := InlineFonts(`url(fonts/missing.woff2)`, fonts); err == nil {
		t.Error("a missing font should be an error")
	}
}

// An icon without a viewBox, as Boxicons 3's are, gets one from its width and height,
// so it scales; one with a viewBox, or without both sizes in px, is as it is.
func TestWithViewBox(t *testing.T) {
	for in, want := range map[string]string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><path/></svg>`: `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" width="24" height="24"><path/></svg>`,
		`<?xml version="1.0"?><!-- <svg --><svg width="16px" height="12px"></svg>`:     `<?xml version="1.0"?><!-- <svg --><svg viewBox="0 0 16 12" width="16px" height="12px"></svg>`,
		`<svg width="24" height="24" viewBox="0 0 12 12"></svg>`:                       `<svg width="24" height="24" viewBox="0 0 12 12"></svg>`,
		`<svg width="100%" height="24"></svg>`:                                         `<svg width="100%" height="24"></svg>`,
		`<svg width="24"></svg>`:                                                       `<svg width="24"></svg>`,
		`<svg width="24" height="24" onload="x" a="&quot;"></svg>`:                     `<svg viewBox="0 0 24 24" width="24" height="24" onload="x" a="&quot;"></svg>`,
	} {
		if got := string(withViewBox([]byte(in))); got != want {
			t.Errorf("withViewBox(%s) = %s, want %s", in, got, want)
		}
	}
}
