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
	calls   recorder // what the handlers were called with
)

var defaults = Options{Edge: "bottom", Height: 110, Level: "normal"} // the zero Theme: the default

// recorder keeps the calls of the test Clicks and handlers: each Click's name, and
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
	if os.Getenv("LEDGE_GUI_TEST") == "" {
		fmt.Println("skipping the tests that open windows; set LEDGE_GUI_TEST=1 to run them")
		os.Exit(m.Run())
	}
	log.SetOutput(&logs)
	// The main loop has the main goroutine, so the tests run beside it.
	Init()
	var err error
	testBar, err = Open(defaults, Handlers{OpenURL: func(url string) { calls.add("url " + url) }})
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
	return map[string]theme.Theme{"frameless": frameless}
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

func TestReadMessage(t *testing.T) {
	for body, want := range map[string]string{
		`{"action": "ready"}`:                            "",
		`{"action": "click", "id": "a"}`:                 "",
		`{"action": "click"}`:                            "click needs an id",
		`{"action": "ready", "id": "a"}`:                 "ready takes no id",
		`{"action": "open", "id": "a"}`:                  `no action "open"`,
		`{"action": "click", "id": "a", "path": "/etc"}`: `unknown field "path"`,
		`"click"`:                           "cannot unmarshal string",
		`{"action": "click", "id": "a"} {}`: "more than one value",
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
}

func TestRender(t *testing.T) {
	gui(t)
	show(t, []components.Basic{
		{ID: "deploy", Title: "Deploy finished", Color: "#e0362c", Click: record("open deploy"),
			Actions: []components.Action{{ID: "aws", Title: "AWS", Color: "red", Click: record("run aws")}}},
		{ID: "note", Title: "<b>not bold</b>", Description: "plain text"},
	})
	waitFor(t, `[...document.querySelectorAll("li .title")].map(e => e.textContent).join("|")`, `"Deploy finished|<b>not bold</b>"`)
	for js, want := range map[string]string{
		`document.querySelector("li .title b") === null`:     "true",
		`document.querySelector("li").className`:             `"basic clickable"`,
		`document.querySelector("li").dataset.color`:         `"#e0362c"`,
		`document.querySelector(".action").closest("li").id`: `"card-deploy"`,
		// It arrived with its card, so isn't new itself.
		`document.querySelector(".action").getAttribute("style")`: `"--action: var(--c-red);"`,
		`document.querySelector(".action").dataset.color`:         `"red"`,
		`document.querySelector("a") === null`:                    "true",
		`document.querySelectorAll("li[data-new]").length > 0`:    "true",
	} {
		if got := eval(t, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
}

// Clicks post only ids, which reach the Clicks. An action with a Click is clicked
// apart from its card; one without is part of it. A click on what's gone is dropped.
func TestClicks(t *testing.T) {
	gui(t)
	show(t, []components.Basic{
		{ID: "card-open", Title: "Opens", Click: record("open card-open"), Dismiss: record("dismiss card-open"),
			Actions: []components.Action{
				{ID: "qa-run", Title: "Run", Click: record("run qa-run")},
				{ID: "qa-link", Title: "Link", Click: record("open qa-link")},
				{ID: "qa-plain", Title: "Plain"},
			}},
		{ID: "card-still", Title: "Still"},
	})
	waitFor(t, `document.querySelectorAll(".action").length`, "3")
	if got := eval(t, `document.querySelectorAll("#card-card-still .dismiss").length`); got != "0" {
		t.Error("a card without a Dismiss has ×")
	}
	eval(t, `document.getElementById("action-qa-run").click(), document.querySelector("#action-qa-link .label").click(), true`)
	for _, call := range []string{"run qa-run", "open qa-link"} {
		eventually(t, call, func() bool { return calls.has(call) })
	}
	time.Sleep(200 * time.Millisecond)
	if calls.has("open card-open") {
		t.Error("clicking an action also opened its card")
	}
	for _, js := range []string{
		`document.getElementById("action-qa-plain").click()`,
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Still")).click()`,
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Opens")).querySelector(".dismiss").click()`,
	} {
		eval(t, js+", true")
	}
	for _, call := range []string{"open card-open", "dismiss card-open"} {
		eventually(t, call, func() bool { return calls.has(call) })
	}
	time.Sleep(200 * time.Millisecond)
	for _, call := range []string{"open qa-plain", "open card-still", "open qa-run"} {
		if calls.has(call) {
			t.Errorf("clicking something that doesn't open called %q", call)
		}
	}

	// Once a render takes a card off, a click on it is dropped.
	before := logs.Len()
	testBar.Render(components.Cards([]components.Basic{{ID: "card-still", Title: "Still"}}))
	eval(t, `window.webkit.messageHandlers.ledge.postMessage({action: "click", id: "action-qa-run"}), true`)
	eventually(t, "the click to be dropped", func() bool { return strings.Contains(logs.String()[before:], "isn't on it") })
}

func TestMessagesAreChecked(t *testing.T) {
	gui(t)
	show(t, []components.Basic{{ID: "kept", Title: "Kept", Click: record("click kept")}})
	waitFor(t, `document.getElementById("card-kept") !== null`, "true")
	before := logs.Len()
	for _, message := range []string{`{action: "click"}`, `{action: "click", id: "card-kept", path: "/etc"}`, `{action: "exec", id: "card-kept"}`, `"click"`} {
		eval(t, `window.webkit.messageHandlers.ledge.postMessage(`+message+`), true`)
	}
	eventually(t, "4 refusals", func() bool { return strings.Count(logs.String()[before:], "the bar sent") == 4 })
	if calls.has("click kept") {
		t.Error("a refused message reached a Click")
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

// A render morphs the bar: cards and actions still there keep their elements,
// the row its scroll and a clicked action its flash, while what changed changes.
func TestRenderMorphs(t *testing.T) {
	gui(t)
	var cards []components.Basic
	for i := range 12 {
		cards = append(cards, components.Basic{ID: fmt.Sprintf("m-%02d", i), Title: fmt.Sprintf("Card %02d", i)})
	}
	cards[5].Actions = []components.Action{{ID: "run", Title: "Run", Click: func() {}}, {ID: "still", Title: "Still"}}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "12")
	eval(t, `window.kept = document.getElementById("card-m-05");
		window.keptAction = document.getElementById("action-still");
		document.getElementById("cards").scrollLeft = 300;
		document.getElementById("action-run").click(), true`)
	if got := eval(t, `document.getElementById("cards").scrollLeft`); got != "300" {
		t.Fatalf("the row scrolled to %s, want 300; the test needs more cards", got)
	}

	// The first card goes, one changes its title, and one arrives.
	changed := slices.Clone(cards[1:])
	changed[4].Title = "Renamed"
	changed = append(changed, components.Basic{ID: "m-new", Title: "New"})
	testBar.Render(components.Cards(changed))
	waitFor(t, `document.getElementById("card-m-new") !== null`, "true")
	for js, want := range map[string]string{
		`document.getElementById("card-m-05") === window.kept`:                             "true",
		`document.getElementById("action-still") === window.keptAction`:                    "true",
		`window.kept.querySelector(".title").textContent`:                                  `"Renamed"`,
		`document.getElementById("card-m-00").hasAttribute("data-leaving")`:                "true",
		`document.getElementById("cards").scrollLeft`:                                      "300",
		`document.getElementById("action-run").classList.contains("clicked")`:              "true",
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

// New cards and actions are marked new, and gone ones are marked leaving,
// told how far to collapse, and removed once they've left.
func TestEnterAndLeave(t *testing.T) {
	gui(t)
	a := components.Action{ID: "a", Title: "A"}
	b := components.Action{ID: "b", Title: "B"}
	c := components.Action{ID: "c", Title: "C"}
	x := components.Basic{ID: "x", Title: "X", Actions: []components.Action{a, b}}
	y := components.Basic{ID: "y", Title: "Y"}
	// leaving is whether the element is leaving and told how far to collapse.
	leaving := func(t *testing.T, id string) {
		t.Helper()
		got := eval(t, `(e => [e.hasAttribute("data-leaving"), parseFloat(e.style.getPropertyValue("--collapse-x")) < 0].join())(document.getElementById("`+id+`"))`)
		if got != `"true,true"` {
			t.Errorf("%s: leaving, collapsing = %s", id, got)
		}
	}
	withTheme(t, theme.Builtins.Default())
	show(t, []components.Basic{x, y})
	waitFor(t, `document.querySelectorAll("#cards > li").length`, "2")
	// Every card's new on a fresh page; its actions arrive with it.
	if got := eval(t, `[...document.querySelectorAll("li")].every(e => e.hasAttribute("data-new"))`); got != "true" {
		t.Errorf("not every card on a fresh page is new")
	}
	if got := eval(t, `document.querySelectorAll(".action[data-new]").length`); got != "0" {
		t.Errorf("%s actions are new apart from their cards", got)
	}

	// Action a goes from x, and c arrives on y, new.
	yc := y
	yc.Actions = []components.Action{c}
	xb := x
	xb.Actions = []components.Action{b}
	testBar.Render(components.Cards([]components.Basic{xb, yc}))
	waitFor(t, `document.getElementById("action-a")?.hasAttribute("data-leaving")`, "true")
	leaving(t, "action-a")
	if got := eval(t, `document.getElementById("action-c").hasAttribute("data-new")`); got != "true" {
		t.Error("an action new to a card isn't marked new")
	}
	waitFor(t, `document.getElementById("action-a") === null`, "true")

	// Card x goes, then the last card.
	testBar.Render(components.Cards([]components.Basic{yc}))
	waitFor(t, `document.getElementById("card-x")?.hasAttribute("data-leaving")`, "true")
	leaving(t, "card-x")
	waitFor(t, `document.getElementById("card-x") === null`, "true")
	testBar.Render(nil)
	waitFor(t, `document.getElementById("card-y")?.hasAttribute("data-leaving")`, "true")
	waitFor(t, `document.getElementById("card-y") === null`, "true")
}

// A card that comes back while it's leaving stays.
func TestComesBackWhileLeaving(t *testing.T) {
	gui(t)
	cards := []components.Basic{{ID: "back", Title: "Back"}, {ID: "other", Title: "Other"}}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "2")
	eval(t, `window.back = document.getElementById("card-back"), true`)
	testBar.Render(components.Cards(cards[1:]))
	waitFor(t, `window.back.hasAttribute("data-leaving")`, "true")
	testBar.Render(components.Cards(cards))
	waitFor(t, `window.back.hasAttribute("data-leaving")`, "false")
	time.Sleep(3500 * time.Millisecond) // past the longest a leaving card is kept
	if got := eval(t, `document.getElementById("card-back") === window.back && window.back.isConnected`); got != "true" {
		t.Error("a card that came back was removed anyway")
	}
}

func idsOf(cards []components.Basic) []string {
	var ids []string
	for _, c := range cards {
		ids = append(ids, "card-"+c.ID)
	}
	return ids
}

// Each theme loads and shows the cards.
func TestThemes(t *testing.T) {
	gui(t)
	show(t, []components.Basic{{ID: "c", Title: "Themed", Description: "Described", Color: "#e0362c",
		Actions: []components.Action{{ID: "q", Title: "Quick", Color: "red"}}}})
	for _, name := range themes(t) {
		withTheme(t, name)
		waitFor(t, `document.querySelector("li .action[data-color=red]") !== null`, "true")
	}
}

// The mouse wheel over something in a card that scrolls vertically, such as a long
// description, is left to scroll it rather than the row.
func TestWheelOverScrolling(t *testing.T) {
	gui(t)
	cards := []components.Basic{{ID: "long", Title: "Long", Description: strings.Repeat("A long description. ", 40)}}
	for i := range 8 {
		cards = append(cards, components.Basic{ID: fmt.Sprintf("wide-%d", i), Title: fmt.Sprintf("Card %d", i)})
	}
	show(t, cards)
	waitFor(t, `document.querySelector("li .description") !== null`, "true")

	// Made to scroll here, so it doesn't depend on how a theme lays the card out.
	const description = `document.querySelector("li .description")`
	eval(t, `(d => (d.style.height = "20px", d.style.overflowY = "auto", true))(`+description+`)`)
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
	return testBar.reportWindow(id)
}

func TestReportWindow(t *testing.T) {
	gui(t)
	w := showReport(t, "report", "A report", reportBlocks)
	waitIn(t, w, `document.querySelector(".titlebar span")?.textContent`, `"A report"`)
	for js, want := range map[string]string{
		`[...document.querySelectorAll("main > article > *")].map(e => e.tagName).join(" ")`: `"H2 P DL DIV OL PRE DIV P"`,
		`document.querySelector("p").textContent`:                                            `"<b>not bold</b>\nsecond line"`,
		`document.querySelector("p b") === null`:                                             "true",
		`[...document.querySelectorAll("dd")].map(e => e.textContent).join()`:                `"main,2"`,
		`[...document.querySelectorAll("tbody tr")].map(r => r.children.length).join()`:      `"2,2,2"`,
		`document.querySelector(".note").dataset.color`:                                      `"yellow"`,
		// <main> has the keys.
		`document.activeElement.tagName`: `"MAIN"`,
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

// Each theme loads in a report.
func TestReportThemes(t *testing.T) {
	gui(t)
	defer setTheme(theme.Builtins.Default())
	for _, name := range themes(t) {
		setTheme(name)
		w := showReport(t, "report", "Themed report", reportBlocks)
		// Each report stylesheet starts with a comment naming its theme.
		waitIn(t, w, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* `+name+`:"))`, "true")
		waitIn(t, w, `document.querySelectorAll("main > article > *").length`, fmt.Sprint(len(reportBlocks)))
	}
}

// The report page can move and close its own window and nothing else; the bar page
// can't do even that. A frameless theme's buttons, drawn by the page, work.
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

	// In a frameless theme, the page's close button closes the window.
	setTheme("frameless")
	w = showReport(t, "report", "Framed report", blocks)
	waitIn(t, w, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* frameless:"))`, "true")
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

// The page counts a timer down by itself, without another Render, and marks its card
// done once it's up.
func TestTimerCountsDown(t *testing.T) {
	gui(t)
	start := time.Now()
	testBar.Render([]components.Card{components.Timer{ID: "egg", Title: "Egg", Start: start, End: start.Add(1500 * time.Millisecond)}})
	t.Cleanup(func() { testBar.Render(nil) })
	waitFor(t, `document.querySelector("#card-egg .time-left")?.textContent`, `"0:01"`)
	if got := eval(t, `document.getElementById("card-egg").hasAttribute("data-done")`); got != "false" {
		t.Error("the timer is done before it's up")
	}
	waitFor(t, `(c => c.querySelector(".time-left").textContent + " " + c.hasAttribute("data-done"))(document.getElementById("card-egg"))`, `"0:00 true"`)
}

// A card that ends has its Complete called once its end passes, once for that end
// however many renders show it, and not at all if it's gone by then. It needs no
// window: a bar whose page isn't ready keeps what it's given.
func TestComplete(t *testing.T) {
	bar := &Bar{}
	var mu sync.Mutex
	counts := map[string]int{}
	timer := func(id string, end time.Time) components.Timer {
		return components.Timer{ID: id, Title: id, Start: time.Now(), End: end,
			Complete: func() { mu.Lock(); counts[id]++; mu.Unlock() }}
	}
	count := func(id string) int { mu.Lock(); defer mu.Unlock(); return counts[id] }

	soon, gone, past := timer("soon", time.Now().Add(100*time.Millisecond)), timer("gone", time.Now().Add(100*time.Millisecond)), timer("past", time.Now().Add(-time.Second))
	bar.Render([]components.Card{soon, gone, past})
	eventually(t, "past to complete at once", func() bool { return count("past") == 1 })
	// Rendering again before soon ends doesn't make it complete twice, and gone's left.
	bar.Render([]components.Card{soon, past})
	eventually(t, "soon to complete", func() bool { return count("soon") == 1 })
	bar.Render([]components.Card{soon, past})
	time.Sleep(200 * time.Millisecond)
	if count("soon") != 1 || count("past") != 1 || count("gone") != 0 {
		t.Errorf("completed %v, want soon and past once and gone not at all", counts)
	}
	// A new end is a new completion.
	bar.Render([]components.Card{timer("soon", time.Now())})
	eventually(t, "soon to complete at its new end", func() bool { return count("soon") == 2 })
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
