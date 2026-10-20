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
// They can't see hover, real clicks or drags, or how the windows look.
var (
	testBar *Bar
	logs    syncBuffer
	calls   recorder // what the handlers were called with
)

var defaults = Options{Edge: "bottom", Height: 110, Level: "normal"} // the zero Theme: the default

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

// show renders pills and cards for the length of the test. Afterwards it clears the
// bar, and waits for them to have left.
func show(t *testing.T, pills []components.Pill, cards []components.Card) {
	t.Helper()
	testBar.Render(pills, cards)
	t.Cleanup(func() {
		testBar.Render(nil, nil)
		waitFor(t, `document.querySelector("#pills > *, #cards > *") === null`, "true")
	})
}

// withTheme reloads the bar's page in theme for the length of the test; the page then
// says it's ready, and the bar draws into it.
func withTheme(t *testing.T, name string) {
	t.Helper()
	load := func(name string) {
		th, err := theme.Builtin(name)
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
		`document.querySelector(".pill").getAttribute("style")`: `"--pill: var(--c-red); --i: 0;"`,
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

// A render morphs the bar: cards and pills still there keep their elements, the row
// its scroll and a run pill its flash, while what changed changes.
func TestRenderMorphs(t *testing.T) {
	gui(t)
	pills := []components.Pill{{ID: "run", Title: "Run", Command: "true"}, {ID: "still", Title: "Still"}}
	var cards []components.Card
	for i := range 12 {
		cards = append(cards, components.Card{ID: fmt.Sprintf("m-%02d", i), Title: fmt.Sprintf("Card %02d", i), Source: "x", Received: received})
	}
	show(t, pills, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "12")
	eval(t, `window.kept = document.getElementById("card-m-05");
		window.keptPill = document.getElementById("pill-still");
		document.getElementById("cards").scrollLeft = 300;
		document.getElementById("pill-run").click(), true`)
	if got := eval(t, `document.getElementById("cards").scrollLeft`); got != "300" {
		t.Fatalf("the row scrolled to %s, want 300; the test needs more cards", got)
	}

	// The first card goes, one changes its title, and one arrives.
	changed := slices.Clone(cards[1:])
	changed[4].Title = "Renamed"
	changed = append(changed, components.Card{ID: "m-new", Title: "New", Source: "x", Received: received})
	testBar.Render(pills, changed)
	waitFor(t, `document.getElementById("card-m-new") !== null`, "true")
	for js, want := range map[string]string{
		`document.getElementById("card-m-05") === window.kept`:                             "true",
		`document.getElementById("pill-still") === window.keptPill`:                        "true",
		`window.kept.querySelector(".title").textContent`:                                  `"Renamed"`,
		`document.getElementById("card-m-00").hasAttribute("data-leaving")`:                "true",
		`document.getElementById("cards").scrollLeft`:                                      "300",
		`document.getElementById("pill-run").classList.contains("ran")`:                    "true",
		`window.kept.hasAttribute("data-new")`:                                             "false",
		`document.getElementById("card-m-new").hasAttribute("data-new")`:                   "true",
		`[...document.querySelectorAll("li:not([data-leaving])")].map(li => li.id).join()`: `"` + strings.Join(idsOf(changed), ",") + `"`,
	} {
		if got := eval(t, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	waitFor(t, `document.getElementById("card-m-00") === null`, "true")
}

// In every theme, new pills and cards animate in, and gone ones animate out before
// they're removed, while what follows closes up; "No events." waits for the last.
func TestEnterAndLeave(t *testing.T) {
	gui(t)
	pills := []components.Pill{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}
	cards := []components.Card{
		{ID: "x", Title: "X", Source: "s", Received: received},
		{ID: "y", Title: "Y", Source: "s", Received: received},
	}
	for _, name := range theme.Builtins.Names() {
		t.Run(name, func(t *testing.T) {
			withTheme(t, name)
			show(t, pills, cards)
			waitFor(t, `document.querySelectorAll("#pills > .pill, #cards > li").length`, "4")
			// Everything's new on a fresh page, and animates in.
			if got := eval(t, `[...document.querySelectorAll(".pill, li")].every(e => e.hasAttribute("data-new") && e.getAnimations().length > 0)`); got != "true" {
				t.Errorf("not every new pill and card is animating in")
			}
			// Once in, nothing's left of the animation: none holds its end, which
			// WebKit can leave a step short of how the element looks.
			waitFor(t, `[...document.querySelectorAll(".pill, li")].every(e => e.getAnimations().length === 0)`, "true")

			// Pill a and card x go: each is kept, leaving, out of reach of clicks,
			// and told how far to collapse.
			testBar.Render(pills[1:], cards[1:])
			waitFor(t, `document.getElementById("card-x")?.hasAttribute("data-leaving")`, "true")
			for _, id := range []string{"pill-a", "card-x"} {
				got := eval(t, `(e => [e.hasAttribute("data-leaving"), getComputedStyle(e).pointerEvents,
					e.getAnimations().length > 0,
					parseFloat(e.style.getPropertyValue(e.classList.contains("pill") ? "--collapse-y" : "--collapse-x")) < 0].join())(document.getElementById("`+id+`"))`)
				if got != `"true,none,true,true"` {
					t.Errorf("%s: leaving, pointer-events, animating, collapsing = %s", id, got)
				}
			}
			waitFor(t, `document.getElementById("card-x") === null && document.getElementById("pill-a") === null`, "true")
			// By layout, not on screen, as a theme can tilt or move its cards.
			if got := eval(t, `document.getElementById("card-y").offsetLeft === document.getElementById("cards").offsetLeft + parseFloat(getComputedStyle(document.getElementById("cards")).paddingLeft)`); got != "true" {
				t.Error("card y didn't close up into card x's place")
			}

			// The last card leaves before "No events." shows.
			testBar.Render(nil, nil)
			waitFor(t, `document.getElementById("card-y")?.hasAttribute("data-leaving")`, "true")
			if got := eval(t, `getComputedStyle(document.querySelector(".empty")).display`); got != `"none"` {
				t.Errorf(`"No events." shows while the last card is leaving (display %s)`, got)
			}
			waitFor(t, `document.getElementById("card-y") === null && getComputedStyle(document.querySelector(".empty")).display !== "none"`, "true")
		})
	}
}

// A card that comes back while it's leaving stays.
func TestComesBackWhileLeaving(t *testing.T) {
	gui(t)
	cards := []components.Card{{ID: "back", Title: "Back", Source: "s", Received: received}, {ID: "other", Title: "Other", Source: "s", Received: received}}
	show(t, nil, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "2")
	eval(t, `window.back = document.getElementById("card-back"), true`)
	testBar.Render(nil, cards[1:])
	waitFor(t, `window.back.hasAttribute("data-leaving")`, "true")
	testBar.Render(nil, cards)
	waitFor(t, `window.back.hasAttribute("data-leaving")`, "false")
	time.Sleep(3500 * time.Millisecond) // past the longest a leaving card is kept
	if got := eval(t, `document.getElementById("card-back") === window.back && window.back.isConnected`); got != "true" {
		t.Error("a card that came back was removed anyway")
	}
}

func idsOf(cards []components.Card) []string {
	var ids []string
	for _, c := range cards {
		ids = append(ids, "card-"+c.ID)
	}
	return ids
}

// Each theme loads, shows the cards, and paints the whole bar.
func TestThemes(t *testing.T) {
	gui(t)
	show(t, []components.Pill{{ID: "p", Title: "Pill", Color: "red"}},
		[]components.Card{{ID: "c", Title: "Themed", Priority: "high", Tags: []string{"a"}, Source: "x", Received: received}})
	for _, name := range theme.Builtins.Names() {
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
	th, err := theme.Builtin(name)
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
	return testBar.reportWindow(id)
}

func TestReportWindow(t *testing.T) {
	gui(t)
	w := showReport(t, "report", "A report", reportBlocks)
	waitIn(t, w, `document.querySelector(".titlebar span")?.textContent`, `"A report"`)
	for js, want := range map[string]string{
		`[...document.querySelectorAll("main > article > *")].map(e => e.tagName).join(" ")`:               `"H2 P DL DIV OL PRE DIV P"`,
		`document.querySelector("p").textContent`:                                                `"<b>not bold</b>\nsecond line"`,
		`document.querySelector("p b") === null`:                                                 "true",
		`[...document.querySelectorAll("dd")].map(e => e.textContent).join()`:                    `"main,2"`,
		`[...document.querySelectorAll("tbody tr")].map(r => r.children.length).join()`:          `"2,2,2"`,
		`document.querySelector(".note").dataset.color`:                                          `"yellow"`,
		`getComputedStyle(document.documentElement).backgroundColor.startsWith("rgb(")`:          "true",
		`(s => s.position + " " + s.top)(getComputedStyle(document.querySelector(".titlebar")))`: `"fixed 0px"`,
		// The page doesn't scroll, so it can't bounce away from the window's edges:
		// <main> scrolls, under the title bar, and has the keys.
		`(s => s.overflow)(getComputedStyle(document.documentElement))`: `"hidden"`,
		`getComputedStyle(document.querySelector("main")).overflowY`:    `"auto"`,
		`document.activeElement.tagName`:                                `"MAIN"`,
		// The page can't post to the app.
		`!(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.ledge)`: "true",
	} {
		if got := evalIn(t, w, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	// The link goes to OpenURL, and the report stays put.
	evalIn(t, w, `document.querySelector("a").click(), true`)
	eventually(t, "OpenURL", func() bool { return calls.has("url https://example.com/from-report") })
	if got := evalIn(t, w, `location.href`); got != `"about:blank"` {
		t.Errorf("the link moved the report to %s", got)
	}
	// Showing its id again replaces it, in the same window.
	if again := showReport(t, "report", "Another", []templ.Component{components.Paragraph("Second version")}); again != w {
		t.Errorf("showing the same id again opened window %d, not %d", again, w)
	}
	waitIn(t, w, `document.querySelector("p")?.textContent`, `"Second version"`)
}

// Every theme draws a report on an opaque background.
func TestReportThemes(t *testing.T) {
	gui(t)
	defer setTheme(theme.Builtins.Default())
	for _, name := range theme.Builtins.Names() {
		setTheme(name)
		w := showReport(t, "report", "Themed report", reportBlocks)
		// Each report stylesheet starts with a comment naming its theme.
		waitIn(t, w, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* `+name+`:"))`, "true")
		if got := evalIn(t, w, `getComputedStyle(document.documentElement).backgroundColor`); !strings.HasPrefix(got, `"rgb(`) {
			t.Errorf("%s: report background = %s, want an opaque colour", name, got)
		}
	}
}

// A long report scrolls in <main>, under the title bar, in every theme: the page and
// its title bar stay put, and the last block clears the bottom of the window.
func TestReportScrolls(t *testing.T) {
	gui(t)
	defer setTheme(theme.Builtins.Default())
	var long []templ.Component
	for i := range 40 {
		long = append(long, components.Paragraph(fmt.Sprintf("Paragraph %d", i)))
	}
	for _, name := range theme.Builtins.Names() {
		setTheme(name)
		w := showReport(t, "report", "Long report", long)
		waitIn(t, w, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* `+name+`:"))`, "true")
		got := evalIn(t, w, `(() => {
			const main = document.querySelector("main"), bar = document.querySelector(".titlebar");
			const top = bar.getBoundingClientRect().top;
			main.scrollTop = main.scrollHeight;
			const last = main.querySelector("article").lastElementChild.getBoundingClientRect().bottom, end = main.getBoundingClientRect().bottom;
			return [main.scrollTop > 0, document.scrollingElement.scrollTop === 0,
			        bar.getBoundingClientRect().top === top, end - last >= 20,
			        main.getBoundingClientRect().top >= bar.getBoundingClientRect().bottom].join();
		})()`)
		if got != `"true,true,true,true,true"` {
			t.Errorf("%s: main scrolls, page still, title bar still, last block clear, main under the title bar = %s", name, got)
		}
	}
}

// The report page can move and close its own window and nothing else; the bar page
// can't do even that. A theme that draws its own buttons shows them, and they work.
func TestReportFrame(t *testing.T) {
	gui(t)
	defer setTheme(theme.Builtins.Default())
	if got := eval(t, `!!window.webkit.messageHandlers.frame`); got != "false" {
		t.Error("the bar page has the frame handler")
	}
	blocks := []templ.Component{components.Paragraph("Framed")}
	w := showReport(t, "report", "Framed report", blocks)
	waitIn(t, w, `document.querySelector(".titlebar span")?.textContent`, `"Framed report"`)
	if got := evalIn(t, w, `!!window.webkit.messageHandlers.frame`); got != "true" {
		t.Fatal("the report page has no frame handler")
	}
	// Anything but the four actions is dropped, and a drag needs the mouse held down.
	evalIn(t, w, `["open", {action: "close"}, 1, "drag"].forEach(m => window.webkit.messageHandlers.frame.postMessage(m)), true`)
	time.Sleep(200 * time.Millisecond)
	if !shim.WindowVisible(w) {
		t.Fatal("a message other than close closed the report window")
	}
	if got := evalIn(t, w, `getComputedStyle(document.querySelector(".titlebar .close")).display`); got != `"none"` {
		t.Errorf("the default theme shows its own close button (display %s)", got)
	}

	// Win 95 draws the whole window, buttons included; its close button closes it.
	setTheme("win95")
	w = showReport(t, "report", "Framed report", blocks)
	waitIn(t, w, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* win95:"))`, "true")
	for _, button := range []string{"close", "minimise", "zoom"} {
		if got := evalIn(t, w, `getComputedStyle(document.querySelector(".titlebar .`+button+`")).display`); got == `"none"` {
			t.Errorf("win95 hides its %s button", button)
		}
	}
	evalIn(t, w, `document.querySelector(".titlebar .close").click(), true`)
	eventually(t, "the report window to close", func() bool { return !shim.WindowVisible(w) })

	// Closed, it's gone: showing its id again opens a new window. Its minimise button
	// works without a title bar too, and showing it brings it back.
	closed := w
	eventually(t, "the closed window to be forgotten", func() bool { return testBar.reportWindow("report") != closed })
	w = showReport(t, "report", "Framed report", blocks)
	if w == closed {
		t.Errorf("showing a closed report's id again reused window %d", w)
	}
	eventually(t, "the report window to open", func() bool { return shim.WindowVisible(w) })
	waitIn(t, w, `document.querySelector(".titlebar .minimise") !== null`, "true")
	evalIn(t, w, `document.querySelector(".titlebar .minimise").click(), true`)
	eventually(t, "the report window to minimise", func() bool { return !shim.WindowVisible(w) })
	w = showReport(t, "report", "Framed report", blocks)
	eventually(t, "the report window to come back", func() bool { return shim.WindowVisible(w) })
}

// Each id has its own report window, opened down and to the right of the last;
// closing one leaves the others.
func TestReportWindowPerID(t *testing.T) {
	gui(t)
	a := showReport(t, "card-a", "Report A", []templ.Component{components.Paragraph("A")})
	b := showReport(t, "card-b", "Report B", []templ.Component{components.Paragraph("B")})
	if a == b || a == BarWindow || b == BarWindow {
		t.Fatalf("card-a and card-b have windows %d and %d", a, b)
	}
	waitIn(t, a, `document.querySelector("p")?.textContent`, `"A"`)
	waitIn(t, b, `document.querySelector("p")?.textContent`, `"B"`)
	if fa, fb := shim.WindowFrame(a), shim.WindowFrame(b); !(fb.X > fa.X && fb.Y < fa.Y) {
		t.Errorf("card-a's window is at %+v and card-b's at %+v, want b down and to the right", fa, fb)
	}

	evalIn(t, a, `window.webkit.messageHandlers.frame.postMessage("close"), true`)
	eventually(t, "card-a's window to close", func() bool { return !shim.WindowVisible(a) })
	if !shim.WindowVisible(b) {
		t.Error("closing card-a's window closed card-b's")
	}
	if _, err := shim.EvalResult(a, `1`); err == nil {
		t.Error("card-a's closed window still has its page")
	}
	evalIn(t, b, `window.webkit.messageHandlers.frame.postMessage("close"), true`)
	eventually(t, "card-b's window to close", func() bool { return !shim.WindowVisible(b) })
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
		{BarWindow + 1, "about:blank", false, true},
		{BarWindow + 1, "file:///etc/hosts", true, false},
		{BarWindow + 1, "https://example.com", false, false},
		{BarWindow + 1, "https://example.com", true, false}, // to OpenURL instead
		{BarWindow + 1, "javascript:alert(1)", true, false},
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
