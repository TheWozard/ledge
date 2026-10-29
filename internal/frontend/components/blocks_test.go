package components

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// Each block shows its text escaped, in its element and nothing else: a heading at
// its level, inline text in its own, a table's rows lined up with its columns, a
// list's items with lists of their own, and a link that isn't safe going nowhere.
func TestBlocks(t *testing.T) {
	blocks := []templ.Component{
		Heading(1, Text("Top")), Heading(2, Text("Jobs")), Heading(4, Text("Sub")), Heading(9, Text("Deep")), Heading(0, Text("Least")),
		Paragraph(Text("<b>not bold</b>\nsecond line "), Strong(Text("bold "), Em(Text("both"))), Text(" "),
			Mono("Render()"), Text(" "), LinkTo("https://example.com/run", Text("The run")), LinkTo("javascript:alert(1)", Text("Bad"))),
		Fields([]Field{{Label: "Branch", Value: Text("main")}, {Label: "Empty"}}),
		Table([]string{"Job", "Result"}, [][]Inline{{Text("lint"), Strong(Text("ok"))}, {Text("test")}, {Text("build"), Text("ok"), Text("extra")}}),
		List(true, Item{Text: Text("one"), Items: []Item{{Text: Text("one a")}}}, Item{Text: Text("two")}),
		Code("$ go test\nFAIL"),
		Note("yellow", Text("Flaky")), Note("", Text("Plain")), Note("purple", Text("Purple")),
		Quote(Text("<i>quoted</i>")),
		Rule(),
		Details(Text("More"), Paragraph(Text("Hidden"))),
	}
	var html strings.Builder
	for _, block := range blocks {
		html.WriteString(render(t, block))
	}
	contains(t, html.String(),
		"<h1>Top</h1>", "<h2>Jobs</h2>", "<h4>Sub</h4>", "<h6>Deep</h6>", "<h1>Least</h1>",
		"<p>&lt;b&gt;not bold&lt;/b&gt;\nsecond line <strong>bold <em>both</em></strong> <code>Render()</code> ",
		`<a href="https://example.com/run" title="https://example.com/run">The run</a>`,
		`<div class="field"><dt>Branch</dt><dd>main</dd></div><div class="field"><dt>Empty</dt><dd></dd></div>`,
		"<th>Job</th><th>Result</th>", "<tr><td>lint</td><td><strong>ok</strong></td></tr>",
		"<tr><td>test</td><td></td></tr>", "<tr><td>build</td><td>ok</td></tr>",
		"<ol><li>one<ul><li>one a</li></ul></li><li>two</li></ol>", "<div class=\"code\"><pre><code>$ go test\nFAIL</code></pre></div>",
		`data-color="yellow" style="--note: var(--c-yellow);">Flaky`, `data-color="gray" style="--note: var(--c-gray);">Plain`,
		`data-color="gray" style="--note: var(--c-gray);">Purple`,
		"<blockquote>&lt;i&gt;quoted&lt;/i&gt;</blockquote>", "<hr>",
		"<details><summary>More</summary> <p>Hidden</p></details>")
	lacks(t, html.String(), "<b>not bold", "<i>", "extra", `href="javascript:`)
}

// An image is shown only from a data: URL of an image; anything else, which the page
// would fetch, shows only its alt.
func TestImage(t *testing.T) {
	png := "data:image/png;base64,iVBORw0KGgo="
	contains(t, render(t, Image(png, "A <chart>")), `<img src="`+png+`" alt="A &lt;chart&gt;">`)
	for _, src := range []string{"https://example.com/x.png", "data:text/html;base64,PGI+", "javascript:alert(1)", ""} {
		if html := render(t, Image(src, "A chart")); html != `<p class="image-alt">A chart</p>` {
			t.Errorf("Image(%q) is %s", src, html)
		}
	}
}

// A block with an id is drawn as itself, its text escaped. ReplaceBlock replaces only
// the Block with its id, as a Block with it, and leaves the blocks it's given as they
// are; without one, nothing changes.
func TestBlock(t *testing.T) {
	if html := render(t, Block{ID: `"x"`, Content: Paragraph(Text("<b>"))}); html != `<p>&lt;b&gt;</p>` {
		t.Errorf("the block is %s", html)
	}
	blocks := []templ.Component{Heading(2, Text("Top")), Block{ID: "x", Content: Paragraph(Text("Old"))}, Block{ID: "y", Content: Paragraph(Text("Kept"))}}
	replaced, ok := ReplaceBlock(blocks, Block{ID: "x", Content: Buttons([]Button{{ID: "go", Title: "Go", Click: nothing}})})
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
	if same, ok := ReplaceBlock(blocks, Block{ID: "z", Content: Paragraph(Text("Lost"))}); ok || len(same) != len(blocks) {
		t.Error("ReplaceBlock replaced a Block that isn't there")
	}
	if _, ok := ReplaceBlock([]templ.Component{Paragraph(Text("x"))}, Block{ID: "", Content: Paragraph(Text("Lost"))}); ok {
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
		Block{ID: "more", Content: Buttons([]Button{{ID: "cancel", Title: "Cancel", Color: "#e0362c", Click: click("cancel")}})},
		Block{ID: "again", Content: Buttons([]Button{{ID: "cancel", Title: "Cancel", Click: click("cancel again")}})},
	}))
	contains(t, html,
		`<div class="buttons"><button type="button" data-color="yellow" style="--button: var(--c-yellow);" class="button" data-click="rerun"><span class="button-dot"></span>&lt;Rerun&gt;</button>`,
		`<button type="button" data-color="gray" style="--button: var(--c-gray);" class="button" disabled><span class="button-dot"></span>Off</button>`,
		`data-color="#e0362c" style="--button: #e0362c;" class="button" data-click="block/more/cancel">`,
		`class="button" data-click="block/again/cancel">`)
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

// A Code block rendered WithCopy has a button that copies its text, with an id from
// the text, in its Block's scope; without, it has none.
func TestCode(t *testing.T) {
	if html := render(t, Code("<b>log</b>")); html != `<div class="code"><pre><code>&lt;b&gt;log&lt;/b&gt;</code></pre></div>` {
		t.Errorf("without WithCopy, the block is %s", html)
	}
	var got []string
	clicks := &Clicks{}
	ctx := WithCopy(WithClicks(context.Background(), clicks), func(text string) { got = append(got, text) })
	var html strings.Builder
	blocks := ReportContent([]templ.Component{Code("<b>log</b>"), Block{ID: "x", Content: Code("other")}, Code("<b>log</b>")})
	if err := blocks.Render(ctx, &html); err != nil {
		t.Fatal(err)
	}
	id := copyID("<b>log</b>")
	contains(t, html.String(), `<pre><code>&lt;b&gt;log&lt;/b&gt;</code></pre><button type="button" class="copy" title="Copy" data-click="`+id+`">Copy</button></div>`,
		`data-click="block/x/`+copyID("other")+`"`)
	if ids := clicks.IDs(); !slices.Equal(ids, []string{"block/x/" + copyID("other"), id}) {
		t.Fatalf("the Clicks have %q", ids)
	}
	clicks.Click(id)(ClickEvent{})
	clicks.Click("block/x/" + copyID("other"))(ClickEvent{})
	if !slices.Equal(got, []string{"<b>log</b>", "other"}) {
		t.Errorf("the buttons copied %q", got)
	}
	// With a copy icon, the button shows it rather than "Copy".
	html.Reset()
	icon := "data:image/svg+xml;base64,PHN2Zy8+"
	if err := Code("log").Render(WithIcons(ctx, Icons{Copy: icon}), &html); err != nil {
		t.Fatal(err)
	}
	contains(t, html.String(), `title="Copy" data-click="`+copyID("log")+`"><span class="icon" aria-hidden="true"></span></button>`)
}

// A table's cells can be links, alone or among other text: each is an <a> with its
// url as its title, its text escaped, and one that isn't safe goes nowhere. A link in
// a cell past the columns is cut with it.
func TestTableLinks(t *testing.T) {
	html := render(t, Table([]string{"Job", "Run"}, [][]Inline{
		{Text("build"), LinkTo("https://example.com/runs/1", Text("#1"))},
		{LinkTo("https://example.com/jobs/lint", Mono("lint")), templ.Join(Text("see "), LinkTo("https://example.com/runs/2?a=1&b=<2>", Strong(Text("<#2>"))))},
		{Text("bad"), LinkTo("javascript:alert(1)", Text("run"))},
		{Text("cut"), Text("kept"), LinkTo("https://example.com/cut", Text("gone"))},
	}))
	contains(t, html,
		`<tr><td>build</td><td><a href="https://example.com/runs/1" title="https://example.com/runs/1">#1</a></td></tr>`,
		`<td><a href="https://example.com/jobs/lint" title="https://example.com/jobs/lint"><code>lint</code></a></td>`,
		`<td>see <a href="https://example.com/runs/2?a=1&amp;b=&lt;2&gt;" title="https://example.com/runs/2?a=1&amp;b=&lt;2&gt;"><strong>&lt;#2&gt;</strong></a></td>`,
		`<td><a href="about:invalid#TemplFailedSanitizationURL" title="javascript:alert(1)">run</a></td>`,
		"<tr><td>cut</td><td>kept</td></tr>")
	lacks(t, html, `href="javascript:`, "<#2>", "gone", "example.com/cut")
}
