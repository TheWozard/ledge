package ui

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// The tests that open windows only run with LEDGE_GUI_TEST=1; the rest always run.
// They can't see hover, real clicks or drags, or how the windows look. Those that go
// through every built-in theme go through only those in LEDGE_THEMES, if it's set.
var (
	testBar *Bar
	logs    syncBuffer
	calls   recorder // what the Clicks and openURL were called with
)

var defaults = Options{Edge: "bottom", Height: 110, Level: "normal"} // the zero Theme: the default

// recorder keeps the calls of the test Clicks and openURL: each Click's name, and
// "url u".
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *recorder) count(call string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		if c == call {
			n++
		}
	}
	return n
}

func (r *recorder) has(call string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c == call {
			return true
		}
	}
	return false
}

// record is a Click that records name.
func record(name string) components.Click { return func() { calls.add(name) } }

func TestMain(m *testing.M) {
	openURL = func(url string) { calls.add("url " + url) }
	if os.Getenv("LEDGE_GUI_TEST") == "" {
		fmt.Println("skipping the tests that open windows; set LEDGE_GUI_TEST=1 to run them")
		os.Exit(m.Run())
	}
	log.SetOutput(&logs)
	// The main loop has the main goroutine, so the tests run beside it.
	Init()
	var err error
	testBar, err = Open(defaults)
	if err != nil {
		log.Fatal(err)
	}
	code := make(chan int, 1)
	go func() {
		code <- m.Run()
		Stop()
	}()
	Run()
	os.Exit(<-code)
}

// themes is the built-in themes for the tests that go through each one: all of them,
// or only those named in LEDGE_THEMES, comma-separated, such as LEDGE_THEMES=default.
func themes(t *testing.T) []string {
	t.Helper()
	names := os.Getenv("LEDGE_THEMES")
	if names == "" {
		return theme.Builtins.Names()
	}
	var chosen []string
	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		if !slices.Contains(theme.Builtins.Names(), name) {
			t.Fatalf("LEDGE_THEMES: no theme %q; the themes are %s", name, strings.Join(theme.Builtins.Names(), ", "))
		}
		chosen = append(chosen, name)
	}
	return chosen
}

func gui(t *testing.T) {
	t.Helper()
	if testBar == nil {
		t.Skip("opens a window; set LEDGE_GUI_TEST=1")
	}
}

func evalIn(t *testing.T, window int, js string) string {
	t.Helper()
	value, err := shim.EvalResult(window, js)
	if err != nil {
		t.Fatalf("%s: %v", js, err)
	}
	return string(value)
}

func eval(t *testing.T, js string) string {
	t.Helper()
	return evalIn(t, BarWindow, js)
}

// waitIn evaluates js in the window until it gives want.
func waitIn(t *testing.T, window int, js, want string) {
	t.Helper()
	var got string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if value, err := shim.EvalResult(window, js); err == nil {
			if got = string(value); got == want {
				return
			}
		}
	}
	t.Fatalf("%s = %s, want %s", js, got, want)
}

func waitFor(t *testing.T, js, want string) {
	t.Helper()
	waitIn(t, BarWindow, js, want)
}

// eventually waits for check to be true.
func eventually(t *testing.T, what string, check func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if check() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// show renders cards for the length of the test. Afterwards it clears the bar, and
// waits for them to have left.
func show(t *testing.T, cards []components.Basic) {
	t.Helper()
	testBar.Render(components.Cards(cards))
	t.Cleanup(func() {
		testBar.Render(nil)
		waitFor(t, `document.querySelector("#cards > *") === null`, "true")
	})
}

// testThemes are themes made for a test, for what no built-in theme does. Each starts
// from the default and adds to it.
var testThemes = func() map[string]theme.Theme {
	def, err := theme.Builtin(theme.Builtins.Default())
	if err != nil {
		panic(err)
	}
	frameless := def
	frameless.Name = "frameless"
	frameless.Report += "\n/* frameless: the window has no frame of its own. */"
	frameless.Frame = theme.Frame{Frameless: true}
	native := def
	native.Name, native.Menu = "native", ""
	native.Bar += "\n/* native: a card's menu is the native one. */"
	return map[string]theme.Theme{"frameless": frameless, "native": native}
}()

// testTheme is the test theme name, or the built-in one.
func testTheme(name string) (theme.Theme, error) {
	if th, ok := testThemes[name]; ok {
		return th, nil
	}
	return theme.Builtin(name)
}

// withTheme reloads the bar's page in theme for the length of the test; the page then
// says it's ready, and the bar draws into it.
func withTheme(t *testing.T, name string) {
	t.Helper()
	load := func(name string) {
		th, err := testTheme(name)
		if err != nil {
			t.Fatal(err)
		}
		var page bytes.Buffer
		components.BarPage(th.Bar, testBar.options.Edge).Render(context.Background(), &page)
		testBar.mu.Lock()
		testBar.options.Theme, testBar.ready = th, false
		testBar.mu.Unlock()
		shim.LoadHTML(BarWindow, page.String())
	}
	load(name)
	t.Cleanup(func() {
		load(theme.Builtins.Default())
		waitFor(t, `document.querySelector("#app > *") !== null`, "true")
	})
	waitFor(t, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* `+name+`:"))`, "true")
}

// setTheme sets the theme the next report is shown in.
func setTheme(name string) {
	th, err := testTheme(name)
	if err != nil {
		panic(err)
	}
	testBar.mu.Lock()
	defer testBar.mu.Unlock()
	testBar.options.Theme = th
}

// showReport shows a report for id and returns its window.
func showReport(t *testing.T, id, title string, blocks []templ.Component) int {
	t.Helper()
	if err := testBar.ShowReport(id, title, blocks); err != nil {
		t.Fatal(err)
	}
	window, _ := testBar.reports.open(id)
	return window
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}
