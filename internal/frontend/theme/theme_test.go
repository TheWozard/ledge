package theme

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
)

// Every theme's two stylesheets load with their fonts inlined, and every file in
// themes/ is used.
func TestCSS(t *testing.T) {
	used := map[string]bool{}
	for _, name := range Names {
		for page, variable := range map[Page]string{Bar: "--bar:", Report: "--page:"} {
			file := name + "." + string(page)
			css, err := CSS(name, page)
			if err != nil {
				t.Errorf("%s: %v", file, err)
				continue
			}
			used[file] = true
			if strings.Contains(css, "url(fonts/") {
				t.Errorf("%s: a font wasn't inlined", file)
			}
			// Each page uses one variable for the opaque colour behind everything.
			if !strings.Contains(css, variable) {
				t.Errorf("%s doesn't set %s", file, variable)
			}
			data, _ := files.ReadFile(path.Join("themes", file))
			for _, match := range fontURL.FindAllStringSubmatch(string(data), -1) {
				used["fonts/"+match[1]] = true
			}
		}
	}
	fs.WalkDir(files, "themes", func(file string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !used[strings.TrimPrefix(file, "themes/")] {
			t.Errorf("%s isn't used by any theme", file)
		}
		return err
	})
	if _, err := CSS("neon", Bar); err == nil {
		t.Error("an unknown theme should be an error")
	}
	if Names[0] != Default {
		t.Errorf("Names starts with %q, not the default", Names[0])
	}
}

// Every theme with a frame is a theme, and its appearance is light, dark or unset.
func TestFrames(t *testing.T) {
	for name, frame := range Frames {
		if !slices.Contains(Names, name) || !slices.Contains([]string{"", "light", "dark"}, frame.Appearance) {
			t.Errorf("Frames[%q] = %+v", name, frame)
		}
	}
}
