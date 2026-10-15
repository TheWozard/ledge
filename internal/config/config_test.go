package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		`{"theme": "neon"}`:                   "theme: value must be one of",
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
