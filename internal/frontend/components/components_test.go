package components

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var out strings.Builder
	if err := c.Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func contains(t *testing.T, html string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in:\n%s", want, html)
		}
	}
}

func lacks(t *testing.T, html string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(html, u) {
			t.Errorf("unexpected %q in:\n%s", u, html)
		}
	}
}

var now = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func TestCard(t *testing.T) {
	cards := []Card{
		{ID: "deploy", Title: "<b>Deploy</b>", Description: "done", Source: "ci", Received: now.Add(-2 * time.Minute),
			Tags: []string{"prod"}, Priority: "high", Opens: true, Hint: "https://example.com"},
		{ID: "plain", Title: "Plain", Source: "manual", Received: now},
	}
	html := render(t, BarContent(nil, cards, now, map[string]int{"card-plain": 0}))
	contains(t, html,
		`class="high opens"`, `data-action="open" data-id="deploy"`, `title="https://example.com"`,
		`&lt;b&gt;Deploy&lt;/b&gt;`, `<div class="description">done</div>`,
		`<span class="when">ci · 2m</span>`, `<span class="when">manual · now</span>`,
		`<span class="tag">prod</span>`,
		`data-action="dismiss" data-id="deploy"`,
		`data-new style="--i: 0;"`)
	// With no pills, their column is there but empty, to be hidden.
	contains(t, html, `<div class="pills" id="pills"></div>`)
	lacks(t, html, "<b>Deploy", "No events.")
	// The plain card neither opens nor is marked new twice.
	if strings.Count(html, "data-new") != 1 || strings.Count(html, `data-action="open"`) != 1 {
		t.Errorf("wrong cards open or are new:\n%s", html)
	}
	contains(t, render(t, BarContent(nil, nil, now, nil)), `<ul id="cards"></ul><div class="empty">No events.</div>`)
}

func TestPill(t *testing.T) {
	pills := []Pill{
		{ID: "aws", Title: "AWS", Color: "green", Command: "aws sso login", Detail: "Session",
			Schedule: []Step{{At: now.Add(-time.Minute), Color: "red"}, {At: now.Add(time.Hour), Color: "yellow"}}},
		{ID: "ci", Title: "CI", Color: "#9b59b6", Link: true},
		{ID: "bad", Title: "Bad", Color: "red; background: url(x)"},
	}
	html := render(t, BarContent(pills, nil, now, map[string]int{"pill-ci": 2}))
	contains(t, html,
		// A new pill is marked, with its place among the new ones beside its colour.
		`data-new style="--pill: #9b59b6; --i: 2;"`,
		// The step that has passed sets the colour; the one to come is in the tooltip.
		`class="pill runs"`, `--pill: var(--c-red);`, `data-color="red"`, `data-action="run" data-id="aws"`,
		"Session\nClick to run: aws sso login\nyellow at",
		`--pill: #9b59b6;`, `<span class="label" data-action="open" data-id="ci">CI</span>`,
		// A colour that isn't one shows as gray, and can't add CSS.
		`style="--pill: var(--c-gray);" data-color="red; background: url(x)"`)
	if strings.Count(html, "data-new") != 1 {
		t.Errorf("only the new pill should be marked new:\n%s", html)
	}

	if c, ok := pills[0].NextChange(now); !ok || !c.Equal(now.Add(time.Hour)) {
		t.Errorf("NextChange = %v, %v", c, ok)
	}
	if _, ok := pills[1].NextChange(now); ok {
		t.Error("a pill with no schedule has a next change")
	}
}

func TestAgo(t *testing.T) {
	for minutes, want := range map[float64]string{0: "now", 0.4: "now", 5: "5m", 59: "59m", 90: "2h", 60 * 23: "23h", 60 * 24 * 3: "3d"} {
		if got := ago(now.Add(-time.Duration(minutes*float64(time.Minute))), now); got != want {
			t.Errorf("ago(%v minutes) = %s, want %s", minutes, got, want)
		}
	}
}

func TestBarPage(t *testing.T) {
	html := render(t, BarPage(":root { --bar: #fff; }", "top"))
	contains(t, html, `<body class="top">`, "<style>\n:root { --bar: #fff; }</style>", `<div id="app"></div>`,
		// The layout and the script come through as written.
		"html, body { margin: 0;", `send("ready");`, `event.target.closest("[data-action]")`,
		// Idiomorph is inlined whole, and update morphs with it.
		"<script>var Idiomorph=", `Idiomorph.morph(app, html, {`)
}

// Pills, cards and what holds them have stable ids, for update to morph each into
// itself.
func TestBarContentIDs(t *testing.T) {
	html := render(t, BarContent([]Pill{{ID: "aws", Title: "AWS"}}, []Card{{ID: "ci", Title: "CI", Received: now}}, now, nil))
	contains(t, html, `<div class="pills" id="pills">`, `id="pill-aws"`, `<ul id="cards">`, `id="card-ci"`)
}

func TestReportPage(t *testing.T) {
	blocks := []templ.Component{
		Heading("Jobs"),
		Paragraph("<b>not bold</b>\nsecond line"),
		Fields([]Field{{Label: "Branch", Value: "main"}}),
		Table([]string{"Job", "Result"}, [][]string{{"lint", "ok"}, {"test"}, {"build", "ok", "extra"}}),
		List([]string{"one", "two"}, true),
		Code("$ go test\nFAIL"),
		Note("Flaky", "yellow"),
		Note("Plain", ""),
		Link("The run", "https://example.com/run"),
		Link("Bad", "javascript:alert(1)"),
	}
	html := render(t, ReportPage(":root { --page: #fff; }", "CI <failed>", blocks))
	contains(t, html,
		`<span>CI &lt;failed&gt;</span>`, `<button class="close"`,
		"<h2>Jobs</h2>", "<p>&lt;b&gt;not bold&lt;/b&gt;\nsecond line</p>",
		`<div class="field"><dt>Branch</dt><dd>main</dd></div>`,
		"<th>Job</th><th>Result</th>",
		"<tr><td>test</td><td></td></tr>", "<tr><td>build</td><td>ok</td></tr>",
		"<ol><li>one</li><li>two</li></ol>", "<pre><code>$ go test\nFAIL</code></pre>",
		`data-color="yellow" style="--note: var(--c-yellow);"`, `data-color="gray" style="--note: var(--c-gray);"`,
		`<a href="https://example.com/run" title="https://example.com/run">The run</a>`,
		`frame("drag")`)
	lacks(t, html, "<b>not bold", "extra", `href="javascript:`)
}
