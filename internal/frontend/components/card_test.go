package components

import (
	"context"
	"fmt"
	"io"
	"slices"
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
		`<div class="card-buttons"><button class="dismiss clickable" title="Dismiss" data-click="dismiss-deploy">×</button></div>`)
	// Without actions, a card has no menu.
	lacks(t, html, "<b>Deploy", "No events.", "data-menu", "actions")
	lacks(t, html, "url(x)")
	// Only the deploy card is clicked or dismissed, and only the new ones are new.
	if strings.Count(html, "data-new") != 2 || strings.Count(html, "data-click") != 2 || strings.Count(html, `class="dismiss`) != 1 {
		t.Errorf("wrong cards are clickable, dismissable or new:\n%s", html)
	}
	contains(t, render(t, BarContent(nil, nil)), `<ul id="cards"></ul><div class="empty">No events.</div>`)
}

// Clicks has each Click on the cards by its id: a card's and its ×'s, which the page
// posts, and its actions', which are chosen from its menu. Nothing that isn't
// clickable is in it.
func TestClicks(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	cards := []Basic{
		{ID: "a", Click: click("a"), Dismiss: click("dismiss a"),
			Actions: []Action{{ID: "q", Click: click("q")}, {ID: "none"}}},
		{ID: "b"},
	}
	clicks := Clicks(Cards(cards))
	html := render(t, BarContent(Cards(cards), nil))
	for _, id := range []string{"card-a", "dismiss-a"} {
		if !strings.Contains(html, `data-click="`+id+`"`) {
			t.Errorf("the page has no data-click %q", id)
		}
	}
	for _, id := range []string{"card-a", "dismiss-a", "action-q"} {
		clicks[id](ClickEvent{})
	}
	if len(clicks) != 3 || strings.Join(got, ",") != "a,dismiss a,q" {
		t.Errorf("Clicks = %d of them, calling %v", len(clicks), got)
	}
}

// A card's parts are in Clicks by their PartClickID, those with a Click, and no two
// cards' parts share an id, whatever their ids have in them.
func TestParts(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	cards := []Card{
		parts{id: "a-b", parts: []Part{{ID: "c", Click: click("a-b c")}, {ID: "none"}}},
		parts{id: "a", parts: []Part{{ID: "b-c", Click: click("a b-c")}}},
	}
	clicks := Clicks(cards)
	if len(clicks) != 2 {
		t.Fatalf("Clicks = %v", clicks)
	}
	for _, id := range []string{"part-3-a-b-c", "part-1-a-b-c"} {
		clicks[id](ClickEvent{})
	}
	if strings.Join(got, ",") != "a-b c,a b-c" {
		t.Errorf("the parts' Clicks called %v", got)
	}
}

// parts is a kind of card for TestParts: nothing but its parts.
type parts struct {
	id    string
	parts []Part
}

func (p parts) Frame() Frame          { return Frame{ID: p.id, Parts: p.parts} }
func (p parts) Body() templ.Component { return templ.NopComponent }

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

// Cards of any kinds share the frame: each kind's body sits in it, after its
// buttons, and its kind is a class on it, if it's a class name.
func TestKinds(t *testing.T) {
	cards := []Card{
		Basic{ID: "basic", Title: "Basic"},
		progress{id: "build", kind: "progress", done: 40},
		progress{id: "odd", kind: "Not a class", done: 10},
	}
	html := render(t, BarContent(cards, nil))
	contains(t, html,
		`<li id="card-basic" class="basic">`,
		`<li id="card-build" class="progress" data-menu="card-build"><div class="card-buttons"><button class="menu clickable" title="Actions" data-menu="card-build">⋯</button> </div><div class="done">40%</div></li>`,
		`<li id="card-odd" class="" data-menu="card-odd">`)
	if clicks := Clicks(cards); clicks["action-build-stop"] == nil || len(clicks) != 2 {
		t.Errorf("the progress cards' actions' Clicks = %v", clicks)
	}
}

// A card's actions are its menu, not on the page: the card has data-menu, and a ⋯
// before its × to ask for the menu too. Its Menu is an item for each action, in
// order: its title, its colour as the page would name it, whether it can be
// chosen, and the id of its Click.
func TestMenu(t *testing.T) {
	card := Basic{ID: "card", Title: "Card", Description: "Desc", Click: nothing, Dismiss: nothing, Actions: []Action{
		{ID: "aws", Title: "AWS", Color: "green", Click: nothing},
		{ID: "ci", Title: "<CI>", Color: "#9b59b6", Click: nothing},
		{ID: "bad", Title: "Bad", Color: "red; background: url(x)"},
	}}
	html := render(t, BarContent([]Card{card}, nil))
	contains(t, html,
		`<li id="card-card" class="basic clickable" data-click="card-card" data-menu="card-card">`,
		`<div class="card-buttons"><button class="menu clickable" title="Actions" data-menu="card-card">⋯</button> `+
			`<button class="dismiss clickable" title="Dismiss" data-click="dismiss-card">×</button></div>`)
	lacks(t, html, "AWS", "CI", "action-")
	want := []MenuItem{
		{Title: "AWS", Color: "green", Enabled: true, ClickID: "action-aws"},
		{Title: "<CI>", Color: "#9b59b6", Enabled: true, ClickID: "action-ci"},
		// A colour that isn't one is gray, and one without a Click can't be chosen.
		{Title: "Bad", Color: "gray", ClickID: "action-bad"},
	}
	if got := card.Frame().Menu(); !slices.Equal(got, want) {
		t.Errorf("Menu() = %+v, want %+v", got, want)
	}
}

// Cards and the row have stable ids, for update to morph each into itself.
func TestBarContentIDs(t *testing.T) {
	html := render(t, BarContent([]Card{Basic{ID: "ci", Title: "CI"}}, nil))
	contains(t, html, `<ul id="cards">`, `id="card-ci"`)
}
