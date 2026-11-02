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
	if got := evalIn(t, window, `[...document.querySelectorAll("main > article > *")].map(e => e.tagName).join(" ")`); got != `"H2 P DL DIV OL DIV DIV BLOCKQUOTE P DIV IMG DETAILS HR"` {
		t.Errorf("report blocks = %s", got)
	}

	// A button in a table cell reaches ledge, which opens its url.
	evalIn(t, window, `[...document.querySelectorAll("td button")].find(b => b.textContent === "Open").click(), true`)
	eventually(t, "open https://example.com/open", func() bool { return opened.has("https://example.com/open") })

	// Sending the event again changes its report.
	add(t, `{"id": "card-report", "title": "Card with a report", "report": [{"type": "text", "text": "Second version"}]}`)
	click(t, cardJS("Card with a report"))
	waitIn(t, window, `document.querySelector("p")?.textContent`, `"Second version"`)

	// A run block's button runs its command, and its output reaches the window.
	add(t, `{"id": "card-report", "title": "Card with a report", "report": [{"type": "run", "command": "echo from the run"}]}`)
	click(t, cardJS("Card with a report"))
	waitIn(t, window, `document.querySelector(".run [data-click]") != null`, "true")
	evalIn(t, window, `document.querySelector(".run [data-click]").click(), true`)
	waitIn(t, window, `document.querySelector(".run pre")?.textContent`, `"from the run\n"`)
	waitIn(t, window, `document.querySelector(".run")?.dataset.state`, `"passed"`)

	// Choosing in a select, or clicking a checkbox, keeps it with the event, and the
	// report shows it.
	add(t, `{"id": "card-report", "title": "Card with a report", "report": [{"type": "table", "columns": ["Job", "Outcome", ""],
		"rows": [["build", {"type": "select", "id": "build", "options": ["pending", "passed", "failed"]},
			{"type": "checkbox", "id": "muted", "label": "Muted"}]]}]}`)
	click(t, cardJS("Card with a report"))
	waitIn(t, window, `document.querySelector("td .select")?.textContent`, `"pending"`)
	evalIn(t, window, `document.querySelector("td .select").click(), true`)
	waitIn(t, ui.MenuWindow, `document.querySelectorAll(".menu-item").length`, "3")
	evalIn(t, ui.MenuWindow, `document.querySelectorAll(".menu-item")[2].click(), true`)
	eventually(t, "build's outcome kept", func() bool {
		event, _ := testStore.Get("card-report")
		return event.Chosen["build"] == "failed"
	})
	waitIn(t, window, `document.querySelector("td .select")?.textContent`, `"failed"`)
	evalIn(t, window, `document.querySelector("td .checkbox").click(), true`)
	eventually(t, "muted kept", func() bool {
		event, _ := testStore.Get("card-report")
		return event.Chosen["muted"] == true
	})
	waitIn(t, window, `document.querySelector("td .checkbox")?.getAttribute("aria-checked")`, `"true"`)
}
