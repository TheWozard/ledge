package components

import (
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// Each block shows its text escaped; a table's rows line up with its columns, and a
// link that isn't safe goes nowhere.
func TestBlocks(t *testing.T) {
	blocks := []templ.Component{
		Heading("Jobs"),
		Paragraph("<b>not bold</b>\nsecond line"),
		Fields([]Field{{Label: "Branch", Value: "main"}}),
		Table([]string{"Job", "Result"}, [][]string{{"lint", "ok"}, {"test"}, {"build", "ok", "extra"}}),
		List([]string{"one", "two"}, true),
		Code("$ go test\nFAIL"),
		Note("Flaky", "yellow"),
		Note("Plain", ""),
		Note("Purple", "purple"),
		Link("The run", "https://example.com/run"),
		Link("Bad", "javascript:alert(1)"),
	}
	var html strings.Builder
	for _, block := range blocks {
		html.WriteString(render(t, block))
	}
	contains(t, html.String(),
		"<h2>Jobs</h2>", "<p>&lt;b&gt;not bold&lt;/b&gt;\nsecond line</p>",
		`<div class="field"><dt>Branch</dt><dd>main</dd></div>`,
		"<th>Job</th><th>Result</th>",
		"<tr><td>test</td><td></td></tr>", "<tr><td>build</td><td>ok</td></tr>",
		"<ol><li>one</li><li>two</li></ol>", "<pre><code>$ go test\nFAIL</code></pre>",
		`data-color="yellow" style="--note: var(--c-yellow);"`, `data-color="gray" style="--note: var(--c-gray);">Plain`,
		`data-color="gray" style="--note: var(--c-gray);">Purple`,
		`<a href="https://example.com/run" title="https://example.com/run">The run</a>`)
	lacks(t, html.String(), "<b>not bold", "extra", `href="javascript:`)
}

// A block with an id is drawn as itself, its text escaped. ReplaceBlock replaces only
// the Block with its id, as a Block with it, and leaves the blocks it's given as they
// are; without one, nothing changes.
func TestBlock(t *testing.T) {
	if html := render(t, Block(`"x"`, Paragraph("<b>"))); html != `<p>&lt;b&gt;</p>` {
		t.Errorf("the block is %s", html)
	}
	blocks := []templ.Component{Heading("Top"), Block("x", Paragraph("Old")), Block("y", Paragraph("Kept"))}
	replaced, ok := ReplaceBlock(blocks, "x", Buttons([]Button{{ID: "go", Title: "Go", Click: nothing}}))
	if !ok {
		t.Fatal("ReplaceBlock didn't find x")
	}
	html, clicks := renderClicks(t, ReportContent(replaced))
	contains(t, html, "<h2>Top</h2>", `data-click="block/x/go"`, "<p>Kept</p>")
	lacks(t, html, "Old")
	if !slices.Equal(clicks.IDs(), []string{"block/x/go"}) {
		t.Errorf("replaced, the Clicks have %q", clicks.IDs())
	}
	if render(t, ReportContent(blocks)) != "<h2>Top</h2><p>Old</p><p>Kept</p>" {
		t.Error("ReplaceBlock changed the blocks it was given")
	}
	if same, ok := ReplaceBlock(blocks, "z", Paragraph("Lost")); ok || len(same) != len(blocks) {
		t.Error("ReplaceBlock replaced a Block that isn't there")
	}
	if _, ok := ReplaceBlock([]templ.Component{Paragraph("x")}, "", Paragraph("Lost")); ok {
		t.Error("ReplaceBlock replaced a block without an id")
	}
}

// A Buttons' button shows its title escaped, with its colour; one with a Click has
// the id ClickID gave it, and one without is disabled. Rendered with Clicks, the
// Clicks are put in them, and those in a Block are the block's, so ids can be the
// same in two.
func TestButtons(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	html, clicks := renderClicks(t, ReportContent([]templ.Component{
		Buttons([]Button{{ID: "rerun", Title: "<Rerun>", Color: "yellow", Click: click("rerun")}, {ID: "off", Title: "Off", Color: "purple"}}),
		Block("more", Buttons([]Button{{ID: "cancel", Title: "Cancel", Color: "#e0362c", Click: click("cancel")}})),
		Block("again", Buttons([]Button{{ID: "cancel", Title: "Cancel", Click: click("cancel again")}})),
	}))
	contains(t, html,
		`<div class="buttons"><button type="button" data-color="yellow" style="--button: var(--c-yellow);" class="button clickable" data-click="rerun"><span class="button-dot"></span>&lt;Rerun&gt;</button>`,
		`<button type="button" data-color="gray" style="--button: var(--c-gray);" class="button" disabled><span class="button-dot"></span>Off</button>`,
		`data-color="#e0362c" style="--button: #e0362c;" class="button clickable" data-click="block/more/cancel">`,
		`class="button clickable" data-click="block/again/cancel">`)
	lacks(t, html, "<Rerun>", `data-click="off"`)
	ids := []string{"block/again/cancel", "block/more/cancel", "rerun"}
	if !slices.Equal(clicks.IDs(), ids) {
		t.Fatalf("Clicks have %q, want %q", clicks.IDs(), ids)
	}
	for _, id := range ids {
		clicks.Click(id)(ClickEvent{})
	}
	if strings.Join(got, ",") != "cancel again,cancel,rerun" {
		t.Errorf("the buttons' Clicks called %v", got)
	}

	// Rendered without Clicks, a button has its id, and nothing takes its Click.
	contains(t, render(t, Buttons([]Button{{ID: "x", Title: "X", Click: click("x")}})), `data-click="x"`)
}
