package bar

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ledge/internal/config"
	"ledge/internal/pages"
	"ledge/internal/schema"
	"ledge/internal/shim"
	"ledge/internal/store"
)

// These tests open a real bar window, so they only run with LEDGE_GUI_TEST=1. They
// can't see hover, real clicks or how the bar looks.
var (
	testBar   *Bar
	testStore *store.Store
	testDir   string
	logs      syncBuffer
	opened    recorder // what `open` would have opened
)

// recorder keeps the urls `open` was run with. Tests never open the browser.
type recorder struct {
	mu   sync.Mutex
	urls []string
}

func (r *recorder) has(url string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.urls {
		if u == url {
			return true
		}
	}
	return false
}

// testStart runs a status's bash command for real, but only records `open`.
func testStart(cmd *exec.Cmd) error {
	if filepath.Base(cmd.Path) == "open" {
		opened.mu.Lock()
		opened.urls = append(opened.urls, cmd.Args[1])
		opened.mu.Unlock()
		return nil
	}
	return startDetached(cmd)
}

func TestMain(m *testing.M) {
	if os.Getenv("LEDGE_GUI_TEST") == "" {
		fmt.Println("skipping: these tests open a window; set LEDGE_GUI_TEST=1 to run them")
		os.Exit(0)
	}
	dir, err := os.MkdirTemp("", "ledge-bar-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	testDir = dir
	if testStore, err = store.Open(filepath.Join(dir, "events.json")); err != nil {
		log.Fatal(err)
	}
	log.SetOutput(&logs)

	// The main loop has the main goroutine, so the tests run beside it.
	shim.Init()
	testBar = Open(testStore, config.Defaults, dir)
	testBar.start = testStart
	code := make(chan int, 1)
	go func() {
		code <- m.Run()
		shim.Stop()
	}()
	shim.Run()
	os.Exit(<-code)
}

func eval(t *testing.T, js string) string {
	t.Helper()
	return evalIn(t, Window, js)
}

func evalIn(t *testing.T, window int, js string) string {
	t.Helper()
	value, err := shim.EvalResult(window, js)
	if err != nil {
		t.Fatalf("%s: %v", js, err)
	}
	return string(value)
}

// waitFor evaluates js in the bar until it gives want.
func waitFor(t *testing.T, js, want string) {
	t.Helper()
	waitIn(t, Window, js, want)
}

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

// click clicks the element matching selector in the bar, once the bar shows it.
func click(t *testing.T, selector string) {
	t.Helper()
	testBar.Render()
	waitFor(t, `document.querySelector(`+jsString(selector)+`) !== null`, "true")
	eval(t, `document.querySelector(`+jsString(selector)+`).click(), true`)
}

func jsString(s string) string {
	return fmt.Sprintf("%q", s)
}

// add adds an event for the length of the test.
func add(t *testing.T, body string) {
	t.Helper()
	value, err := schema.Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	id, err := testStore.Add(value, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testStore.Remove(id) })
}

func TestWindow(t *testing.T) {
	if frame := testBar.place(); frame.Width == 0 || frame.Height != float64(config.Defaults.Height) {
		t.Errorf("frame = %+v, want the full width and %d high", frame, config.Defaults.Height)
	}
	waitFor(t, `document.getElementById("app") !== null`, "true")
	if got := eval(t, `document.body.className`); got != `"bottom"` {
		t.Errorf("body class = %s", got)
	}
	// The bar is solid: nothing behind it shows through.
	if got := eval(t, `getComputedStyle(document.body).backgroundColor`); !strings.HasPrefix(got, `"rgb(`) {
		t.Errorf("body background = %s, want an opaque colour", got)
	}
}

func TestRender(t *testing.T) {
	add(t, `{"id": "deploy", "title": "Deploy finished", "priority": "high", "tags": ["ci"], "url": "https://example.com"}`)
	add(t, `{"id": "note", "title": "<b>not bold</b>", "description": "plain text"}`)
	add(t, `{"kind": "status", "id": "aws", "title": "AWS", "color": "green",
		"schedule": [{"at": "2020-01-01T00:00:00Z", "color": "red"}], "command": "true"}`)
	testBar.Render()

	waitFor(t, `[...document.querySelectorAll("li .title")].map(e => e.textContent).join("|")`, `"Deploy finished|<b>not bold</b>"`)
	if got := eval(t, `document.querySelector("li .title b") === null`); got != "true" {
		t.Error("event text was rendered as HTML")
	}
	if got := eval(t, `document.querySelector("li").className`); got != `"high opens"` {
		t.Errorf("card class = %s", got)
	}
	// The schedule's step has passed, so the pill shows its colour.
	if got := eval(t, `document.querySelector(".pill").getAttribute("style")`); got != `"--pill: var(--c-red)"` {
		t.Errorf("pill style = %s", got)
	}
	if got := eval(t, `document.querySelector("a") === null`); got != "true" {
		t.Error("the bar has a link, which the window would refuse to follow")
	}
}

func TestDismiss(t *testing.T) {
	add(t, `{"id": "gone", "title": "Dismiss me"}`)
	testBar.Render()
	waitFor(t, `!![...document.querySelectorAll("li .title")].find(e => e.textContent === "Dismiss me")`, "true")

	eval(t, `[...document.querySelectorAll("li")].find(li => li.textContent.includes("Dismiss me")).querySelector(".dismiss").click()`)
	waitFor(t, `!![...document.querySelectorAll("li .title")].find(e => e.textContent === "Dismiss me")`, "false")
	if _, ok := testStore.Get("gone"); ok {
		t.Error("dismissed in the page but still in the store")
	}
}

func TestMessagesAreChecked(t *testing.T) {
	add(t, `{"id": "kept", "title": "Kept"}`)
	before := logs.Len()
	for _, message := range []string{`{action: "dismiss"}`, `{action: "dismiss", id: "kept", path: "/etc"}`, `{action: "exec", id: "kept"}`, `"dismiss"`} {
		eval(t, `window.webkit.messageHandlers.ledge.postMessage(`+message+`), true`)
	}
	deadline := time.Now().Add(5 * time.Second)
	for strings.Count(logs.String()[before:], "bar sent") < 4 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got := strings.Count(logs.String()[before:], "bar sent"); got != 4 {
		t.Errorf("%d of 4 bad messages refused; log:\n%s", got, logs.String()[before:])
	}
	if _, ok := testStore.Get("kept"); !ok {
		t.Error("a refused message dismissed an event")
	}
}

func TestNoNavigation(t *testing.T) {
	waitFor(t, `document.getElementById("app") !== null`, "true")
	eval(t, `location.href = "https://example.com", true`)
	time.Sleep(500 * time.Millisecond)
	if got := eval(t, `location.href`); got != `"about:blank"` {
		t.Errorf("the bar navigated to %s", got)
	}
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

func TestPillsScrollVertically(t *testing.T) {
	for i := range 12 {
		add(t, fmt.Sprintf(`{"kind": "status", "id": "pill-%02d", "source": "pills", "title": "Pill %02d"}`, i, i))
	}
	defer func() {
		value, _ := schema.Decode([]byte(`[]`))
		testStore.ReplaceSource("pills", value.([]any), time.Now())
		testBar.Render()
	}()
	testBar.Render()
	waitFor(t, `document.querySelectorAll(".pill").length >= 12`, "true")

	// One column: every pill starts at the same x.
	if got := eval(t, `new Set([...document.querySelectorAll(".pill")].map(p => p.offsetLeft)).size`); got != "1" {
		t.Errorf("pills are in %s columns, want 1", got)
	}
	if got := eval(t, `(p => p.scrollHeight > p.clientHeight)(document.querySelector(".pills"))`); got != "true" {
		t.Fatal("12 pills fit without scrolling; the test needs more")
	}
	eval(t, `document.querySelector(".pills").scrollTop = 40, true`)
	if got := eval(t, `document.querySelector(".pills").scrollTop`); got != "40" {
		t.Fatalf("pills scrolled to %s, want 40", got)
	}
	testBar.Render()
	time.Sleep(200 * time.Millisecond)
	if got := eval(t, `document.querySelector(".pills").scrollTop`); got != "40" {
		t.Errorf("after a render the pills are scrolled to %s, want 40", got)
	}
}

// Each theme loads, shows the cards, and paints the whole bar. Loading a page sends
// "ready", so the bar renders itself. The default theme goes back afterwards.
func TestThemes(t *testing.T) {
	defer func() {
		shim.LoadHTML(Window, page(config.Defaults))
		waitFor(t, `document.querySelector("li") !== null`, "true")
	}()
	add(t, `{"id": "themed", "title": "Themed", "priority": "high", "tags": ["a"]}`)
	add(t, `{"kind": "status", "id": "themed-pill", "title": "Pill", "color": "red"}`)
	for _, name := range pages.Themes {
		cfg := config.Defaults
		cfg.Theme = name
		shim.LoadHTML(Window, page(cfg))
		waitFor(t, `document.querySelector(".pill[data-color=red]") !== null && document.querySelector("li") !== null`, "true")
		if got := eval(t, `getComputedStyle(document.body).backgroundColor`); !strings.HasPrefix(got, `"rgb(`) {
			t.Errorf("%s: body background = %s, want an opaque colour", name, got)
		}
	}
}

// In win95 a long description scrolls, with a scroll bar that takes up room, and the
// mouse wheel over it scrolls the description rather than the row.
func TestWin95DescriptionScrolls(t *testing.T) {
	defer func() {
		shim.LoadHTML(Window, page(config.Defaults))
		waitFor(t, `document.querySelector("li") !== null`, "true")
	}()
	add(t, `{"id": "long", "title": "Long", "description": "`+strings.Repeat("A long description. ", 40)+`"}`)
	for i := range 8 {
		add(t, fmt.Sprintf(`{"id": "wide-%d", "title": "Card %d"}`, i, i))
	}
	cfg := config.Defaults
	cfg.Theme = "win95"
	shim.LoadHTML(Window, page(cfg))
	waitFor(t, `document.querySelector("li .description") !== null`, "true")

	const description = `document.querySelector("li .description")`
	if got := eval(t, `(d => d.scrollHeight > d.clientHeight && d.clientWidth < d.offsetWidth - 6)(`+description+`)`); got != "true" {
		t.Fatal("the description doesn't scroll with a scroll bar")
	}
	got := eval(t, `(d => {
		const row = document.querySelector("ul"), before = row.scrollLeft;
		const wheel = new WheelEvent("wheel", {deltaY: 40, bubbles: true, cancelable: true});
		d.dispatchEvent(wheel);
		return !wheel.defaultPrevented && row.scrollLeft === before;
	})(`+description+`)`)
	if got != "true" {
		t.Error("the wheel over a scrolling description moved the row")
	}
}
