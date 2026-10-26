package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/app"
	"github.com/TheWozard/ledge/internal/state"
	"github.com/spf13/pflag"
)

// The test binary stands in for ledge: `ledge start` runs it as `<test binary> run`,
// and with LEDGE_CLI_TEST set, that runs the CLI as ledge would.
func TestMain(m *testing.M) {
	if os.Getenv("LEDGE_CLI_TEST") != "" && len(os.Args) > 1 && os.Args[1] == "run" {
		Execute()
		return
	}
	os.Exit(m.Run())
}

// stateHome makes a new state home, which $XDG_STATE_HOME names for the length of the
// test.
func stateHome(t *testing.T) string {
	t.Helper()
	// Not t.TempDir(): a socket's path must fit in 104 bytes.
	home, err := os.MkdirTemp("", "ledge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("XDG_STATE_HOME", home)
	return home
}

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

// runLedge runs a headless ledge in this test, in a new state home.
func runLedge(t *testing.T) {
	t.Helper()
	stateHome(t)
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

// start, stop and restart, running the test binary as a background ledge.
func TestBackground(t *testing.T) {
	home := stateHome(t)
	t.Setenv("LEDGE_CLI_TEST", "1")
	t.Chdir(home)
	t.Cleanup(func() { ledge(t, "", "stop") })
	if err := os.WriteFile("config.json", []byte(`{"commands": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("bad.json", []byte(`{"bad": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	status := func() api.Status {
		t.Helper()
		client, err := client()
		if err != nil {
			t.Fatal(err)
		}
		var status api.Status
		if err := client.Call("GET", "/status", nil, &status); err != nil {
			t.Fatalf("no ledge answering: %v", err)
		}
		return status
	}

	for _, c := range []struct {
		args []string
		want []string // what it prints, in part
		err  string   // "" for none
	}{
		{[]string{"start", "--headless"}, []string{"ledge started"}, ""},
		{[]string{"start"}, []string{"ledge is already running"}, ""},
		{[]string{"version"}, []string{"running: ledge"}, ""},
		// Without flags, the running ledge's.
		{[]string{"restart"}, []string{"Using the flags: --headless\n", "ledge stopped", "ledge started"}, ""},
		// A bad config leaves it running.
		{[]string{"restart", "--headless", "--config", "bad.json"}, nil, "additional properties 'bad' not allowed"},
		// A relative config is made absolute, as the background ledge runs in $HOME.
		{[]string{"restart", "--headless", "--config", "config.json"}, []string{"ledge stopped", "ledge started"}, ""},
		{[]string{"stop"}, []string{"ledge stopped"}, ""},
		{[]string{"stop"}, []string{"ledge isn't running"}, ""},
		// With none running, the last start's.
		{[]string{"restart"}, []string{"Using the flags: --config /", "config.json --headless\n", "ledge started"}, ""},
	} {
		out, err := ledge(t, "", c.args...)
		if (c.err == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Fatalf("%q: %v; want error %q", c.args, err, c.err)
		}
		for _, want := range c.want {
			if !strings.Contains(out, want) {
				t.Errorf("%q printed %q; want %q", c.args, out, want)
			}
		}
		if c.err != "" {
			status() // still running
		}
	}
	if options := status().Options; !options.Headless || !filepath.IsAbs(options.Config) || filepath.Base(options.Config) != "config.json" {
		t.Errorf("the running ledge's options = %+v", options)
	}
}

func TestUpdateRefusesADevBuild(t *testing.T) {
	if _, err := ledge(t, "", "update"); err == nil || !strings.Contains(err.Error(), "development build") {
		t.Errorf("update = %v", err)
	}
}

// $LEDGE_CONFIG is the config unless --config is given, for start and restart alike.
func TestConfigFromEnv(t *testing.T) {
	home := stateHome(t)
	t.Setenv("LEDGE_CLI_TEST", "1")
	t.Chdir(home)
	t.Cleanup(func() { ledge(t, "", "stop") })
	for _, name := range []string{"env.json", "flag.json"} {
		if err := os.WriteFile(name, []byte(`{"edge": "top"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := func() string {
		t.Helper()
		client, err := client()
		if err != nil {
			t.Fatal(err)
		}
		var status api.Status
		if err := client.Call("GET", "/status", nil, &status); err != nil {
			t.Fatalf("no ledge answering: %v", err)
		}
		return filepath.Base(status.Options.Config)
	}

	t.Setenv("LEDGE_CONFIG", "env.json")
	for _, c := range []struct {
		args []string
		want string // the running ledge's config file, "." for none
	}{
		{[]string{"start", "--headless"}, "env.json"},
		{[]string{"restart", "--headless", "--config", "flag.json"}, "flag.json"},
		// Without flags, the environment's beats the running ledge's.
		{[]string{"restart"}, "env.json"},
		// --config "" is none, whatever the environment says.
		{[]string{"restart", "--headless", "--config", ""}, "."},
	} {
		if _, err := ledge(t, "", c.args...); err != nil {
			t.Fatalf("%q: %v", c.args, err)
		}
		if got := config(); got != c.want {
			t.Errorf("after %q, the config is %q; want %q", c.args, got, c.want)
		}
	}

	// A bad one fails as --config would, before anything stops.
	t.Setenv("LEDGE_CONFIG", "missing.json")
	if _, err := ledge(t, "", "restart"); err == nil || !strings.Contains(err.Error(), "missing.json") {
		t.Errorf("restart with a missing $LEDGE_CONFIG = %v", err)
	}
	config()
}
