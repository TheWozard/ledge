package components

import (
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// HTML keeps the markup a theme styles, and takes off everything else: scripts,
// styles, classes, ids, other attributes, links that aren't http(s), and images that
// aren't data: ones. Text is escaped, and its white space collapsed but in a <pre>.
func TestHTML(t *testing.T) {
	html := render(t, HTML(`<h3 id="x" class="big" style="color: red" onclick="alert(1)">Jobs</h3>
<p>Two
  <strong>failed</strong>, see <a href="https://example.com/run" target="_blank">the run</a>.</p>
<script>alert(1)</script><style>p { color: red }</style><iframe src="https://example.com"></iframe>
<p><a href="javascript:alert(1)">bad</a> <a href="file:///etc/passwd">file</a></p>
<pre><code>$ go test
  FAIL</code></pre>
<ol start="3"><li>one</li></ol>
<details open><summary>More</summary><p>&lt;b&gt;not bold&lt;/b&gt;</p></details>
<img src="https://example.com/x.png" alt="fetched"><img src="data:image/png;base64,iVBORw0KGgo=" alt="inline">
<button data-click="forged">Forged</button><div data-click="forged">div</div>`, nil))
	contains(t, html,
		"<h3>Jobs</h3>",
		`<p>Two <strong>failed</strong>, see <a href="https://example.com/run">the run</a>.</p>`,
		"<pre><code>$ go test\n  FAIL</code></pre>", `<ol start="3"><li>one</li></ol>`,
		`<details open=""><summary>More</summary><p>&lt;b&gt;not bold&lt;/b&gt;</p></details>`,
		`<img src="data:image/png;base64,iVBORw0KGgo=" alt="inline"/>`,
		`<button type="button" class="button" disabled="">Forged</button>`)
	lacks(t, html, "<script", "alert", "<style", "color", "<iframe", "big", `id=`, "_blank",
		"file:", "fetched", "forged", "<div")
}

// An <aside> is a note in its colour, as Note is, and a table is wrapped to scroll
// sideways, as Table is.
func TestHTMLNotesAndTables(t *testing.T) {
	html := render(t, HTML(`<aside data-color="red">Failed</aside><aside data-color="#9b59b6">Hex</aside>`+
		`<aside data-color="purple; background: red">Bad</aside><aside>None</aside>`+
		`<table><tr><th>Job</th></tr><tr><td colspan="2">build</td></tr></table>`, nil))
	contains(t, html,
		`<aside data-color="red" class="note" style="--note: var(--c-red)">Failed</aside>`,
		`<aside data-color="#9b59b6" class="note" style="--note: #9b59b6">Hex</aside>`,
		`<aside data-color="gray" class="note" style="--note: var(--c-gray)">Bad</aside>`,
		`<aside class="note" data-color="gray" style="--note: var(--c-gray)">None</aside>`,
		`<div class="table"><table><tbody><tr><th>Job</th></tr><tr><td colspan="2">build</td></tr></tbody></table></div>`)
	lacks(t, html, "background")
}

// A <button data-action> or a link to action: calls its action's Click, with the id
// ClickID gives it, scoped to its Block; one whose action has no Click is disabled.
func TestHTMLActions(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	actions := map[string]Click{"rerun": click("rerun"), "open": click("open"), "off": nil}
	html, clicks := renderClicks(t, ReportContent([]templ.Component{
		HTML(`<p><button data-action="rerun">Rerun</button><button data-action="off">Off</button>`+
			`<button data-action="missing">Missing</button><button>None</button></p>`+
			`<p><a href="action:open">Open</a> <a href="action:missing">Missing</a></p>`, actions),
		Block{ID: "more", Content: HTML(`<button data-action="rerun">Again</button>`, actions)},
	}))
	contains(t, html,
		`<button type="button" class="button" data-click="rerun">Rerun</button>`,
		`<button type="button" class="button" disabled="">Off</button>`,
		`<button type="button" class="button" disabled="">Missing</button>`,
		`<button type="button" class="button" disabled="">None</button>`,
		`<a data-click="open">Open</a>`, `<a aria-disabled="true">Missing</a>`,
		`<button type="button" class="button" data-click="block/more/rerun">Again</button>`)
	lacks(t, html, "data-action", "action:")
	ids := []string{"block/more/rerun", "open", "rerun"}
	if !slices.Equal(clicks.IDs(), ids) {
		t.Fatalf("Clicks have %q, want %q", clicks.IDs(), ids)
	}
	for _, id := range ids {
		clicks.Click(id)(ClickEvent{})
	}
	if strings.Join(got, ",") != "rerun,open,rerun" {
		t.Errorf("the actions' Clicks called %v", got)
	}
}
