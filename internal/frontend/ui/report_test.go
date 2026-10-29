package ui

import (
	"fmt"
	"slices"
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
	components.Heading(2, components.Text("Summary")),
	components.Paragraph(components.Text("<b>not bold</b>\nsecond line")),
	components.Fields([]components.Field{{Label: "Branch", Value: components.Text("main")}, {Label: "Failures", Value: components.Text("2")}}),
	components.Table([]string{"Job", "Result"}, [][]components.Inline{{components.Text("lint"), components.Text("ok")}, {components.Text("test")},
		{components.Text("build"), components.Text("ok"), components.Text("extra")}}),
	components.List(true, components.Item{Text: components.Text("one")}, components.Item{Text: components.Text("two")}),
	components.Code("$ go test\nFAIL"),
	components.Note("yellow", components.Text("Flaky")),
	components.Paragraph(components.LinkTo("https://example.com/from-report", components.Text("The run"))),
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
	// The link opens in the browser, and the report stays put.
	evalIn(t, w, `document.querySelector("a").click(), true`)
	eventually(t, "the link to open", func() bool { return calls.has("url https://example.com/from-report") })
	if got := evalIn(t, w, `location.href`); got != `"about:blank"` {
		t.Errorf("the link moved the report to %s", got)
	}
	// Showing its id again replaces it, in the same window, and the same page.
	evalIn(t, w, `window.kept = true`)
	if again := showReport(t, "report", "Another", []templ.Component{components.Paragraph(components.Text("Second version"))}); again != w {
		t.Errorf("showing the same id again opened window %d, not %d", again, w)
	}
	waitIn(t, w, `document.querySelector("p")?.textContent`, `"Second version"`)
	for js, want := range map[string]string{
		`document.querySelector(".titlebar span").textContent`:   `"Another"`,
		`document.querySelectorAll("main > article > *").length`: "1",
		`window.kept === true`:                                   "true",
	} {
		if got := evalIn(t, w, js); got != want {
			t.Errorf("shown again, %s = %s, want %s", js, got, want)
		}
	}
}

// The report's Content-Security-Policy stops a script that gets into it, a <script> or
// an onerror, though the page's own and the app's calls run.
func TestReportCSP(t *testing.T) {
	gui(t)
	w := showReport(t, "csp", "CSP", []templ.Component{components.Paragraph(components.Text("Kept"))})
	waitIn(t, w, `document.querySelector("p")?.textContent`, `"Kept"`)
	evalIn(t, w, `(() => {
		window.blocked = 0;
		document.addEventListener("securitypolicyviolation", () => window.blocked++);
		const article = document.querySelector("article");
		const s = document.createElement("script");
		s.textContent = "window.ran = 'script'";
		article.append(s);
		article.insertAdjacentHTML("beforeend", '<img src="x:" onerror="window.ran = \'onerror\'">');
		return true;
	})()`)
	waitIn(t, w, `window.blocked >= 2`, "true")
	if got := evalIn(t, w, `window.ran ?? "nothing"`); got != `"nothing"` {
		t.Errorf("a script that got into the report ran: %s", got)
	}
}

// A block with an id is drawn as itself, and ShowReportBlock replaces only it, leaving
// the rest of the report's elements and its scroll. A report that isn't open, or a
// block it hasn't, is left alone.
func TestReportBlock(t *testing.T) {
	gui(t)
	blocks := []templ.Component{components.Heading(2, components.Text("Top"))}
	for i := range 60 {
		blocks = append(blocks, components.Paragraph(components.Text(fmt.Sprint("Filler ", i))))
	}
	blocks = append(blocks, components.Block{ID: "status", Content: components.Note("blue", components.Text("Running"))}, components.Paragraph(components.Text("After")))
	w := showReport(t, "blocks", "Blocks", blocks)
	waitIn(t, w, `document.querySelectorAll("main > article > *").length`, fmt.Sprint(len(blocks)))
	evalIn(t, w, `window.kept = document.querySelector("h2");
		document.querySelector("main").scrollTop = 200, true`)
	if got := evalIn(t, w, `document.querySelector("main").scrollTop`); got != "200" {
		t.Fatalf("the report scrolled to %s, want 200; the test needs more blocks", got)
	}

	open, err := testBar.ShowReportBlock("blocks", components.Block{ID: "status", Content: components.Paragraph(components.Text("<b>Done</b>"))})
	if err != nil || !open {
		t.Fatalf("ShowReportBlock = %v, %v", open, err)
	}
	const status = `[...document.querySelectorAll("main > article > *")].at(-2)`
	waitIn(t, w, status+`.tagName`, `"P"`)
	for js, want := range map[string]string{
		status + `.textContent`:                                              `"<b>Done</b>"`,
		`document.querySelectorAll(".note, p b").length`:                     "0",
		`document.querySelector("main > article > :last-child").textContent`: `"After"`,
		`document.querySelector("h2") === window.kept`:                       "true",
		`document.querySelector("main").scrollTop`:                           "200",
	} {
		if got := evalIn(t, w, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}

	if open, err := testBar.ShowReportBlock("blocks", components.Block{ID: "missing", Content: components.Paragraph(components.Text("Lost"))}); open || err != nil {
		t.Errorf("ShowReportBlock of a block the report hasn't = %v, %v", open, err)
	}
	if got := evalIn(t, w, `document.querySelectorAll("main > article > *").length`); got != fmt.Sprint(len(blocks)) {
		t.Errorf("%s blocks after replacing a missing one, want %d", got, len(blocks))
	}
	if open, _ := testBar.ShowReportBlock("not-open", components.Block{ID: "status", Content: components.Paragraph(components.Text("Lost"))}); open {
		t.Error("ShowReportBlock found a report that isn't open")
	}
}

// A report's buttons call their Clicks, with how they were clicked, busy while they
// run; a disabled one calls nothing. A block replaced with ShowReportBlock brings its
// own.
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
		components.Block{ID: "more", Content: components.Buttons([]components.Button{
			{ID: "slow", Title: "Slow", Click: func(components.ClickEvent) { calls.add("slow"); <-release }},
		})},
	})
	waitIn(t, w, `document.querySelectorAll(".button").length`, "3")
	button := func(id string) string { return `document.querySelector('[data-click="` + id + `"]')` }

	evalIn(t, w, button("go")+`.dispatchEvent(new MouseEvent("click", {bubbles: true, metaKey: true})), true`)
	eventually(t, "go", func() bool { return calls.has("go") })
	if got, want := calls.event("go"), (components.ClickEvent{Cmd: true}); got != want {
		t.Errorf("a ⌘-click on Go was %+v, want %+v", got, want)
	}
	evalIn(t, w, `document.querySelector(".button[disabled]").click(), true`)

	// A slow one is busy until it's done.
	evalIn(t, w, button("block/more/slow")+`.click(), true`)
	waitIn(t, w, button("block/more/slow")+`.hasAttribute("data-busy")`, "true")
	close(release)
	waitIn(t, w, button("block/more/slow")+`.hasAttribute("data-busy")`, "false")

	// Replacing the block brings its buttons: stop works.
	if open, err := testBar.ShowReportBlock("buttons", components.Block{ID: "more", Content: components.Buttons([]components.Button{
		{ID: "stop", Title: "Stop", Click: record("stop")},
	})}); !open || err != nil {
		t.Fatalf("ShowReportBlock = %v, %v", open, err)
	}
	waitIn(t, w, button("block/more/stop")+` !== null`, "true")
	evalIn(t, w, button("block/more/stop")+`.dispatchEvent(new MouseEvent("auxclick", {bubbles: true, button: 1})), true`)
	eventually(t, "stop", func() bool { return calls.has("stop") })
	if got, want := calls.event("stop"), (components.ClickEvent{Middle: true}); got != want {
		t.Errorf("a middle click on Stop was %+v, want %+v", got, want)
	}
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
	blocks := []templ.Component{components.Paragraph(components.Text("Framed"))}
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
	eventually(t, "the closed window to be forgotten", func() bool { _, open := reportWindow("report"); return !open })
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
	a := showReport(t, "card-a", "Report A", []templ.Component{components.Paragraph(components.Text("A"))})
	b := showReport(t, "card-b", "Report B", []templ.Component{components.Paragraph(components.Text("B"))})
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

// Each report has a page of its own, in a window after the menu's, kept for its id
// until its window closes; then the id gets a new one, and the closed window's page
// is gone. The bar's page is never closed.
func TestReportPages(t *testing.T) {
	b := newBar(Options{}, &fakeNative{})
	a, other := b.report("a"), b.report("b")
	if a.window <= MenuWindow || other.window == a.window || b.report("a") != a {
		t.Fatalf("a is in window %d, b in %d, and a again in %d", a.window, other.window, b.report("a").window)
	}
	b.closed(a.window)
	b.closed(BarWindow)
	if b.pages[a.window] != nil || b.pages[BarWindow] != b.bar || b.report("b") != other {
		t.Error("closing a window forgot the wrong pages")
	}
	if again := b.report("a"); again == a || again.window <= other.window {
		t.Errorf("a, shown again once closed, is in window %d, after %d", again.window, other.window)
	}
}

// A window only ever shows its page. A link clicked in a report opens in the browser
// if it's http(s), and nothing else does.
func TestNavigate(t *testing.T) {
	f := &fakeNative{}
	bar := newBar(Options{}, f)
	report := bar.report("r").window
	for _, c := range []struct {
		window  int
		url     string
		clicked bool
		want    bool
	}{
		{BarWindow, "about:blank", false, true},
		{BarWindow, "https://example.com", true, false},
		{MenuWindow, "https://example.com", true, false},
		{report, "about:blank", false, true},
		{report, "file:///etc/hosts", true, false},
		{report, "https://example.com", false, false},
		{report, "https://example.com/opened", true, false}, // to the browser instead
		{report, "javascript:alert(1)", true, false},
	} {
		if got := bar.navigate(c.window, c.url, c.clicked); got != c.want {
			t.Errorf("navigate(%d, %s, %v) = %v, want %v", c.window, c.url, c.clicked, got, c.want)
		}
	}
	eventually(t, "the link to open", func() bool { return len(f.opened()) > 0 })
	if time.Sleep(50 * time.Millisecond); !slices.Equal(f.opened(), []string{"https://example.com/opened"}) {
		t.Errorf("opened %q, want only the clicked report link", f.opened())
	}
}

// A report's page is loaded and its window shown when it's first shown, and what's
// shown in it, and done to it, waits for the page to be ready. Shown again, the page
// is given what it shows, not loaded again, unless the theme has changed. Its clicks
// reach only its own Clicks, as they are now, not the bar's nor another window's, until
// it's closed; a block replaced with ShowReportBlock brings its own in place of the
// old, and one the report hasn't is left alone.
func TestReportMessages(t *testing.T) {
	b, f := fakeBar(t, theme.Theme{Report: "/* first */"})
	b.Render(components.Cards([]components.Basic{{ID: "fbar", Title: "Bar", Click: record("fake open fbar")}}))
	if err := b.ShowReport("fr", "Fake <report>", []templ.Component{
		components.Buttons([]components.Button{{ID: "go", Title: "Go", Click: record("fake report go")}}),
		components.Block{ID: "more", Content: components.Buttons([]components.Button{{ID: "old", Title: "Old", Click: record("fake report old")}})},
	}); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	w := b.reports["fr"].window
	b.mu.Unlock()
	if !strings.Contains(f.page(w), "/* first */") || f.loadCount(w) != 1 || !f.showing(w) {
		t.Fatalf("the report's window, loaded %d times, is showing %v, with %s", f.loadCount(w), f.showing(w), f.page(w))
	}
	if open, err := b.ShowReportBlock("fr", components.Block{ID: "more", Content: components.Buttons([]components.Button{
		{ID: "new", Title: "New", Click: record("fake report new")},
	})}); !open || err != nil {
		t.Fatalf("ShowReportBlock = %v, %v", open, err)
	}
	if open, err := b.ShowReportBlock("fr", components.Block{ID: "missing", Content: components.Paragraph(components.Text("Lost"))}); open || err != nil {
		t.Errorf("ShowReportBlock of a block the report hasn't = %v, %v", open, err)
	}
	if ran := f.ran(w); len(ran) != 0 {
		t.Fatalf("before its page was ready, the report ran %q", ran)
	}
	post(b, w, `{"action": "ready"}`)
	ran := f.ran(w)
	if len(ran) != 2 || !strings.HasPrefix(ran[0], `update("Fake \u003creport\u003e", `) || !strings.Contains(ran[0], `data-click=\"block/more/old\"`) ||
		!strings.Contains(ran[1], `data-click=\"block/more/new\"`) || strings.Contains(ran[1], "old") || !strings.HasSuffix(ran[1], "; setBusy([])") {
		t.Errorf("once ready, the report ran %q, want it shown, then with its block replaced, each with the busy marks", ran)
	}

	for _, id := range []string{"go", "block/more/new", "block/more/old", "card/fbar/click"} {
		post(b, w, `{"action": "click", "id": "`+id+`"}`)
	}
	post(b, BarWindow, `{"action": "click", "id": "go"}`)
	if !calls.has("fake report go") || !calls.has("fake report new") || calls.has("fake report old") || calls.has("fake open fbar") {
		t.Error("the report's clicks didn't reach only its own Clicks, as they are now")
	}
	if open, _ := b.ShowReportBlock("fake-not-open", components.Block{ID: "more", Content: components.Paragraph(components.Text("Lost"))}); open {
		t.Error("ShowReportBlock found a report that isn't open")
	}

	// Shown again while one of its buttons runs, it's given what to show, with that
	// button busy, without loading its page again.
	release, done := make(chan bool), make(chan bool)
	blocks := []templ.Component{components.Buttons([]components.Button{{ID: "slow", Title: "Slow", Click: func(components.ClickEvent) { <-release }}})}
	if err := b.ShowReport("fr", "Fake report", blocks); err != nil {
		t.Fatal(err)
	}
	go func() { post(b, w, `{"action": "click", "id": "slow"}`); done <- true }()
	eventually(t, "slow to be busy", func() bool { return f.last(w) == `setBusy(["slow"])` })
	if err := b.ShowReport("fr", "Fake report", blocks); err != nil {
		t.Fatal(err)
	}
	if got := f.last(w); !strings.HasPrefix(got, "update(") || !strings.HasSuffix(got, `; setBusy(["slow"])`) || f.loadCount(w) != 1 {
		t.Errorf("shown again while slow runs, the report, loaded %d times, ran %s", f.loadCount(w), got)
	}
	close(release)
	<-done
	if got := f.last(w); got != "setBusy([])" {
		t.Errorf("once slow is done, the report ran %s", got)
	}

	// In another theme, its page is loaded again, in that theme, and given what to
	// show once it's ready.
	b.mu.Lock()
	b.options.Theme.Report = "/* second */"
	b.mu.Unlock()
	if err := b.ShowReport("fr", "Fake report", blocks); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.page(w), "/* second */") || f.loadCount(w) != 2 || len(f.ran(w)) != 0 {
		t.Errorf("in another theme, the report, loaded %d times, ran %q before it was ready", f.loadCount(w), f.ran(w))
	}
	post(b, w, `{"action": "ready"}`)
	if ran := f.ran(w); len(ran) == 0 || !strings.HasPrefix(ran[0], "update(") {
		t.Errorf("once ready in another theme, the report ran %q", ran)
	}

	b.closed(w)
	post(b, w, `{"action": "click", "id": "go"}`)
	if n := calls.count("fake report go"); n != 1 {
		t.Errorf("Go ran %d times, want only the once before its report closed", n)
	}
}
