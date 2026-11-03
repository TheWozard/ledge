package components

import (
	"context"
	"fmt"
	"io"
	"maps"
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
	html := render(t, BarContent(Cards(cards), nil))
	contains(t, html,
		`<li id="card-deploy" class="basic" data-click="card/deploy/click" data-color="#e0362c" style="--card-color: #e0362c; --card-width: 240px;">`,
		`<span class="title">Plain</span>`,
		// A colour that isn't #rrggbb is none.
		`<li id="card-plain" class="basic">`,
		`<li id="card-new" class="basic" data-color="#2f9e5b" style="--card-color: #2f9e5b;">`,
		`&lt;b&gt;Deploy&lt;/b&gt;`, `<div class="description">done</div>`,
		// × shows only on a card that can be dismissed.
		`<div class="card-buttons"><button class="dismiss" title="Dismiss" data-click="card/deploy/dismiss">×</button></div>`)
	// Without actions, a card has no menu. Which cards are new is the page's to tell.
	lacks(t, html, "<b>Deploy", "No events.", "data-menu", "actions", "data-new", "--i")
	lacks(t, html, "url(x)")
	// Only the deploy card is clicked or dismissed.
	if strings.Count(html, "data-click") != 2 || strings.Count(html, `class="dismiss`) != 1 {
		t.Errorf("wrong cards are clickable or dismissable:\n%s", html)
	}
	contains(t, render(t, BarContent(nil, nil)), `<ul id="cards"></ul><div class="empty">No events.</div>`)
}

// Rendered with Clicks, the bar puts in them each Click on the cards by the id the page
// has for it: a card's and its ×'s. Nothing that isn't clickable is in them, nor are
// the actions, which are chosen from the card's menu.
func TestClicks(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	cards := []Basic{
		{ID: "a", Click: click("a"), Dismiss: click("dismiss a"),
			Actions: []Action{{ID: "q", Click: click("q")}, {ID: "none"}}},
		{ID: "b"},
	}
	html, clicks := renderClicks(t, BarContent(Cards(cards), nil))
	ids := []string{"card/a/click", "card/a/dismiss"}
	if !slices.Equal(clicks.IDs(), ids) {
		t.Fatalf("Clicks have %q, want %q", clicks.IDs(), ids)
	}
	for _, id := range ids {
		contains(t, html, `data-click="`+id+`"`)
		clicks.Click(id)(ClickEvent{})
	}
	if strings.Join(got, ",") != "a,dismiss a" {
		t.Errorf("the Clicks called %v", got)
	}
}

// A card's body can have parts clicked on their own, by ids of its own: no two
// cards' parts share an id, whatever their ids have in them, and none is ever the
// frame's, whatever it's called.
func TestParts(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	cards := []Card{
		parts{id: "a/b", parts: map[string]Click{"c": click("a/b c"), "click": click("a/b click")}},
		parts{id: "a", parts: map[string]Click{"b/c": click("a b/c"), "none": nil}},
	}
	html, clicks := renderClicks(t, BarContent(cards, nil))
	ids := []string{"body/a%2Fb/c", "body/a%2Fb/click", "body/a/b%2Fc"}
	if !slices.Equal(clicks.IDs(), ids) {
		t.Fatalf("Clicks have %q, want %q", clicks.IDs(), ids)
	}
	contains(t, html, `data-click="body/a/none"`)
	for _, id := range ids {
		clicks.Click(id)(ClickEvent{})
	}
	if strings.Join(got, ",") != "a/b c,a/b click,a b/c" {
		t.Errorf("the parts' Clicks called %v", got)
	}
}

// parts is a kind of card for TestParts: nothing but its parts, by their ids.
type parts struct {
	id    string
	parts map[string]Click
}

func (p parts) Frame() Frame { return Frame{ID: p.id} }

func (p parts) Body() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		for _, id := range slices.Sorted(maps.Keys(p.parts)) {
			if _, err := fmt.Fprintf(w, `<span data-click="%s"></span>`, ClickID(ctx, id, p.parts[id])); err != nil {
				return err
			}
		}
		return nil
	})
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

// Cards of any kinds share the frame: each kind's body sits in it, after its
// buttons, and its kind is a class on it, if it's a class name.
func TestKinds(t *testing.T) {
	cards := []Card{
		Basic{ID: "basic", Title: "Basic"},
		progress{id: "build", kind: "progress", done: 40},
		progress{id: "odd", kind: "Not a class", done: 10},
	}
	html, clicks := renderClicks(t, BarContent(cards, nil))
	contains(t, html,
		`<li id="card-basic" class="basic">`,
		`<li id="card-build" class="progress" data-menu="card/build/menu"><div class="card-buttons"><button class="menu" title="Actions" data-click="card/build/menu">⋯</button> </div><div class="done">40%</div></li>`,
		`<li id="card-odd" class="" data-menu="card/odd/menu">`)
	if click := clicks.Action("action/build/build-stop"); click == nil {
		t.Error("the progress card's action has no Click")
	}
}

// A card's actions are its menu, not on the page: a ⋯ before its × is a click that
// shows it, and the card has the ⋯'s id in data-menu, for a right-click to post. Its
// Menu is an item for each action, in order: its title, its colour as the page would
// name it, whether it can be chosen, and its id, with its card's in it.
func TestMenu(t *testing.T) {
	card := Basic{ID: "card", Title: "Card", Description: "Desc", Click: nothing, Dismiss: nothing, Actions: []Action{
		{ID: "aws", Title: "AWS", Color: "green", Click: nothing},
		{ID: "ci", Title: "<CI>", Color: "#9b59b6", Click: nothing},
		{ID: "bad", Title: "Bad", Color: "red; background: url(x)"},
	}}
	html := render(t, BarContent([]Card{card}, nil))
	contains(t, html,
		`<li id="card-card" class="basic" data-click="card/card/click" data-menu="card/card/menu">`,
		`<div class="card-buttons"><button class="menu" title="Actions" data-click="card/card/menu">⋯</button> `+
			`<button class="dismiss" title="Dismiss" data-click="card/card/dismiss">×</button></div>`)
	lacks(t, html, "AWS", "CI", "action/")
	want := []MenuItem{
		{Title: "AWS", Color: "green", Enabled: true, ID: "action/card/aws"},
		{Title: "<CI>", Color: "#9b59b6", Enabled: true, ID: "action/card/ci"},
		// A colour that isn't one is gray, and one without a Click can't be chosen.
		{Title: "Bad", Color: "gray", ID: "action/card/bad"},
	}
	if got := card.Frame().Menu(); !slices.Equal(got, want) {
		t.Errorf("Menu() = %+v, want %+v", got, want)
	}

	// The Click that shows it is the one menu gives for its frame, once, and only for
	// a card with actions.
	var opened []string
	_, clicks := renderClicks(t, BarContent([]Card{card, Basic{ID: "plain"}}, func(f Frame) Click {
		opened = append(opened, f.ID)
		return func(ClickEvent) { opened = append(opened, "shown "+f.ID) }
	}))
	if ids := []string{"card/card/click", "card/card/dismiss", "card/card/menu"}; !slices.Equal(clicks.IDs(), ids) {
		t.Fatalf("Clicks have %q, want %q", clicks.IDs(), ids)
	}
	clicks.Click("card/card/menu")(ClickEvent{})
	if !slices.Equal(opened, []string{"card", "shown card"}) {
		t.Errorf("menu was called, and the ⋯ clicked, for %q", opened)
	}
}

// Rendered with Clicks, the bar puts the Click of each action in them by its menu
// item's id, for its menu to choose from, and not where a page's click reaches; two
// cards' actions with the same id are each their own card's, and one without a Click
// has none.
func TestAction(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	a := Basic{ID: "a", Actions: []Action{{ID: "rerun", Click: click("a rerun")}, {ID: "info"}}}
	b := Basic{ID: "b", Actions: []Action{{ID: "rerun", Click: click("b rerun")}}}
	_, clicks := renderClicks(t, BarContent([]Card{a, b}, nil))
	clicks.Action(a.Frame().Menu()[0].ID)(ClickEvent{})
	clicks.Action(b.Frame().Menu()[0].ID)(ClickEvent{})
	if strings.Join(got, ",") != "a rerun,b rerun" {
		t.Errorf("the actions' Clicks called %v", got)
	}
	for _, id := range []string{a.Frame().Menu()[1].ID, "rerun", ""} {
		if clicks.Action(id) != nil {
			t.Errorf("clicks.Action(%q) has a Click", id)
		}
	}
	for _, item := range append(a.Frame().Menu(), b.Frame().Menu()...) {
		if clicks.Click(item.ID) != nil {
			t.Errorf("the page can click %q, an action", item.ID)
		}
	}
}

// Cards and the row have stable ids, for update to morph each into itself.
func TestBarContentIDs(t *testing.T) {
	html := render(t, BarContent([]Card{Basic{ID: "ci", Title: "CI"}}, nil))
	contains(t, html, `<ul id="cards">`, `id="card-ci"`)
}
