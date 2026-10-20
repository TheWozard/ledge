package bar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheWozard/ledge-frontend/ui"
)

func TestCardOpensURL(t *testing.T) {
	gui(t)
	add(t, `{"id": "card-url", "title": "Card with a url", "url": "file:///tmp/ledge-test-page.html"}`)
	click(t, cardJS("Card with a url"))
	eventually(t, "open file:///tmp/ledge-test-page.html", func() bool { return opened.has("file:///tmp/ledge-test-page.html") })
}

func TestPillOpensOnlyWebURLs(t *testing.T) {
	gui(t)
	add(t, `{"kind": "status", "id": "pill-web", "title": "Web pill", "url": "https://example.com/status"}`)
	click(t, pillLabelJS("Web pill"))
	eventually(t, "open https://example.com/status", func() bool { return opened.has("https://example.com/status") })

	// The page doesn't make a non-http(s) pill url clickable, and ledge doesn't open
	// one even when asked.
	add(t, `{"kind": "status", "id": "pill-file", "title": "File pill", "url": "file:///etc/hosts"}`)
	testBar.open("pill-file")
	if opened.has("file:///etc/hosts") {
		t.Error("opened a pill's file:// url")
	}
}

func TestPillRunsCommand(t *testing.T) {
	gui(t)
	marker := filepath.Join(t.TempDir(), "ran")
	add(t, `{"kind": "status", "id": "pill-run", "title": "Run pill", "command": "echo ran in $PWD; touch `+marker+`"}`)
	click(t, `document.querySelector(".pill.runs")`)
	eventually(t, "the command to run", func() bool { _, err := os.Stat(marker); return err == nil })

	home, _ := os.UserHomeDir()
	eventually(t, "the command's output in commands.log", func() bool {
		data, _ := os.ReadFile(filepath.Join(testDir, "commands.log"))
		return strings.Contains(string(data), "Run pill (pill-run): echo ran in") &&
			strings.Contains(string(data), "ran in "+home+"\n")
	})
}

// A card's JSON report reaches the report window, and its link opens with `open`.
func TestReportWindow(t *testing.T) {
	gui(t)
	// The first report window opened is the one after the bar's; this is the only test that opens one.
	const window = ui.BarWindow + 1
	add(t, `{"id": "card-report", "title": "Card with a report", "report": `+reportJSON+`}`)
	click(t, cardJS("Card with a report"))
	waitIn(t, window, `document.querySelector(".titlebar span")?.textContent`, `"Card with a report"`)
	if got := evalIn(t, window, `[...document.querySelectorAll("main > article > *")].map(e => e.tagName).join(" ")`); got != `"H2 P DL DIV OL PRE DIV P"` {
		t.Errorf("report blocks = %s", got)
	}
	evalIn(t, window, `document.querySelector("a").click(), true`)
	eventually(t, "open https://example.com/from-report", func() bool { return opened.has("https://example.com/from-report") })

	// Sending the event again changes its report.
	add(t, `{"id": "card-report", "title": "Card with a report", "report": [{"type": "text", "text": "Second version"}]}`)
	click(t, cardJS("Card with a report"))
	waitIn(t, window, `document.querySelector("p")?.textContent`, `"Second version"`)
}
