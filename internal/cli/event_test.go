package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/app"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/spf13/pflag"
)

// ledge runs the CLI with args and stdin, and gives what it printed.
func ledge(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	// Flags keep their values between runs of the same command.
	for _, cmd := range rootCmd.Commands() {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Value.Set(f.DefValue)
			f.Changed = false
		})
	}
	var out bytes.Buffer
	rootCmd.SetArgs(args)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetOut(&out)
	err := rootCmd.Execute()
	return out.String(), err
}

// runLedge runs a headless ledge in a new state dir, which $XDG_STATE_HOME names for
// the length of the test.
func runLedge(t *testing.T) {
	t.Helper()
	// Not t.TempDir(): a socket's path must fit in 104 bytes.
	home, err := os.MkdirTemp("", "ledge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("XDG_STATE_HOME", home)
	dir, err := state.Dir()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx, dir, api.Options{Headless: true}) }()
	t.Cleanup(func() { cancel(); <-done })
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		var status api.Status
		if (api.Client{Socket: state.Socket(dir)}).Call("GET", "/status", nil, &status) == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the API never answered")
		}
	}
}

func TestEvent(t *testing.T) {
	runLedge(t)
	for _, c := range []struct {
		stdin string
		args  []string
		want  string // what it prints, in part
		err   string // "" for none
	}{
		{"", []string{"event", `{"id": "a", "title": "A"}`}, "a\n", ""},
		{`{"source": "s", "items": [{"id": "b", "title": "B"}, {"id": "c", "title": "C"}]}`, []string{"event"}, "b\nc\n", ""},
		{"", []string{"event", "--list"}, `"title": "C"`, ""},
		{"", []string{"event", "--dismiss", "a"}, "", ""},
		{"", []string{"event", "--dismiss", "a"}, "", `no event "a"`},
		{"", []string{"event", `{"items": []}`}, "", `a section needs a "source"`},
		{"", []string{"event", `{"title": `}, "", "body isn't JSON"},
		{"", []string{"event", "--list", `{"title": "x"}`}, "", "can't be combined"},
		{"", []string{"event", "--schema", "section"}, "urn:ledge:section", ""},
		{"", []string{"event", "--schema", "nope"}, "", `no schema "nope"; there are event, section`},
	} {
		out, err := ledge(t, c.stdin, c.args...)
		if !strings.Contains(out, c.want) || (c.err == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%q: printed %q, %v; want %q and error %q", c.args, out, err, c.want, c.err)
		}
	}
	if out, _ := ledge(t, "", "event", "--list"); strings.Contains(out, `"id": "a"`) || !strings.Contains(out, `"id": "b"`) {
		t.Errorf("after dismissing a, --list = %s", out)
	}
}
