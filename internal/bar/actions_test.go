package bar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// writeReport writes a report with an image beside it, links to try, and an image
// from outside its directory: testdata/outside.svg, in the repo. (Not a file beside the
// temp dir: WebKit's web process can read the user's whole temp directory anyway.)
func writeReport(t *testing.T, heading string) string {
	t.Helper()
	dir := t.TempDir()
	outside, err := filepath.Abs("testdata/outside.svg")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"dot.svg":      `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"><rect width="20" height="10" fill="red"/></svg>`,
		"details.html": `<!doctype html><title>Details</title><h1 id="details">Details</h1>`,
		"index.html": `<!doctype html><title>Report title</title><h1>` + heading + `</h1>
<img id="inside" src="dot.svg"><img id="outside" src="file://` + outside + `">
<a id="to-details" href="details.html">details</a>
<a id="to-hosts" href="file:///etc/hosts">hosts</a>
<a id="to-web" href="https://example.com/from-report">web</a>`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "index.html")
}

func TestReportWindow(t *testing.T) {
	path := writeReport(t, "First version")
	add(t, `{"id": "card-report", "title": "Card with a report", "report": "`+path+`"}`)
	clickCard(t, "Card with a report")

	waitIn(t, ReportWindow, `document.querySelector("h1")?.textContent`, `"First version"`)
	if got := evalIn(t, ReportWindow, `document.title`); got != `"Report title"` {
		t.Errorf("title = %s", got)
	}
	// It can read its own directory and nothing outside it.
	waitIn(t, ReportWindow, `document.getElementById("inside").complete && document.getElementById("outside").complete`, "true")
	if got := evalIn(t, ReportWindow, `[document.getElementById("inside").naturalWidth, document.getElementById("outside").naturalWidth]`); got != "[20,0]" {
		t.Errorf("image widths (inside, outside the directory) = %s, want [20,0]", got)
	}
	// And it can't post to ledge.
	if got := evalIn(t, ReportWindow, `!(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.ledge)`); got != "true" {
		t.Error("the report page has ledge's message handler")
	}

	// A link outside the directory goes nowhere; an https one opens in the browser.
	evalIn(t, ReportWindow, `document.getElementById("to-hosts").click(), true`)
	evalIn(t, ReportWindow, `document.getElementById("to-web").click(), true`)
	eventually(t, "open https://example.com/from-report", func() bool { return opened.has("https://example.com/from-report") })
	if got := evalIn(t, ReportWindow, `location.pathname.endsWith("/index.html")`); got != "true" {
		t.Errorf("a blocked link moved the report to %s", evalIn(t, ReportWindow, `location.href`))
	}

	// A link to a file beside it opens in the window.
	evalIn(t, ReportWindow, `document.getElementById("to-details").click(), true`)
	waitIn(t, ReportWindow, `document.getElementById("details")?.textContent`, `"Details"`)

	// Clicking the card again reads the file afresh.
	if err := os.WriteFile(path, []byte(`<!doctype html><title>Report title</title><h1>Second version</h1>`), 0o644); err != nil {
		t.Fatal(err)
	}
	clickCard(t, "Card with a report")
	waitIn(t, ReportWindow, `document.querySelector("h1")?.textContent`, `"Second version"`)
}

func TestMissingReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nowhere.html")
	add(t, `{"id": "card-missing", "title": "Card with a missing report", "report": "`+path+`"}`)
	clickCard(t, "Card with a missing report")
	waitIn(t, ReportWindow, `document.getElementById("path")?.textContent`, jsString(path))
	if got := evalIn(t, ReportWindow, `document.getElementById("reason").textContent`); got != `"There's no file there."` {
		t.Errorf("reason = %s", got)
	}
}

func TestNavigate(t *testing.T) {
	dir := t.TempDir()
	testBar.mu.Lock()
	testBar.reportDir = dir
	testBar.mu.Unlock()
	for _, c := range []struct {
		window int
		url    string
		want   bool
	}{
		{Window, "about:blank", true},
		{Window, "file://" + dir + "/a.html", false},
		{Window, "https://example.com", false},
		{ReportWindow, "file://" + dir + "/a.html", true},
		{ReportWindow, "file://" + dir + "/sub/b.html#part", true},
		{ReportWindow, "file://" + dir + "/../escape.html", false},
		{ReportWindow, "file://" + dir + "-sibling/a.html", false},
		{ReportWindow, "https://example.com", false},
		{ReportWindow, "javascript:alert(1)", false},
	} {
		if got := testBar.navigate(c.window, c.url, false); got != c.want {
			t.Errorf("navigate(%d, %s) = %v, want %v", c.window, c.url, got, c.want)
		}
	}
}
