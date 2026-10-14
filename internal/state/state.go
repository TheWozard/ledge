// Package state locates ledge's state directory and the port its API listens on.
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// DefaultPort is the API port when $LEDGE_PORT isn't set.
const DefaultPort = 47391

// Dir is where events, the pid, saved settings and logs live:
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

// Port is $LEDGE_PORT, or DefaultPort.
func Port() (int, error) {
	text := os.Getenv("LEDGE_PORT")
	if text == "" {
		return DefaultPort, nil
	}
	port, err := strconv.Atoi(text)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("LEDGE_PORT %q isn't a port number", text)
	}
	return port, nil
}
