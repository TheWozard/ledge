package ui

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
)

func TestReadMessage(t *testing.T) {
	for body, want := range map[string]string{
		`{"action": "ready"}`:                            "",
		`{"action": "click", "id": "a"}`:                 "",
		`{"action": "menu", "id": "card-a"}`:             "",
		`{"action": "menu"}`:                             "menu needs an id",
		`{"action": "click"}`:                            "click needs an id",
		`{"action": "ready", "id": "a"}`:                 "ready takes no id",
		`{"action": "open", "id": "a"}`:                  `no action "open"`,
		`{"action": "click", "id": "a", "path": "/etc"}`: `unknown field "path"`,
		`"click"`:                           "cannot unmarshal string",
		`{"action": "click", "id": "a"} {}`: "more than one value",
	} {
		_, err := readMessage([]byte(body))
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s: got %v, want %q", body, err, want)
		}
	}
}

// Clicks post only ids, which reach the Clicks: a card's, and its ×'s apart from the
// card's. A click on what's gone is dropped.
func TestClicks(t *testing.T) {
	gui(t)
	show(t, []components.Basic{
		{ID: "card-open", Title: "Opens", Click: record("open card-open"), Dismiss: record("dismiss card-open")},
		{ID: "card-still", Title: "Still"},
	})
	waitFor(t, `document.querySelectorAll("li").length`, "2")
	if got := eval(t, `document.querySelectorAll("#card-card-still .dismiss").length`); got != "0" {
		t.Error("a card without a Dismiss has ×")
	}
	eval(t, `document.querySelector("#card-card-open .dismiss").click(), true`)
	eventually(t, "dismiss card-open", func() bool { return calls.has("dismiss card-open") })
	time.Sleep(200 * time.Millisecond)
	if calls.has("open card-open") {
		t.Error("clicking × also opened its card")
	}
	for _, js := range []string{
		`[...document.querySelectorAll("li")].find(li => li.textContent.includes("Still")).click()`,
		`document.querySelector("#card-card-open .title").click()`,
	} {
		eval(t, js+", true")
	}
	eventually(t, "open card-open", func() bool { return calls.has("open card-open") })
	time.Sleep(200 * time.Millisecond)
	if calls.has("open card-still") {
		t.Error("clicking a card without a Click called something")
	}

	// Once a render takes a card off, a click on it is dropped.
	before := logs.Len()
	testBar.Render(components.Cards([]components.Basic{{ID: "card-still", Title: "Still"}}))
	eval(t, `window.webkit.messageHandlers.ledge.postMessage({action: "click", id: "card-card-open"}), true`)
	eventually(t, "the click to be dropped", func() bool { return strings.Contains(logs.String()[before:], "isn't on it") })
}

// A right-click on a card with actions, or a click on its ⋯, pops up its menu of them,
// each with its colour, and greyed out without a Click. Choosing one calls its Click,
// and not its card's; choosing nothing does nothing; and choosing one a render has
// since taken off is dropped. WebKit's own menu never shows. No real menu opens: one
// that records it, and chooses what it's told to, stands in.
func TestMenus(t *testing.T) {
	gui(t)
	var mu sync.Mutex
	var menus [][]shim.MenuItem
	choose, then := 0, func() {}
	opened := func() int { mu.Lock(); defer mu.Unlock(); return len(menus) }
	real := showMenu
	showMenu = func(items []shim.MenuItem) int {
		mu.Lock()
		menus = append(menus, items)
		chosen, before := choose, then
		mu.Unlock()
		before()
		return chosen
	}
	t.Cleanup(func() { showMenu = real })
	cards := []components.Basic{
		{ID: "m", Title: "Menu", Click: record("open m"), Actions: []components.Action{
			{ID: "run", Title: "Run", Color: "green", Click: record("run run")}, {ID: "info", Title: "Info"},
		}},
		{ID: "plain", Title: "Plain"},
	}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "2")
	rightClick := func(id string) string {
		return eval(t, `document.getElementById("`+id+`").dispatchEvent(new MouseEvent("contextmenu", {bubbles: true, cancelable: true}))`)
	}

	// A right-click chooses Run.
	if got := rightClick("card-m"); got != "false" {
		t.Error("WebKit's menu wasn't stopped")
	}
	eventually(t, "run run", func() bool { return calls.has("run run") })
	want := []shim.MenuItem{{Title: "Run", Color: "green", Enabled: true}, {Title: "Info", Color: "gray"}}
	if mu.Lock(); !slices.Equal(menus[0], want) {
		t.Errorf("the menu was %+v, want %+v", menus[0], want)
	}
	mu.Unlock()
	if time.Sleep(200 * time.Millisecond); calls.has("open m") {
		t.Error("choosing from the menu also opened its card")
	}

	// The ⋯ opens it too; choosing nothing does nothing.
	mu.Lock()
	choose = -1
	mu.Unlock()
	eval(t, `document.querySelector("#card-m .menu").click(), true`)
	eventually(t, "the ⋯ menu", func() bool { return opened() == 2 })

	// A card without actions has no menu, nor WebKit's.
	if got := rightClick("card-plain"); got != "false" {
		t.Error("WebKit's menu wasn't stopped on a card without actions")
	}
	if time.Sleep(200 * time.Millisecond); opened() != 2 {
		t.Error("a card without actions had a menu")
	}

	// Run, chosen once a render has taken its card off, is dropped.
	before := logs.Len()
	mu.Lock()
	choose, then = 0, func() { testBar.Render(components.Cards(cards[1:])) }
	mu.Unlock()
	rightClick("card-m")
	eventually(t, "the choice to be dropped", func() bool { return strings.Contains(logs.String()[before:], "isn't on the bar") })
}

func TestMessagesAreChecked(t *testing.T) {
	gui(t)
	show(t, []components.Basic{{ID: "kept", Title: "Kept", Click: record("click kept")}})
	waitFor(t, `document.getElementById("card-kept") !== null`, "true")
	before := logs.Len()
	for _, message := range []string{`{action: "click"}`, `{action: "click", id: "card-kept", path: "/etc"}`, `{action: "exec", id: "card-kept"}`, `"click"`} {
		eval(t, `window.webkit.messageHandlers.ledge.postMessage(`+message+`), true`)
	}
	eventually(t, "4 refusals", func() bool { return strings.Count(logs.String()[before:], "the bar sent") == 4 })
	if calls.has("click kept") {
		t.Error("a refused message reached a Click")
	}
}

// A value that can't be JSON, posted or returned, is dropped or an error, not a crash.
func TestNotJSON(t *testing.T) {
	gui(t)
	eval(t, `[new Date(), NaN, {n: Infinity}].forEach(m => window.webkit.messageHandlers.ledge.postMessage(m)), true`)
	if _, err := shim.EvalResult(BarWindow, `new Date()`); err == nil {
		t.Error("a Date result should be an error")
	}
	if got := eval(t, `1 + 1`); got != "2" {
		t.Errorf("after values that aren't JSON, 1 + 1 = %s", got)
	}
}
