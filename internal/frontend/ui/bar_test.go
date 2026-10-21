package ui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/theme"
)

func TestWindow(t *testing.T) {
	gui(t)
	if frame := testBar.Place(); frame.Width == 0 || frame.Height != 110 {
		t.Errorf("frame = %+v, want the full width and 110 high", frame)
	}
	waitFor(t, `document.getElementById("app") !== null`, "true")
	if got := eval(t, `document.body.className`); got != `"bottom"` {
		t.Errorf("body class = %s", got)
	}
}

// Each card new since the last draw is fresh, numbered among the new ones. What's gone
// is forgotten, so it's new again if it comes back.
func TestArrivals(t *testing.T) {
	draws := []struct {
		ids   []string
		fresh map[string]int
	}{
		// On a fresh page, every card is new.
		{[]string{"x", "y"}, map[string]int{"card-x": 0, "card-y": 1}},
		{[]string{"x", "y", "z"}, map[string]int{"card-z": 0}},
		// Nothing changed.
		{[]string{"x", "y", "z"}, map[string]int{}},
		// y goes, w arrives, and they're reordered.
		{[]string{"z", "w", "x"}, map[string]int{"card-w": 0}},
		// y comes back, new again.
		{[]string{"y", "x"}, map[string]int{"card-y": 0}},
	}
	shown := map[string]bool{}
	for i, draw := range draws {
		var cards []components.Basic
		var want []string
		for _, id := range draw.ids {
			cards = append(cards, components.Basic{ID: id})
			want = append(want, "card-"+id)
		}
		var fresh map[string]int
		fresh, shown = arrivals(components.Cards(cards), shown)
		if !maps.Equal(fresh, draw.fresh) {
			t.Errorf("draw %d: fresh = %v, want %v", i, fresh, draw.fresh)
		}
		if got := slices.Sorted(maps.Keys(shown)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
			t.Errorf("draw %d: shown = %v, want %v", i, got, want)
		}
	}
}

func TestRender(t *testing.T) {
	gui(t)
	show(t, []components.Basic{
		{ID: "deploy", Title: "Deploy finished", Color: "#e0362c", Click: record("open deploy"),
			Actions: []components.Action{{ID: "aws", Title: "AWS", Color: "red", Click: record("run aws")}}},
		{ID: "note", Title: "<b>not bold</b>", Description: "plain text"},
	})
	waitFor(t, `[...document.querySelectorAll("li .title")].map(e => e.textContent).join("|")`, `"Deploy finished|<b>not bold</b>"`)
	for js, want := range map[string]string{
		`document.querySelector("li .title b") === null`:   "true",
		`document.querySelector("li").className`:           `"basic clickable"`,
		`document.querySelector("li").dataset.color`:       `"#e0362c"`,
		`document.querySelector("li").dataset.menu`:        `"card-deploy"`,
		`document.querySelector(".menu").closest("li").id`: `"card-deploy"`,
		// Its actions are in its menu, not on the page.
		`document.body.textContent.includes("AWS")`:                "false",
		`document.querySelector("#card-note[data-menu]") === null`: "true",
		`document.querySelector("a") === null`:                     "true",
		`document.querySelectorAll("li[data-new]").length > 0`:     "true",
	} {
		if got := eval(t, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
}

func TestOpenOnce(t *testing.T) {
	gui(t)
	if _, err := Open(defaults); err == nil {
		t.Error("a second Open should be an error")
	}
}

func TestNoNavigation(t *testing.T) {
	gui(t)
	waitFor(t, `document.getElementById("app") !== null`, "true")
	eval(t, `location.href = "https://example.com", true`)
	time.Sleep(500 * time.Millisecond)
	if got := eval(t, `location.href`); got != `"about:blank"` {
		t.Errorf("the bar navigated to %s", got)
	}
}

// A render morphs the bar: cards still there keep their elements and the row its
// scroll, while what changed changes.
func TestRenderMorphs(t *testing.T) {
	gui(t)
	var cards []components.Basic
	for i := range 12 {
		cards = append(cards, components.Basic{ID: fmt.Sprintf("m-%02d", i), Title: fmt.Sprintf("Card %02d", i)})
	}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "12")
	eval(t, `window.kept = document.getElementById("card-m-05");
		document.getElementById("cards").scrollLeft = 300, true`)
	if got := eval(t, `document.getElementById("cards").scrollLeft`); got != "300" {
		t.Fatalf("the row scrolled to %s, want 300; the test needs more cards", got)
	}

	// The first card goes, one changes its title, and one arrives.
	changed := slices.Clone(cards[1:])
	changed[4].Title = "Renamed"
	changed = append(changed, components.Basic{ID: "m-new", Title: "New"})
	testBar.Render(components.Cards(changed))
	waitFor(t, `document.getElementById("card-m-new") !== null`, "true")
	for js, want := range map[string]string{
		`document.getElementById("card-m-05") === window.kept`:                             "true",
		`window.kept.querySelector(".title").textContent`:                                  `"Renamed"`,
		`document.getElementById("card-m-00").hasAttribute("data-leaving")`:                "true",
		`document.getElementById("cards").scrollLeft`:                                      "300",
		`window.kept.hasAttribute("data-new")`:                                             "false",
		`document.getElementById("card-m-new").hasAttribute("data-new")`:                   "true",
		`[...document.querySelectorAll("li:not([data-leaving])")].map(li => li.id).join()`: `"` + strings.Join(idsOf(changed), ",") + `"`,
	} {
		if got := eval(t, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	waitFor(t, `document.getElementById("card-m-00") === null`, "true")
}

// RenderCard replaces only the card with its ID, in its place, and its Click with
// it. A card that isn't on the bar isn't added.
func TestRenderCard(t *testing.T) {
	gui(t)
	cards := []components.Basic{{ID: "rc-a", Title: "A"}, {ID: "rc-b", Title: "B"}, {ID: "rc-c", Title: "C"}}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("#cards > li").length`, "3")
	eval(t, `window.kept = document.getElementById("card-rc-a"), true`)

	if !testBar.RenderCard(components.Basic{ID: "rc-b", Title: "Changed", Click: record("open rc-b")}) {
		t.Fatal("RenderCard didn't find rc-b")
	}
	waitFor(t, `document.querySelector("#card-rc-b .title").textContent`, `"Changed"`)
	for js, want := range map[string]string{
		`document.getElementById("card-rc-a") === window.kept`:          "true",
		`[...document.querySelectorAll("li")].map(li => li.id).join()`:  `"card-rc-a,card-rc-b,card-rc-c"`,
		`document.getElementById("card-rc-b").hasAttribute("data-new")`: "false",
	} {
		if got := eval(t, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	eval(t, `document.getElementById("card-rc-b").click(), true`)
	eventually(t, "open rc-b", func() bool { return calls.has("open rc-b") })

	if testBar.RenderCard(components.Basic{ID: "rc-none", Title: "None"}) {
		t.Error("RenderCard found a card that isn't on the bar")
	}
	if got := eval(t, `document.querySelectorAll("#cards > li").length`); got != "3" {
		t.Errorf("%s cards on the bar, want 3", got)
	}
}

// New cards are marked new, and gone ones are marked leaving, told how far to
// collapse, and removed once they've left.
func TestEnterAndLeave(t *testing.T) {
	gui(t)
	x := components.Basic{ID: "x", Title: "X"}
	y := components.Basic{ID: "y", Title: "Y"}
	// leaving is whether the element is leaving and told how far to collapse.
	leaving := func(t *testing.T, id string) {
		t.Helper()
		got := eval(t, `(e => [e.hasAttribute("data-leaving"), parseFloat(e.style.getPropertyValue("--collapse-x")) < 0].join())(document.getElementById("`+id+`"))`)
		if got != `"true,true"` {
			t.Errorf("%s: leaving, collapsing = %s", id, got)
		}
	}
	withTheme(t, theme.Builtins.Default())
	show(t, []components.Basic{x, y})
	waitFor(t, `document.querySelectorAll("#cards > li").length`, "2")
	// Every card's new on a fresh page.
	if got := eval(t, `[...document.querySelectorAll("li")].every(e => e.hasAttribute("data-new"))`); got != "true" {
		t.Errorf("not every card on a fresh page is new")
	}

	// Card x goes, then the last card.
	testBar.Render(components.Cards([]components.Basic{y}))
	waitFor(t, `document.getElementById("card-x")?.hasAttribute("data-leaving")`, "true")
	leaving(t, "card-x")
	waitFor(t, `document.getElementById("card-x") === null`, "true")
	testBar.Render(nil)
	waitFor(t, `document.getElementById("card-y")?.hasAttribute("data-leaving")`, "true")
	waitFor(t, `document.getElementById("card-y") === null`, "true")
}

// A card that comes back while it's leaving stays.
func TestComesBackWhileLeaving(t *testing.T) {
	gui(t)
	cards := []components.Basic{{ID: "back", Title: "Back"}, {ID: "other", Title: "Other"}}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "2")
	eval(t, `window.back = document.getElementById("card-back"), true`)
	testBar.Render(components.Cards(cards[1:]))
	waitFor(t, `window.back.hasAttribute("data-leaving")`, "true")
	testBar.Render(components.Cards(cards))
	waitFor(t, `window.back.hasAttribute("data-leaving")`, "false")
	time.Sleep(3500 * time.Millisecond) // past the longest a leaving card is kept
	if got := eval(t, `document.getElementById("card-back") === window.back && window.back.isConnected`); got != "true" {
		t.Error("a card that came back was removed anyway")
	}
}

func idsOf(cards []components.Basic) []string {
	var ids []string
	for _, c := range cards {
		ids = append(ids, "card-"+c.ID)
	}
	return ids
}

// Each theme loads and shows the cards.
func TestThemes(t *testing.T) {
	gui(t)
	show(t, []components.Basic{{ID: "c", Title: "Themed", Description: "Described", Color: "#e0362c",
		Actions: []components.Action{{ID: "q", Title: "Quick", Color: "red"}}}})
	for _, name := range themes(t) {
		withTheme(t, name)
		waitFor(t, `document.querySelector("li[data-color] .card-buttons .menu") !== null`, "true")
	}
}

// The mouse wheel over something in a card that scrolls vertically, such as a long
// description, is left to scroll it rather than the row.
func TestWheelOverScrolling(t *testing.T) {
	gui(t)
	cards := []components.Basic{{ID: "long", Title: "Long", Description: strings.Repeat("A long description. ", 40)}}
	for i := range 8 {
		cards = append(cards, components.Basic{ID: fmt.Sprintf("wide-%d", i), Title: fmt.Sprintf("Card %d", i)})
	}
	show(t, cards)
	waitFor(t, `document.querySelector("li .description") !== null`, "true")

	// Made to scroll here, so it doesn't depend on how a theme lays the card out.
	const description = `document.querySelector("li .description")`
	eval(t, `(d => (d.style.height = "20px", d.style.overflowY = "auto", true))(`+description+`)`)
	got := eval(t, `(d => {
		const row = document.querySelector("ul"), before = row.scrollLeft;
		const wheel = new WheelEvent("wheel", {deltaY: 40, bubbles: true, cancelable: true});
		d.dispatchEvent(wheel);
		return !wheel.defaultPrevented && row.scrollLeft === before;
	})(`+description+`)`)
	if got != "true" {
		t.Error("the wheel over a scrolling description moved the row")
	}
}

// The page counts a timer down by itself, without another Render, and marks its card
// done once it's up.
func TestTimerCountsDown(t *testing.T) {
	gui(t)
	start := time.Now()
	// One without a start, for something coming up, counts down the same.
	testBar.Render([]components.Card{
		components.Timer{Basic: components.Basic{ID: "egg", Title: "Egg"}, Start: start, End: start.Add(1500 * time.Millisecond)},
		components.Timer{Basic: components.Basic{ID: "meeting", Title: "Meeting"}, End: start.Add(1500 * time.Millisecond)},
	})
	t.Cleanup(func() { testBar.Render(nil) })
	for _, id := range []string{"card-egg", "card-meeting"} {
		waitFor(t, `document.querySelector("#`+id+` .time-left")?.textContent`, `"0:01"`)
		if got := eval(t, `document.getElementById("`+id+`").hasAttribute("data-done")`); got != "false" {
			t.Errorf("%s is done before it's up", id)
		}
	}
	for _, id := range []string{"card-egg", "card-meeting"} {
		waitFor(t, `(c => c.querySelector(".time-left").textContent + " " + c.hasAttribute("data-done"))(document.getElementById("`+id+`"))`, `"0:00 true"`)
	}
}

// A timer without an end counts up from its start by itself, without another Render,
// and is never done.
func TestTimerCountsUp(t *testing.T) {
	gui(t)
	start := time.Now().Add(-58 * time.Second)
	testBar.Render([]components.Card{components.Timer{Basic: components.Basic{ID: "incident", Title: "Incident"}, Start: start}})
	t.Cleanup(func() { testBar.Render(nil) })
	waitFor(t, `document.querySelector("#card-incident .time-left")?.textContent`, `"1:00"`)
	if got := eval(t, `document.getElementById("card-incident").hasAttribute("data-done")`); got != "false" {
		t.Error("a timer without an end is done")
	}
}
