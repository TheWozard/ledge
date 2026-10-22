package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

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
	} {
		if got := evalIn(t, w, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	// The link goes to openURL, and the report stays put.
	evalIn(t, w, `document.querySelector("a").click(), true`)
	eventually(t, "openURL", func() bool { return calls.has("url https://example.com/from-report") })
	if got := evalIn(t, w, `location.href`); got != `"about:blank"` {
		t.Errorf("the link moved the report to %s", got)
	}
	// Showing its id again replaces it, in the same window.
	if again := showReport(t, "report", "Another", []templ.Component{components.Paragraph("Second version")}); again != w {
		t.Errorf("showing the same id again opened window %d, not %d", again, w)
	}
	waitIn(t, w, `document.querySelector("p")?.textContent`, `"Second version"`)
}

// A block with an id is drawn as itself, and ShowReportBlock replaces only it, leaving
// the report's scroll. A report that isn't open, or a block it hasn't, is left alone.
func TestReportBlock(t *testing.T) {
	gui(t)
	blocks := []templ.Component{components.Heading("Top")}
	for i := range 60 {
		blocks = append(blocks, components.Paragraph(fmt.Sprint("Filler ", i)))
	}
	blocks = append(blocks, components.Block("status", components.Note("Running", "blue")), components.Paragraph("After"))
	w := showReport(t, "blocks", "Blocks", blocks)
	waitIn(t, w, `document.querySelector(".note")?.dataset.block`, `"status"`)
	evalIn(t, w, `window.kept = document.querySelector("h2");
		document.querySelector("main").scrollTop = 200, true`)
	if got := evalIn(t, w, `document.querySelector("main").scrollTop`); got != "200" {
		t.Fatalf("the report scrolled to %s, want 200; the test needs more blocks", got)
	}

	open, err := testBar.ShowReportBlock("blocks", "status", components.Paragraph("<b>Done</b>"))
	if err != nil || !open {
		t.Fatalf("ShowReportBlock = %v, %v", open, err)
	}
	waitIn(t, w, `document.querySelector("[data-block=status]")?.tagName`, `"P"`)
	for js, want := range map[string]string{
		`document.querySelector("[data-block=status]").textContent`:                 `"<b>Done</b>"`,
		`document.querySelectorAll("template, .note").length`:                       "0",
		`[...document.querySelectorAll("main > article > *")].at(-2).dataset.block`: `"status"`,
		`document.querySelector("main > article > :last-child").textContent`:        `"After"`,
		`document.querySelector("h2") === window.kept`:                              "true",
		`document.querySelector("main").scrollTop`:                                  "200",
	} {
		if got := evalIn(t, w, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}

	if _, err := testBar.ShowReportBlock("blocks", "missing", components.Paragraph("Lost")); err != nil {
		t.Fatal(err)
	}
	if got := evalIn(t, w, `document.querySelectorAll("main > article > *").length`); got != fmt.Sprint(len(blocks)) {
		t.Errorf("%s blocks after replacing a missing one, want %d", got, len(blocks))
	}
	if open, _ := testBar.ShowReportBlock("not-open", "status", components.Paragraph("Lost")); open {
		t.Error("ShowReportBlock found a report that isn't open")
	}
}

// A report's buttons call their Clicks, busy while they run, and nothing else: not a
// disabled one, not one a replaced block took out, and not a Click on the bar. A
// block replaced with ShowReportBlock brings its own.
func TestReportClicks(t *testing.T) {
	gui(t)
	release := make(chan bool)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	show(t, []components.Basic{{ID: "bar", Title: "Bar", Click: record("open bar")}})
	w := showReport(t, "buttons", "Buttons", []templ.Component{
		components.Buttons([]components.Button{
			{ID: "go", Title: "Go", Color: "green", Click: record("go")},
			{ID: "off", Title: "Off"},
		}),
		components.Block("more", components.Buttons([]components.Button{
			{ID: "slow", Title: "Slow", Click: func(components.ClickEvent) { calls.add("slow"); <-release }},
		})),
	})
	waitIn(t, w, `document.querySelectorAll(".button").length`, "3")
	button := func(id string) string { return `document.querySelector('[data-click="` + id + `"]')` }

	evalIn(t, w, button("go")+`.dispatchEvent(new MouseEvent("click", {bubbles: true, metaKey: true})), true`)
	eventually(t, "go", func() bool { return calls.has("go") })
	if got, want := calls.event("go"), (components.ClickEvent{Cmd: true}); got != want {
		t.Errorf("a ⌘-click on Go was %+v, want %+v", got, want)
	}
	evalIn(t, w, `document.querySelector(".button[disabled]").click(), true`)

	// A slow one is busy until it's done, and a click on it meanwhile is dropped.
	evalIn(t, w, button("slow")+`.click(), true`)
	waitIn(t, w, button("slow")+`.hasAttribute("data-busy")`, "true")
	before := logs.Len()
	evalIn(t, w, `window.webkit.messageHandlers.ledge.postMessage({action: "click", id: "slow"}), true`)
	eventually(t, "the click on slow to be dropped", func() bool { return strings.Contains(logs.String()[before:], "still running") })
	close(release)
	waitIn(t, w, button("slow")+`.hasAttribute("data-busy")`, "false")

	// The page can post only clicks, and only of its own Clicks.
	before = logs.Len()
	for _, message := range []string{
		`{action: "click", id: "card-bar"}`, `{action: "menu", id: "card-bar"}`, `{action: "click", id: "off"}`,
	} {
		evalIn(t, w, `window.webkit.messageHandlers.ledge.postMessage(`+message+`), true`)
	}
	eventually(t, "the messages to be refused", func() bool {
		return strings.Count(logs.String()[before:], "isn't in it") == 2 && strings.Contains(logs.String()[before:], `no action "menu"`)
	})

	// Replacing the block replaces its Clicks: slow is gone, and stop works.
	if open, err := testBar.ShowReportBlock("buttons", "more", components.Buttons([]components.Button{
		{ID: "stop", Title: "Stop", Click: record("stop")},
	})); !open || err != nil {
		t.Fatalf("ShowReportBlock = %v, %v", open, err)
	}
	waitIn(t, w, button("stop")+` !== null`, "true")
	evalIn(t, w, button("stop")+`.dispatchEvent(new MouseEvent("auxclick", {bubbles: true, button: 1})), true`)
	eventually(t, "stop", func() bool { return calls.has("stop") })
	if got, want := calls.event("stop"), (components.ClickEvent{Middle: true}); got != want {
		t.Errorf("a middle click on Stop was %+v, want %+v", got, want)
	}
	before = logs.Len()
	evalIn(t, w, `window.webkit.messageHandlers.ledge.postMessage({action: "click", id: "slow"}), true`)
	eventually(t, "the click on slow to be dropped", func() bool { return strings.Contains(logs.String()[before:], "isn't in it") })
	time.Sleep(200 * time.Millisecond)
	if calls.count("slow") != 1 || calls.has("open bar") || calls.count("go") != 1 {
		t.Errorf("the report's clicks called slow %d times, go %d, and the bar's %d; want 1, 1 and 0",
			calls.count("slow"), calls.count("go"), calls.count("open bar"))
	}
	evalIn(t, w, `window.webkit.messageHandlers.frame.postMessage("close"), true`)
	eventually(t, "the report window to close", func() bool { return !shim.WindowVisible(w) })
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
	eventually(t, "the closed window to be forgotten", func() bool { _, open := testBar.reports.open("report"); return !open })
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

// A report's Clicks are its own: replacing a block replaces only that block's, and
// closing it forgets them.
func TestReportWindowClicks(t *testing.T) {
	r := newReportWindows()
	nothing := func(components.ClickEvent) {}
	w := r.show("a", &components.ReportClicks{
		Clicks: map[string]components.Click{"top": nothing, "old": nothing},
		Blocks: map[string][]string{"block": {"old"}},
	})
	other := r.show("b", &components.ReportClicks{})
	r.replaceBlock(w, "block", &components.ReportClicks{
		Clicks: map[string]components.Click{"new": nothing},
		Blocks: map[string][]string{"block": {"new"}},
	})
	for id, want := range map[string]bool{"top": true, "old": false, "new": true} {
		if got := r.click(w, id) != nil; got != want {
			t.Errorf("a's %q is there: %v, want %v", id, got, want)
		}
	}
	if r.click(other, "top") != nil {
		t.Error("b has a's Click")
	}
	r.closed(w)
	if r.click(w, "top") != nil {
		t.Error("a closed still has its Clicks")
	}
}

func TestNavigate(t *testing.T) {
	bar := &Bar{}
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
		{BarWindow + 1, "https://example.com", true, false}, // to openURL instead
		{BarWindow + 1, "javascript:alert(1)", true, false},
	} {
		if got := bar.navigate(c.window, c.url, c.clicked); got != c.want {
			t.Errorf("navigate(%d, %s, %v) = %v, want %v", c.window, c.url, c.clicked, got, c.want)
		}
	}
}
