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

// event, list, dismiss and schema, against a headless ledge.
func TestEvents(t *testing.T) {
	runLedge(t)
	for _, c := range []struct {
		stdin string
		args  []string
		want  string // what it prints, in part
		err   string // "" for none
	}{
		{"", []string{"event", `{"id": "a", "title": "A"}`}, "a\n", ""},
		{`{"source": "s", "items": [{"id": "b", "title": "B"}, {"id": "c", "title": "C"}, {"id": "d", "title": "D"}]}`, []string{"event"}, "b\nc\nd\n", ""},
		{"", []string{"list"}, `"title": "C"`, ""},
		{"", []string{"dismiss", "a"}, "", ""},
		{"", []string{"dismiss", "a"}, "", `no event "a"`},
		// The others go even if one isn't an event.
		{"", []string{"dismiss", "b", "nope", "c"}, "", `no event "nope"`},
		{"", []string{"dismiss"}, "", "requires at least 1 arg"},
		{"", []string{"event", `{"items": []}`}, "", `a section needs a "source"`},
		{"", []string{"event", `{"title": `}, "", "body isn't JSON"},
		{"", []string{"list", "x"}, "", "unknown command"},
		{"", []string{"schema", "section"}, "urn:ledge:section", ""},
		{"", []string{"schema", "nope"}, "", `no schema "nope"; there are event, section`},
	} {
		out, err := ledge(t, c.stdin, c.args...)
		if !strings.Contains(out, c.want) || (c.err == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%q: printed %q, %v; want %q and error %q", c.args, out, err, c.want, c.err)
		}
	}
	if out, _ := ledge(t, "", "list"); strings.Count(out, `"id"`) != 1 || !strings.Contains(out, `"id": "d"`) {
		t.Errorf("after the dismisses, list = %s", out)
	}
}
