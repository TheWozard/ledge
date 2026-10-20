package components

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

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
		`style="--action: var(--c-gray);" data-color="gray"`)
	lacks(t, html, " else", "title=")
	if strings.Count(html, "data-new") != 1 {
		t.Errorf("only the new action should be marked new:\n%s", html)
	}
	// One without a Click is left to its card: only the card, aws and ci are clicked.
	if strings.Count(html, "data-click") != 3 {
		t.Errorf("wrong things are clickable:\n%s", html)
	}
}

// Cards, their actions and the row have stable ids, for update to morph each
// into itself.
func TestBarContentIDs(t *testing.T) {
	html := render(t, BarContent([]Card{Basic{ID: "ci", Title: "CI", Actions: []Action{{ID: "aws", Title: "AWS"}}}}, nil))
	contains(t, html, `<ul id="cards">`, `id="card-ci"`, `id="action-aws"`)
}
