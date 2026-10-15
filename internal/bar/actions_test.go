package bar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ledge/internal/config"
	"ledge/internal/pages"
	"ledge/internal/shim"
)

// clickCard clicks the card with this title, once the bar shows it.
func clickCard(t *testing.T, title string) {
	t.Helper()
	find := `[...document.querySelectorAll("li")].find(li => li.querySelector(".title").textContent === ` + jsString(title) + `)`
	testBar.Render()
	waitFor(t, `!!`+find, "true")
	eval(t, find+`.click(), true`)
}

// clickPill clicks the label of the pill with this title (the pill itself for one
// with a command).
func clickPill(t *testing.T, title string) {
	t.Helper()
	find := `[...document.querySelectorAll(".pill .label")].find(l => l.textContent === ` + jsString(title) + `)`
	testBar.Render()
	waitFor(t, `!!`+find, "true")
	eval(t, find+`.click(), true`)
}

func TestCardOpensURL(t *testing.T) {
	add(t, `{"id": "card-url", "title": "Card with a url", "url": "file:///tmp/ledge-test-page.html"}`)
	clickCard(t, "Card with a url")
	eventually(t, "open file:///tmp/ledge-test-page.html", func() bool { return opened.has("file:///tmp/ledge-test-page.html") })
}

func TestPillOpensOnlyWebURLs(t *testing.T) {
	add(t, `{"kind": "status", "id": "pill-web", "title": "Web pill", "url": "https://example.com/status"}`)
	clickPill(t, "Web pill")
	eventually(t, "open https://example.com/status", func() bool { return opened.has("https://example.com/status") })

	// The page doesn't make a non-http(s) pill url clickable, and Go doesn't open one
	// even when asked.
	add(t, `{"kind": "status", "id": "pill-file", "title": "File pill", "url": "file:///etc/hosts"}`)
	testBar.handle([]byte(`{"action": "open", "id": "pill-file"}`))
	if opened.has("file:///etc/hosts") {
		t.Error("opened a pill's file:// url")
	}
}

func TestPillRunsCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	add(t, `{"kind": "status", "id": "pill-run", "title": "Run pill", "command": "echo ran in $PWD; touch `+marker+`"}`)
	clickPill(t, "Run pill")
	eventually(t, "the command to run", func() bool { _, err := os.Stat(marker); return err == nil })

	home, _ := os.UserHomeDir()
	eventually(t, "the command's output in commands.log", func() bool {
		data, _ := os.ReadFile(filepath.Join(testDir, "commands.log"))
		return strings.Contains(string(data), "Run pill (pill-run): echo ran in") &&
			strings.Contains(string(data), "ran in "+home+"\n")
	})
}

// reportJSON is a report with one of each block, its text partly HTML-looking.
const reportJSON = `[
	{"type": "heading", "text": "Summary"},
	{"type": "text", "text": "<b>not bold</b>\nsecond line"},
	{"type": "fields", "items": [{"label": "Branch", "value": "main"}, {"label": "Failures", "value": 2}]},
	{"type": "table", "columns": ["Job", "Result"], "rows": [["lint", "ok"], ["test"], ["build", "ok", "extra"]]},
	{"type": "list", "items": ["one", "two"], "ordered": true},
	{"type": "code", "text": "$ go test\nFAIL"},
	{"type": "note", "text": "Flaky", "color": "yellow"},
	{"type": "link", "text": "The run", "url": "https://example.com/from-report"}
]`

func TestReportWindow(t *testing.T) {
	add(t, `{"id": "card-report", "title": "Card with a report", "report": `+reportJSON+`}`)
	clickCard(t, "Card with a report")

	waitIn(t, ReportWindow, `document.querySelector(".titlebar")?.textContent`, `"Card with a report"`)
	for js, want := range map[string]string{
		`[...document.querySelectorAll("main > *")].map(e => e.tagName).join(" ")`: `"H2 P DL DIV OL PRE DIV P"`,
		`document.querySelector("p").textContent`:                                  `"<b>not bold</b>\nsecond line"`,
		`document.querySelector("p b") === null`:                                   "true",
		`[...document.querySelectorAll("dd")].map(e => e.textContent).join()`:      `"main,2"`,
		// Rows are padded or cut to the columns.
		`[...document.querySelectorAll("tbody tr")].map(r => r.children.length).join()`: `"2,2,2"`,
		`document.querySelector(".note").dataset.color`:                                 `"yellow"`,
		`getComputedStyle(document.documentElement).backgroundColor.startsWith("rgb(")`: "true",
		// The page's title bar is fixed across the top, under the window's buttons.
		`(s => s.position + " " + s.top)(getComputedStyle(document.querySelector(".titlebar")))`: `"fixed 0px"`,
	} {
		if got := evalIn(t, ReportWindow, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	// The page can't post to ledge.
	if got := evalIn(t, ReportWindow, `!(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.ledge)`); got != "true" {
		t.Error("the report page has ledge's message handler")
	}

	// The link opens in the browser, and the report stays put.
	evalIn(t, ReportWindow, `document.querySelector("a").click(), true`)
	eventually(t, "open https://example.com/from-report", func() bool { return opened.has("https://example.com/from-report") })
	if got := evalIn(t, ReportWindow, `location.href`); got != `"about:blank"` {
		t.Errorf("the link moved the report to %s", got)
	}

	// Sending the event again changes its report.
	add(t, `{"id": "card-report", "title": "Card with a report", "report": [{"type": "text", "text": "Second version"}]}`)
	clickCard(t, "Card with a report")
	waitIn(t, ReportWindow, `document.querySelector("p")?.textContent`, `"Second version"`)
}

// Every theme draws a report on an opaque background.
func TestReportThemes(t *testing.T) {
	defer func() { testBar.config = config.Defaults }()
	add(t, `{"id": "card-themed-report", "title": "Themed report", "report": `+reportJSON+`}`)
	for _, name := range pages.Themes {
		testBar.config.Theme = name
		clickCard(t, "Themed report")
		// Each report stylesheet starts with a comment naming its theme.
		waitIn(t, ReportWindow, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* `+name+`:"))`, "true")
		if got := evalIn(t, ReportWindow, `getComputedStyle(document.documentElement).backgroundColor`); !strings.HasPrefix(got, `"rgb(`) {
			t.Errorf("%s: report background = %s, want an opaque colour", name, got)
		}
	}
}

func TestNavigate(t *testing.T) {
	for _, c := range []struct {
		window  int
		url     string
		clicked bool
		want    bool
	}{
		{Window, "about:blank", false, true},
		{Window, "https://example.com", true, false},
		{ReportWindow, "about:blank", false, true},
		{ReportWindow, "file:///etc/hosts", true, false},
		{ReportWindow, "https://example.com", false, false},
		{ReportWindow, "https://example.com", true, false}, // opened in the browser instead
		{ReportWindow, "javascript:alert(1)", true, false},
	} {
		if got := testBar.navigate(c.window, c.url, c.clicked); got != c.want {
			t.Errorf("navigate(%d, %s, %v) = %v, want %v", c.window, c.url, c.clicked, got, c.want)
		}
	}
}

// The report page can move and close its own window and nothing else; the bar page
// can't do even that. A theme that draws its own buttons shows them, and they work.
func TestReportFrame(t *testing.T) {
	defer func() { testBar.config = config.Defaults }()
	add(t, `{"id": "card-frame", "title": "Framed report", "report": [{"type": "text", "text": "Framed"}]}`)
	if got := eval(t, `!!window.webkit.messageHandlers.frame`); got != "false" {
		t.Error("the bar page has the frame handler")
	}

	clickCard(t, "Framed report")
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
	testBar.config.Theme = "win95"
	clickCard(t, "Framed report")
	waitIn(t, ReportWindow, `[...document.querySelectorAll("style")].some(s => s.textContent.includes("/* win95:"))`, "true")
	for _, button := range []string{"close", "minimise", "zoom"} {
		if got := evalIn(t, ReportWindow, `getComputedStyle(document.querySelector(".titlebar .`+button+`")).display`); got == `"none"` {
			t.Errorf("win95 hides its %s button", button)
		}
	}
	evalIn(t, ReportWindow, `document.querySelector(".titlebar .close").click(), true`)
	eventually(t, "the report window to close", func() bool { return !shim.WindowVisible(ReportWindow) })

	// Its minimise button works without a title bar too, and the card brings it back.
	clickCard(t, "Framed report")
	eventually(t, "the report window to open", func() bool { return shim.WindowVisible(ReportWindow) })
	evalIn(t, ReportWindow, `document.querySelector(".titlebar .minimise").click(), true`)
	eventually(t, "the report window to minimise", func() bool { return !shim.WindowVisible(ReportWindow) })
	clickCard(t, "Framed report")
	eventually(t, "the report window to come back", func() bool { return shim.WindowVisible(ReportWindow) })
}
