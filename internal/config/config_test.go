package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/TheWozard/ledge-frontend/theme"
)

func TestLoad(t *testing.T) {
	if config, err := Load(""); err != nil || config != Defaults {
		t.Errorf("no file: %v, %v", config, err)
	}
	dir := t.TempDir()
	for body, want := range map[string]string{
		`{"edge": "top"}`:                     "",
		`{"edgee": "top"}`:                    "additional properties 'edgee' not allowed",
		`{"edge": "left"}`:                    "edge: value must be one of",
		`{"edge": "top", "theme": "default"}`: "",
		`{"theme": "neon"}`:                   "theme: value must be",
		`{"edge": "top", "level": "normal"}`:  "",
		`{"level": "above"}`:                  "level: value must be one of",
		`{"height": 0}`:                       "height: minimum",
		`{"screen": 1.5}`:                     "screen: got number, want integer",
		`[]`:                                  "got array, want object",
		`{"edge": "top"} {"edge": "top"}`:     "isn't valid JSON",
	} {
		path := filepath.Join(dir, "config.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		config, err := Load(path)
		if want == "" {
			if err != nil || config != (Config{Screen: 0, Edge: "top", Height: 110, Theme: "default", Level: "normal"}) {
				t.Errorf("%s: %+v, %v", body, config, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want an error containing %q", body, err, want)
		}
	}
	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("a missing file named with --config should be an error")
	}
}

// The schema allows exactly the frontend's themes.
func TestThemesMatchFrontend(t *testing.T) {
	data, err := schemaFiles.ReadFile("config.schema.json")
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
	if allowed, names := slices.Sorted(slices.Values(schema.Properties.Theme.Enum)), slices.Sorted(slices.Values(theme.Builtins.Names())); !slices.Equal(allowed, names) {
		t.Errorf("config.schema.json allows themes %v, but ledge-frontend has %v", allowed, names)
	}
	if Defaults.Theme != theme.Builtins.Default() {
		t.Errorf("the default theme is %q, but ledge-frontend's is %q", Defaults.Theme, theme.Builtins.Default())
	}
}
