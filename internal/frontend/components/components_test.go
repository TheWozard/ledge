package components

import (
	"context"
	"fmt"
	"io"
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

// nothing is a Click for a test that only renders.
func nothing() {}

func TestCard(t *testing.T) {
	cards := []Basic{
		{ID: "deploy", Title: "<b>Deploy</b>", Description: "done",
			Color: "#e0362c", Width: 240, Click: nothing, Dismiss: nothing},
		{ID: "plain", Title: "Plain", Color: "red; background: url(x)"},
		{ID: "new", Title: "New", Color: "#2f9e5b"},
	}
	html := render(t, BarContent(Cards(cards), map[string]int{"card-plain": 0, "card-new": 1}))
	contains(t, html,
		`<li id="card-deploy" class="basic clickable" data-click="card-deploy" data-color="#e0362c" style="--card-color: #e0362c; --card-width: 240px;">`,
		`<span class="title">Plain</span>`,
		// A colour that isn't #rrggbb is none.
		`<li id="card-plain" class="basic" data-new style="--i: 0;">`,
		`<li id="card-new" class="basic" data-color="#2f9e5b" data-new style="--card-color: #2f9e5b; --i: 1;">`,
		`&lt;b&gt;Deploy&lt;/b&gt;`, `<div class="description">done</div>`,
		// × shows only on a card that can be dismissed.
		`<button class="dismiss clickable" title="Dismiss" data-click="dismiss-deploy">×</button>`)
	// With no actions, their row is there but empty, to be hidden.
	contains(t, html, `<div class="actions"></div>`)
	lacks(t, html, "<b>Deploy", "No events.")
	lacks(t, html, "url(x)")
	// Only the deploy card is clicked or dismissed, and only the new ones are new.
	if strings.Count(html, "data-new") != 2 || strings.Count(html, "data-click") != 2 || strings.Count(html, `class="dismiss`) != 1 {
		t.Errorf("wrong cards are clickable, dismissable or new:\n%s", html)
	}
	contains(t, render(t, BarContent(nil, nil)), `<ul id="cards"></ul><div class="empty">No events.</div>`)
}

// Clicks has each Click on the cards by the id the page posts for it, and nothing
// that isn't clickable.
func TestClicks(t *testing.T) {
	var got []string
	click := func(name string) Click { return func() { got = append(got, name) } }
	cards := []Basic{
		{ID: "a", Click: click("a"), Dismiss: click("dismiss a"),
			Actions: []Action{{ID: "q", Click: click("q")}, {ID: "none"}}},
		{ID: "b"},
	}
	clicks := Clicks(Cards(cards))
	html := render(t, BarContent(Cards(cards), nil))
	for _, id := range []string{"card-a", "dismiss-a", "action-q"} {
		if !strings.Contains(html, `data-click="`+id+`"`) {
			t.Errorf("the page has no data-click %q", id)
		}
		clicks[id]()
	}
	if len(clicks) != 3 || strings.Join(got, ",") != "a,dismiss a,q" {
		t.Errorf("Clicks = %d of them, calling %v", len(clicks), got)
	}
}

// progress is a kind of card for TestKinds: how far along something is.
type progress struct {
	id, kind string
	done     int
}

func (p progress) Frame() Frame {
	return Frame{ID: p.id, Kind: p.kind, Actions: []Action{{ID: p.id + "-stop", Title: "Stop", Click: nothing}}}
}

func (p progress) Body() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := fmt.Fprintf(w, `<div class="done">%d%%</div>`, p.done)
		return err
	})
}

// Cards of any kinds share the frame: each kind's body sits in it, before the
// actions, and its kind is a class on it, if it's a class name.
func TestKinds(t *testing.T) {
	cards := []Card{
		Basic{ID: "basic", Title: "Basic"},
		progress{id: "build", kind: "progress", done: 40},
		progress{id: "odd", kind: "Not a class", done: 10},
	}
	html := render(t, BarContent(cards, nil))
	contains(t, html,
		`<li id="card-basic" class="basic">`,
		`<li id="card-build" class="progress"><div class="done">40%</div><div class="actions"><div id="action-build-stop"`,
		`<li id="card-odd" class=""><div class="done">10%</div>`)
	if clicks := Clicks(cards); clicks["action-build-stop"] == nil || len(clicks) != 2 {
		t.Errorf("the progress cards' actions' Clicks = %v", clicks)
	}
}

// A timer is a card of its own kind: its title, and the time it has left, with its
// start and end for the page to count it down. Its frame ends when it does.
func TestTimer(t *testing.T) {
	start := time.Now()
	timer := Timer{ID: "tea", Title: "<b>Tea</b>", Start: start, End: start.Add(90*time.Second + 500*time.Millisecond),
		Color: "#3b82f6", Click: nothing, Dismiss: nothing, Complete: nothing}
	html := render(t, BarContent([]Card{timer}, nil))
	contains(t, html,
		`<li id="card-tea" class="timer clickable" data-click="card-tea" data-color="#3b82f6"`,
		`<span class="title">&lt;b&gt;Tea&lt;/b&gt;</span>`,
		fmt.Sprintf(`<div class="time-left" data-start="%d" data-end="%d">1:31</div>`, start.UnixMilli(), timer.End.UnixMilli()),
		`data-click="dismiss-tea"`)
	if f := timer.Frame(); !f.Ends.Equal(timer.End) || f.Complete == nil {
		t.Errorf("the timer's frame ends %v, with Complete %v", f.Ends, f.Complete != nil)
	}
	if clicks := Clicks([]Card{timer}); len(clicks) != 2 {
		t.Errorf("the timer has %d Clicks, want its own and its ×'s", len(clicks))
	}
}

func TestTimeLeft(t *testing.T) {
	for d, want := range map[time.Duration]string{
		-time.Second: "0:00", 0: "0:00", time.Millisecond: "0:01", 59 * time.Second: "0:59",
		61 * time.Second: "1:01", 10*time.Minute + 5*time.Second: "10:05", time.Hour + 2*time.Minute + 3*time.Second: "1:02:03",
	} {
		if got := timeLeft(d); got != want {
			t.Errorf("timeLeft(%v) = %s, want %s", d, got, want)
		}
	}
}

// Cards sort highest priority first, those of the same priority keeping their order.
func TestSortByPriority(t *testing.T) {
	cards := Cards([]Basic{{ID: "a"}, {ID: "b", Priority: 2}, {ID: "c", Priority: -1}, {ID: "d", Priority: 2}, {ID: "e"}})
	SortByPriority(cards)
	var ids []string
	for _, c := range cards {
		ids = append(ids, c.Frame().ID)
	}
	if got := strings.Join(ids, ""); got != "bdaec" {
		t.Errorf("sorted %s, want bdaec", got)
	}
}

func TestAction(t *testing.T) {
	actions := []Action{
		{ID: "aws", Title: "AWS", Color: "green", Click: nothing},
		{ID: "ci", Title: "CI", Color: "#9b59b6", Click: nothing},
		{ID: "bad", Title: "Bad", Color: "red; background: url(x)"},
	}
	cards := []Basic{{ID: "card", Title: "Card", Description: "Desc", Click: nothing, Actions: actions}}
	html := render(t, BarContent(Cards(cards), map[string]int{"action-ci": 2}))
	contains(t, html,
		// They're last in the card, after its title and description.
		`<span class="title">Card</span> <div class="description">Desc</div><div class="actions"><div id="action-aws"`,
		`<span class="label">Bad</span></div></div></li>`,
		// A new action is marked, with its place among the new ones beside its colour.
		`data-new style="--action: #9b59b6; --i: 2;"`,
		// A named colour is the theme's.
		`class="action clickable" style="--action: var(--c-green);" data-color="green" data-click="action-aws"`,
		// One with a Click is itself the button.
		`data-click="action-ci"><span class="label">CI</span>`,
		`<div id="action-bad" class="action" style`,
		// A colour that isn't one shows as gray, and can't add CSS.
		`style="--action: var(--c-gray);" data-color="red; background: url(x)"`)
	lacks(t, html, " else", "title=")
	if strings.Count(html, "data-new") != 1 {
		t.Errorf("only the new action should be marked new:\n%s", html)
	}
	// One without a Click is left to its card: only the card, aws and ci are clicked.
	if strings.Count(html, "data-click") != 3 {
		t.Errorf("wrong things are clickable:\n%s", html)
	}
}

func TestBarPage(t *testing.T) {
	html := render(t, BarPage(":root { --bar: #fff; }", "top"))
	contains(t, html, `<body class="top">`, "<style>\n:root { --bar: #fff; }</style>", `<div id="app"></div>`,
		// The layout and the script come through as written.
		"html, body { margin: 0;", `send("ready");`, `event.target.closest("[data-click]")`,
		// Idiomorph is inlined whole, and update morphs with it.
		"<script>var Idiomorph=", `Idiomorph.morph(app, html, {`)
}

// Cards, their actions and the row have stable ids, for update to morph each
// into itself.
func TestBarContentIDs(t *testing.T) {
	html := render(t, BarContent([]Card{Basic{ID: "ci", Title: "CI", Actions: []Action{{ID: "aws", Title: "AWS"}}}}, nil))
	contains(t, html, `<ul id="cards">`, `id="card-ci"`, `id="action-aws"`)
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
