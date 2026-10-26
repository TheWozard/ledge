package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMake(t *testing.T) {
	// One made before ledge closed it up, and one that doesn't exist yet.
	open := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{open, filepath.Join(t.TempDir(), "new", "ledge")} {
		if err := Make(dir); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("%s: mode %v, %v; want 0700", dir, info.Mode().Perm(), err)
		}
	}
}
