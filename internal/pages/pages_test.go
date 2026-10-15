package pages

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

// The config schema allows exactly the themes there are.
func TestThemesMatchConfigSchema(t *testing.T) {
	data, err := os.ReadFile("../config/config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Theme struct {
				Enum []string `json:"enum"`
			} `json:"theme"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	allowed := slices.Sorted(slices.Values(schema.Properties.Theme.Enum))
	names := slices.Sorted(slices.Values(Themes))
	if !slices.Equal(allowed, names) {
		t.Errorf("config.schema.json allows themes %v, but there are %v", allowed, names)
	}
}

// Every theme's two stylesheets load into their pages with fonts inlined, and every
// file in themes/ is used.
func TestThemes(t *testing.T) {
	used := map[string]bool{}
	for _, name := range Themes {
		for suffix, page := range map[string]func() string{
			"css":        func() string { return Bar(name) },
			"report.css": func() string { page, _ := Report(name, "Title", []any{}); return page },
		} {
			css, err := Theme(name, suffix)
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			file := name + "." + suffix
			used[file] = true
			if strings.Contains(css, "url(fonts/") {
				t.Errorf("%s: a font wasn't inlined", file)
			}
			// Each page's base style uses one variable for the colour behind everything.
			if variable := map[string]string{"css": "--bar:", "report.css": "--page:"}[suffix]; !strings.Contains(css, variable) {
				t.Errorf("%s doesn't set %s", file, variable)
			}
			if !strings.Contains(page(), css+"</style>\n</head>") {
				t.Errorf("%s isn't at the end of its page's <head>", file)
			}
			data, _ := themeFiles.ReadFile(path.Join("themes", file))
			for _, match := range fontURL.FindAllStringSubmatch(string(data), -1) {
				used["fonts/"+match[1]] = true
			}
		}
	}
	fs.WalkDir(themeFiles, "themes", func(file string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !used[strings.TrimPrefix(file, "themes/")] {
			t.Errorf("%s isn't used by any theme", file)
		}
		return err
	})
	if _, err := Theme("neon", "css"); err == nil {
		t.Error("an unknown theme should be an error")
	}
}

// The report's data can't break out of its <script>, whatever its text.
func TestReportData(t *testing.T) {
	page, err := Report("default", "</script><script>alert(1)</script>", []any{map[string]any{"type": "text", "text": "<!-- & -->"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(page, "</script>") != strings.Count(report, "</script>")+1 {
		t.Error("the report's text closed a <script>")
	}
	if strings.Contains(page, "<!-- report -->") || strings.Contains(page, "<!-- & -->") {
		t.Error("the report's data wasn't put in, or was put in unescaped")
	}
}

// Every theme with a frame is a theme, and its appearance is light, dark or unset.
func TestFrames(t *testing.T) {
	for theme, frame := range Frames {
		if !slices.Contains(Themes, theme) || !slices.Contains([]string{"", "light", "dark"}, frame.Appearance) {
			t.Errorf("Frames[%q] = %+v", theme, frame)
		}
	}
}
