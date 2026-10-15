package ui

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
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
// They can't see hover, real clicks or drags, or how the windows look.
var (
	testBar *Bar
	logs    syncBuffer
	calls   recorder // what the handlers were called with
)

var defaults = Options{Edge: "bottom", Height: 110, Level: "normal", Theme: theme.Default}

// recorder keeps the handlers' calls, as "dismiss id", "open id", "run id", "url u".
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
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

func TestMain(m *testing.M) {
	if os.Getenv("LEDGE_GUI_TEST") == "" {
		fmt.Println("skipping the tests that open windows; set LEDGE_GUI_TEST=1 to run them")
		os.Exit(m.Run())
	}
	log.SetOutput(&logs)
	// The main loop has the main goroutine, so the tests run beside it.
	Init()
	var err error
	testBar, err = Open(defaults, Handlers{
		Dismiss: func(id string) { calls.add("dismiss " + id) },
		Open:    func(id string) { calls.add("open " + id) },
		Run:     func(id string) { calls.add("run " + id) },
		OpenURL: func(url string) { calls.add("url " + url) },
	})
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

// show renders pills and cards for the length of the test.
func show(t *testing.T, pills []components.Pill, cards []components.Card) {
	t.Helper()
	testBar.Render(pills, cards)
	t.Cleanup(func() { testBar.Render(nil, nil) })
}

// withTheme reloads the bar's page in theme for the length of the test; the page then
// says it's ready, and the bar draws into it.
func withTheme(t *testing.T, name string) {
	t.Helper()
	load := func(name string) {
		css, err := theme.CSS(name, theme.Bar)
		if err != nil {
			t.Fatal(err)
		}
		var page bytes.Buffer
		components.BarPage(css, testBar.options.Edge).Render(context.Background(), &page)
		testBar.mu.Lock()
		testBar.options.Theme, testBar.ready = name, false
		testBar.mu.Unlock()
		shim.LoadHTML(BarWindow, page.String())
	}
	load(name)
	t.Cleanup(func() {
		load(theme.Default)
		waitFor(t, `document.querySelector("#app > *") !== null`, "true")
	})
	waitFor(t, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* `+name+`:"))`, "true")
}

var received = time.Now().Add(-2 * time.Minute)

func TestReadMessage(t *testing.T) {
	for body, want := range map[string]string{
		`{"action": "ready"}`:                           "",
		`{"action": "open", "id": "a"}`:                 "",
		`{"action": "dismiss"}`:                         "dismiss needs an id",
		`{"action": "ready", "id": "a"}`:                "ready takes no id",
		`{"action": "exec", "id": "a"}`:                 `no action "exec"`,
		`{"action": "open", "id": "a", "path": "/etc"}`: `unknown field "path"`,
		`"dismiss"`:                        "cannot unmarshal string",
		`{"action": "open", "id": "a"} {}`: "more than one value",
	} {
		_, err := readMessage([]byte(body))
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s: got %v, want %q", body, err, want)
		}
	}
}

func TestWindow(t *testing.T) {
	gui(t)
	if frame := testBar.Place(); frame.Width == 0 || frame.Height != 110 {
		t.Errorf("frame = %+v, want the full width and 110 high", frame)
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
	gui(t)
	show(t, []components.Pill{
		{ID: "aws", Title: "AWS", Color: "green", Command: "true",
			Schedule: []components.Step{{At: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Color: "red"}}},
	}, []components.Card{
		{ID: "deploy", Title: "Deploy finished", Priority: "high", Tags: []string{"ci"}, Opens: true, Hint: "https://example.com", Source: "ci", Received: received},
		{ID: "note", Title: "<b>not bold</b>", Description: "plain text", Source: "manual", Received: received},
	})
	waitFor(t, `[...document.querySelectorAll("li .title")].map(e => e.textContent).join("|")`, `"Deploy finished|<b>not bold</b>"`)
	for js, want := range map[string]string{
		`document.querySelector("li .title b") === null`:        "true",
		`document.querySelector("li").className`:                `"high opens"`,
		`document.querySelector("li .when").textContent`:        `"ci · 2m"`,
		`document.querySelector(".pill").getAttribute("style")`: `"--pill: var(--c-red);"`,
		`document.querySelector(".pill").dataset.color`:         `"red"`,
		`document.querySelector("a") === null`:                  "true",
		`document.querySelectorAll("li[data-new]").length > 0`:  "true",
	} {
		if got := eval(t, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
}

// A pill's scheduled colour change shows when it's due, without another Render.
func TestScheduledColour(t *testing.T) {
	gui(t)
	show(t, []components.Pill{{ID: "soon", Title: "Soon", Color: "green",
		Schedule: []components.Step{{At: time.Now().Add(time.Second), Color: "red"}}}}, nil)
	waitFor(t, `document.querySelector(".pill")?.dataset.color`, `"green"`)
	waitFor(t, `document.querySelector(".pill")?.dataset.color`, `"red"`)
}

// Clicks post only ids, which reach the handlers.
func TestClicks(t *testing.T) {
	gui(t)
	show(t, []components.Pill{
		{ID: "pill-run", Title: "Run", Command: "echo hi"},
		{ID: "pill-link", Title: "Link", Link: true},
		{ID: "pill-plain", Title: "Plain"},
	}, []components.Card{
		{ID: "card-open", Title: "Opens", Opens: true, Source: "x", Received: received},
		{ID: "card-still", Title: "Still", Source: "x", Received: received},
	})
	waitFor(t, `document.querySelectorAll(".pill").length`, "3")
	for _, js := range []string{
		`document.querySelector(".pill.runs").click()`,
		`document.querySelector(".pill.opens .label").click()`,
		`[...document.querySelectorAll(".pill .label")].find(l => l.textContent === "Plain").click()`,
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Opens")).click()`,
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Still")).click()`,
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Opens")).querySelector(".dismiss").click()`,
	} {
		eval(t, js+", true")
	}
	for _, call := range []string{"run pill-run", "open pill-link", "open card-open", "dismiss card-open"} {
		eventually(t, call, func() bool { return calls.has(call) })
	}
	time.Sleep(200 * time.Millisecond)
	for _, call := range []string{"open pill-plain", "open card-still", "open pill-run"} {
		if calls.has(call) {
			t.Errorf("clicking something that doesn't open called %q", call)
		}
	}
}

func TestMessagesAreChecked(t *testing.T) {
	gui(t)
	before := logs.Len()
	for _, message := range []string{`{action: "dismiss"}`, `{action: "dismiss", id: "kept", path: "/etc"}`, `{action: "exec", id: "kept"}`, `"dismiss"`} {
		eval(t, `window.webkit.messageHandlers.ledge.postMessage(`+message+`), true`)
	}
	eventually(t, "4 refusals", func() bool { return strings.Count(logs.String()[before:], "the bar sent") == 4 })
	if calls.has("dismiss kept") {
		t.Error("a refused message reached a handler")
	}
}

// A value that can't be JSON, posted or returned, is dropped or an error, not a crash.
func TestNotJSON(t *testing.T) {
	gui(t)
	eval(t, `[new Date(), NaN, {n: Infinity}].forEach(m => window.webkit.messageHandlers.ledge.postMessage(m)), true`)
	if _, err := shim.EvalResult(BarWindow, `new Date()`); err == nil {
		t.Error("a Date result should be an error")
	}
	if got := eval(t, `1 + 1`); got != "2" {
		t.Errorf("after values that aren't JSON, 1 + 1 = %s", got)
	}
}

func TestOpenOnce(t *testing.T) {
	gui(t)
	if _, err := Open(defaults, Handlers{}); err == nil {
		t.Error("a second Open should be an error")
	}
}

func TestNoNavigation(t *testing.T) {
	gui(t)
	waitFor(t, `document.getElementById("app") !== null`, "true")
	eval(t, `location.href = "https://example.com", true`)
	time.Sleep(500 * time.Millisecond)
	if got := eval(t, `location.href`); got != `"about:blank"` {
		t.Errorf("the bar navigated to %s", got)
	}
}

func TestPillsScrollVertically(t *testing.T) {
	gui(t)
	var pills []components.Pill
	for i := range 12 {
		pills = append(pills, components.Pill{ID: fmt.Sprintf("pill-%02d", i), Title: fmt.Sprintf("Pill %02d", i)})
	}
	show(t, pills, nil)
	waitFor(t, `document.querySelectorAll(".pill").length`, "12")

	// One column: every pill starts at the same x.
	if got := eval(t, `new Set([...document.querySelectorAll(".pill")].map(p => p.offsetLeft)).size`); got != "1" {
		t.Errorf("pills are in %s columns, want 1", got)
	}
	if got := eval(t, `(p => p.scrollHeight > p.clientHeight)(document.querySelector(".pills"))`); got != "true" {
		t.Fatal("12 pills fit without scrolling; the test needs more")
	}
	eval(t, `document.querySelector(".pills").scrollTop = 40, true`)
	testBar.Render(pills, nil)
	time.Sleep(200 * time.Millisecond)
	if got := eval(t, `document.querySelector(".pills").scrollTop`); got != "40" {
		t.Errorf("after a render the pills are scrolled to %s, want 40", got)
	}
}

// Each theme loads, shows the cards, and paints the whole bar.
func TestThemes(t *testing.T) {
	gui(t)
	show(t, []components.Pill{{ID: "p", Title: "Pill", Color: "red"}},
		[]components.Card{{ID: "c", Title: "Themed", Priority: "high", Tags: []string{"a"}, Source: "x", Received: received}})
	for _, name := range theme.Names {
		withTheme(t, name)
		waitFor(t, `document.querySelector(".pill[data-color=red]") !== null && document.querySelector("li") !== null`, "true")
		if got := eval(t, `getComputedStyle(document.body).backgroundColor`); !strings.HasPrefix(got, `"rgb(`) {
			t.Errorf("%s: body background = %s, want an opaque colour", name, got)
		}
	}
}

// In win95 a long description scrolls, with a scroll bar that takes up room, and the
// mouse wheel over it scrolls the description rather than the row.
func TestWin95DescriptionScrolls(t *testing.T) {
	gui(t)
	cards := []components.Card{{ID: "long", Title: "Long", Description: strings.Repeat("A long description. ", 40), Source: "x", Received: received}}
	for i := range 8 {
		cards = append(cards, components.Card{ID: fmt.Sprintf("wide-%d", i), Title: fmt.Sprintf("Card %d", i), Source: "x", Received: received})
	}
	show(t, nil, cards)
	withTheme(t, "win95")
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

// reportBlocks is one of each block, its text partly HTML-looking.
var reportBlocks = []templ.Component{
	components.Heading("Summary"),
	components.Paragraph("<b>not bold</b>\nsecond line"),
	components.Fields([]components.Field{{Label: "Branch", Value: "main"}, {Label: "Failures", Value: "2"}}),
	components.Table([]string{"Job", "Result"}, [][]string{{"lint", "ok"}, {"test"}, {"build", "ok", "extra"}}),
	components.List([]string{"one", "two"}, true),
	components.Code("$ go test\nFAIL"),
	components.Note("Flaky", "yellow"),
	components.Link("The run", "https://example.com/from-report"),
}

// setTheme sets the theme the next report is shown in.
func setTheme(name string) {
	testBar.mu.Lock()
	defer testBar.mu.Unlock()
	testBar.options.Theme = name
}

func showReport(t *testing.T, title string, blocks []templ.Component) {
	t.Helper()
	if err := testBar.ShowReport(title, blocks); err != nil {
		t.Fatal(err)
	}
}

func TestReportWindow(t *testing.T) {
	gui(t)
	showReport(t, "A report", reportBlocks)
	waitIn(t, ReportWindow, `document.querySelector(".titlebar span")?.textContent`, `"A report"`)
	for js, want := range map[string]string{
		`[...document.querySelectorAll("main > *")].map(e => e.tagName).join(" ")`:               `"H2 P DL DIV OL PRE DIV P"`,
		`document.querySelector("p").textContent`:                                                `"<b>not bold</b>\nsecond line"`,
		`document.querySelector("p b") === null`:                                                 "true",
		`[...document.querySelectorAll("dd")].map(e => e.textContent).join()`:                    `"main,2"`,
		`[...document.querySelectorAll("tbody tr")].map(r => r.children.length).join()`:          `"2,2,2"`,
		`document.querySelector(".note").dataset.color`:                                          `"yellow"`,
		`getComputedStyle(document.documentElement).backgroundColor.startsWith("rgb(")`:          "true",
		`(s => s.position + " " + s.top)(getComputedStyle(document.querySelector(".titlebar")))`: `"fixed 0px"`,
		// Scrolling past an end doesn't bounce the page away from the window's edges.
		`getComputedStyle(document.documentElement).overscrollBehaviorY`: `"none"`,
		// The page can't post to the app.
		`!(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.ledge)`: "true",
	} {
		if got := evalIn(t, ReportWindow, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	// The link goes to OpenURL, and the report stays put.
	evalIn(t, ReportWindow, `document.querySelector("a").click(), true`)
	eventually(t, "OpenURL", func() bool { return calls.has("url https://example.com/from-report") })
	if got := evalIn(t, ReportWindow, `location.href`); got != `"about:blank"` {
		t.Errorf("the link moved the report to %s", got)
	}
	// Showing another report replaces it.
	showReport(t, "Another", []templ.Component{components.Paragraph("Second version")})
	waitIn(t, ReportWindow, `document.querySelector("p")?.textContent`, `"Second version"`)
}

// Every theme draws a report on an opaque background.
func TestReportThemes(t *testing.T) {
	gui(t)
	defer setTheme(theme.Default)
	for _, name := range theme.Names {
		setTheme(name)
		showReport(t, "Themed report", reportBlocks)
		// Each report stylesheet starts with a comment naming its theme.
		waitIn(t, ReportWindow, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* `+name+`:"))`, "true")
		if got := evalIn(t, ReportWindow, `getComputedStyle(document.documentElement).backgroundColor`); !strings.HasPrefix(got, `"rgb(`) {
			t.Errorf("%s: report background = %s, want an opaque colour", name, got)
		}
	}
}

// The report page can move and close its own window and nothing else; the bar page
// can't do even that. A theme that draws its own buttons shows them, and they work.
func TestReportFrame(t *testing.T) {
	gui(t)
	defer setTheme(theme.Default)
	if got := eval(t, `!!window.webkit.messageHandlers.frame`); got != "false" {
		t.Error("the bar page has the frame handler")
	}
	blocks := []templ.Component{components.Paragraph("Framed")}
	showReport(t, "Framed report", blocks)
	waitIn(t, ReportWindow, `document.querySelector(".titlebar span")?.textContent`, `"Framed report"`)
	if got := evalIn(t, ReportWindow, `!!window.webkit.messageHandlers.frame`); got != "true" {
		t.Fatal("the report page has no frame handler")
	}
	// Anything but the four actions is dropped, and a drag needs the mouse held down.
	evalIn(t, ReportWindow, `["open", {action: "close"}, 1, "drag"].forEach(m => window.webkit.messageHandlers.frame.postMessage(m)), true`)
	time.Sleep(200 * time.Millisecond)
	if !shim.WindowVisible(ReportWindow) {
		t.Fatal("a message other than close closed the report window")
	}
	if got := evalIn(t, ReportWindow, `getComputedStyle(document.querySelector(".titlebar .close")).display`); got != `"none"` {
		t.Errorf("the default theme shows its own close button (display %s)", got)
	}

	// Win 95 draws the whole window, buttons included; its close button closes it.
	setTheme("win95")
	showReport(t, "Framed report", blocks)
	waitIn(t, ReportWindow, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* win95:"))`, "true")
	for _, button := range []string{"close", "minimise", "zoom"} {
		if got := evalIn(t, ReportWindow, `getComputedStyle(document.querySelector(".titlebar .`+button+`")).display`); got == `"none"` {
			t.Errorf("win95 hides its %s button", button)
		}
	}
	evalIn(t, ReportWindow, `document.querySelector(".titlebar .close").click(), true`)
	eventually(t, "the report window to close", func() bool { return !shim.WindowVisible(ReportWindow) })

	// Its minimise button works without a title bar too, and showing it brings it back.
	showReport(t, "Framed report", blocks)
	eventually(t, "the report window to open", func() bool { return shim.WindowVisible(ReportWindow) })
	evalIn(t, ReportWindow, `document.querySelector(".titlebar .minimise").click(), true`)
	eventually(t, "the report window to minimise", func() bool { return !shim.WindowVisible(ReportWindow) })
	showReport(t, "Framed report", blocks)
	eventually(t, "the report window to come back", func() bool { return shim.WindowVisible(ReportWindow) })
}

func TestNavigate(t *testing.T) {
	bar := &Bar{handlers: Handlers{OpenURL: func(string) {}}}
	for _, c := range []struct {
		window  int
		url     string
		clicked bool
		want    bool
	}{
		{BarWindow, "about:blank", false, true},
		{BarWindow, "https://example.com", true, false},
		{ReportWindow, "about:blank", false, true},
		{ReportWindow, "file:///etc/hosts", true, false},
		{ReportWindow, "https://example.com", false, false},
		{ReportWindow, "https://example.com", true, false}, // to OpenURL instead
		{ReportWindow, "javascript:alert(1)", true, false},
	} {
		if got := bar.navigate(c.window, c.url, c.clicked); got != c.want {
			t.Errorf("navigate(%d, %s, %v) = %v, want %v", c.window, c.url, c.clicked, got, c.want)
		}
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
