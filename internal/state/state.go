// Package state locates ledge's state directory, and the socket its API listens on.
package state

import (
	"fmt"
	"os"
	"path/filepath"
)

// Dir is where events, the socket, the pid, saved settings and logs live:
// ${XDG_STATE_HOME:-~/.local/state}/ledge.
func Dir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding the state directory: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "ledge"), nil
}

// Socket is the API's Unix socket in dir, the state directory.
func Socket(dir string) string {
	return filepath.Join(dir, "ledge.sock")
}

// Make creates dir, the state directory, open only to your user. One made before
// ledge did this is closed up too: events can hold reports and commands.
func Make(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}
