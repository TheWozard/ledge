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

func TestCardRunsCommand(t *testing.T) {
	gui(t)
	marker := filepath.Join(t.TempDir(), "ran")
	add(t, `{"id": "card-run", "title": "Card with a command", "command": "echo ran in $PWD; touch `+marker+`"}`)
	click(t, cardJS("Card with a command"))
	eventually(t, "the command to run", func() bool { _, err := os.Stat(marker); return err == nil })

	home, _ := os.UserHomeDir()
	eventually(t, "the command's output in commands.log", func() bool {
		data, _ := os.ReadFile(filepath.Join(testDir, "commands.log"))
		return strings.Contains(string(data), "Card with a command (card-run): echo ran in") &&
			strings.Contains(string(data), "ran in "+home+"\n")
	})
}

// A card's JSON report reaches the report window. Its links are the frontend's to open.
func TestReportWindow(t *testing.T) {
	gui(t)
	// The first report window opened is the one after the menu's; this is the only test that opens one.
	const window = ui.MenuWindow + 1
	add(t, `{"id": "card-report", "title": "Card with a report", "report": `+reportJSON+`}`)
	click(t, cardJS("Card with a report"))
	waitIn(t, window, `document.querySelector(".titlebar span")?.textContent`, `"Card with a report"`)
	if got := evalIn(t, window, `[...document.querySelectorAll("main > article > *")].map(e => e.tagName).join(" ")`); got != `"H2 P DL DIV OL PRE DIV P"` {
		t.Errorf("report blocks = %s", got)
	}

	// Sending the event again changes its report.
	add(t, `{"id": "card-report", "title": "Card with a report", "report": [{"type": "text", "text": "Second version"}]}`)
	click(t, cardJS("Card with a report"))
	waitIn(t, window, `document.querySelector("p")?.textContent`, `"Second version"`)
}
