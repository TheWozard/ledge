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
	var names []string
	for name := range Themes {
		names = append(names, name)
	}
	slices.Sort(names)
	if !slices.Equal(allowed, names) {
		t.Errorf("config.schema.json allows themes %v, but there are %v", allowed, names)
	}
}

// Every theme loads into the page with its fonts inlined, and every stylesheet and
// font in themes/ is used.
func TestThemes(t *testing.T) {
	used := map[string]bool{}
	for name, files := range Themes {
		css, err := Theme(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if strings.Contains(css, "url(fonts/") {
			t.Errorf("%s: a font wasn't inlined", name)
		}
		if !strings.Contains(css, "--bar:") {
			t.Errorf("%s doesn't set --bar, the colour behind the bar", name)
		}
		if page := Bar(name); !strings.Contains(page, css+"</style>\n</head>") {
			t.Errorf("%s: the theme isn't at the end of the page's <head>", name)
		}
		for _, file := range files {
			used[file] = true
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
	if _, err := Theme("neon"); err == nil {
		t.Error("an unknown theme should be an error")
	}
}
