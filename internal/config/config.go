// Package config reads the window settings passed with --config. There's no default
// location: without a file, ledge uses the defaults. The file is checked against
// config.schema.json, so an unknown key or a bad value stops ledge from starting
// rather than being quietly ignored.
package config

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/TheWozard/ledge/internal/schema"
)

//go:embed *.schema.json
var schemaFiles embed.FS

var schemas = func() *schema.Set {
	set, err := schema.Load(schemaFiles)
	if err != nil {
		panic(err)
	}
	return set
}()

// Config is the window settings.
type Config struct {
	Screen int    `json:"screen"`
	Edge   string `json:"edge"`
	Height int    `json:"height"`
	Theme  string `json:"theme"`
	Level  string `json:"level"`
}

// Defaults is the config without a file, and what a file's missing keys fall back to.
var Defaults = Config{Screen: 0, Edge: "bottom", Height: 110, Theme: "default", Level: "normal"}

// Load reads the config at path, or returns Defaults if path is "".
func Load(path string) (Config, error) {
	config := Defaults
	if path == "" {
		return config, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("config: %w", err)
	}
	value, err := schema.Decode(data)
	if err != nil {
		return config, fmt.Errorf("config %s isn't valid JSON: %w", path, err)
	}
	if err := schemas.Validate("config", value); err != nil {
		var invalid *schema.Error
		if errors.As(err, &invalid) && len(invalid.Path) == 0 {
			return config, fmt.Errorf("config %s: %s", path, invalid.Message)
		}
		return config, fmt.Errorf("config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("config %s: %w", path, err)
	}
	return config, nil
}
