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

// A message is one of the page's actions, with an id if it takes one, an event of
// only the keys and button if it's a click or a choice, and nothing more.
func TestReadMessage(t *testing.T) {
	for _, c := range []struct {
		actions map[string]bool
		body    string
		want    string
	}{
		{barMessages, `{"action": "ready"}`, ""},
		{barMessages, `{"action": "click", "id": "a"}`, ""},
		{barMessages, `{"action": "menu", "id": "card-a"}`, ""},
		{barMessages, `{"action": "menu"}`, "menu needs an id"},
		{barMessages, `{"action": "click"}`, "click needs an id"},
		{barMessages, `{"action": "ready", "id": "a"}`, "ready takes no id"},
		{barMessages, `{"action": "open", "id": "a"}`, `no action "open"`},
		{barMessages, `{"action": "choose", "id": "0"}`, `no action "choose"`},
		{barMessages, `{"action": "click", "id": "a", "path": "/etc"}`, `unknown field "path"`},
		{barMessages, `"click"`, "cannot unmarshal string"},
		{barMessages, `{"action": "click", "id": "a"} {}`, "more than one value"},
		{menuMessages, `{"action": "choose", "id": "0"}`, ""},
		{menuMessages, `{"action": "close"}`, ""},
		{menuMessages, `{"action": "close", "id": "0"}`, "close takes no id"},
		{menuMessages, `{"action": "choose"}`, "choose needs an id"},
		{menuMessages, `{"action": "click", "id": "card-a"}`, `no action "click"`},
		{barMessages, `{"action": "click", "id": "a", "event": {"alt": true, "cmd": false, "shift": true, "ctrl": false, "middle": true}}`, ""},
		{barMessages, `{"action": "click", "id": "a", "event": {}}`, ""},
		{menuMessages, `{"action": "choose", "id": "0", "event": {"cmd": true}}`, ""},
		{reportMessages, `{"action": "click", "id": "go", "event": {"shift": true}}`, ""},
		{barMessages, `{"action": "menu", "id": "card-a", "event": {"alt": true}}`, "menu takes no event"},
		{barMessages, `{"action": "ready", "event": {}}`, "ready takes no event"},
		{barMessages, `{"action": "click", "id": "a", "event": {"alt": "yes"}}`, "cannot unmarshal string"},
		{barMessages, `{"action": "click", "id": "a", "event": {"url": "x"}}`, `unknown field "url"`},
		{barMessages, `{"action": "click", "id": "a", "event": "alt"}`, "cannot unmarshal string"},
	} {
		_, err := readMessage([]byte(c.body), c.actions)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: got %v, want %q", c.body, err, c.want)
		}
	}
	m, err := readMessage([]byte(`{"action": "click", "id": "a", "event": {"alt": true, "middle": true}}`), barMessages)
	if want := (components.ClickEvent{Alt: true, Middle: true}); err != nil || m.clickEvent() != want {
		t.Errorf("the click's event is %+v, %v; want %+v", m.clickEvent(), err, want)
	}
	if m, _ := readMessage([]byte(`{"action": "click", "id": "a"}`), barMessages); m.clickEvent() != (components.ClickEvent{}) {
		t.Errorf("a click without an event has %+v", m.clickEvent())
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
	eval(t, `window.webkit.messageHandlers.ledge.postMessage({action: "click", id: "card/card-open/click"}), true`)
	eventually(t, "the click to be dropped", func() bool { return strings.Contains(logs.String()[before:], "isn't on it") })
}

// A Click is given how it was clicked: the modifier keys held, and whether with the
// middle button, which clicks as the main one does, but not a card's ⋯.
func TestClickEvents(t *testing.T) {
	gui(t)
	show(t, []components.Basic{{ID: "ev", Title: "Events", Click: record("open ev"), Dismiss: record("dismiss ev"),
		Actions: []components.Action{{ID: "ev-run", Title: "Run", Click: record("run ev")}}}})
	waitFor(t, `document.querySelectorAll("li").length`, "1")
	mouse := func(selector, kind, init string) {
		eval(t, `document.querySelector("`+selector+`").dispatchEvent(new MouseEvent("`+kind+`", {bubbles: true, cancelable: true, `+init+`})), true`)
	}

	mouse("#card-ev .title", "click", "altKey: true, metaKey: true")
	eventually(t, "open ev", func() bool { return calls.has("open ev") })
	if got, want := calls.event("open ev"), (components.ClickEvent{Alt: true, Cmd: true}); got != want {
		t.Errorf("an ⌥⌘-click was %+v, want %+v", got, want)
	}
	mouse("#card-ev .dismiss", "click", "shiftKey: true")
	eventually(t, "dismiss ev", func() bool { return calls.has("dismiss ev") })
	if got, want := calls.event("dismiss ev"), (components.ClickEvent{Shift: true}); got != want {
		t.Errorf("a ⇧-click on × was %+v, want %+v", got, want)
	}
	mouse("#card-ev .title", "auxclick", "button: 1")
	eventually(t, "a middle click", func() bool { return calls.count("open ev") == 2 })
	if got, want := calls.event("open ev"), (components.ClickEvent{Middle: true}); got != want {
		t.Errorf("a middle click was %+v, want %+v", got, want)
	}
	// Not the right button, and not on the ⋯.
	mouse("#card-ev .title", "auxclick", "button: 2")
	mouse("#card-ev .menu", "auxclick", "button: 1")
	if time.Sleep(200 * time.Millisecond); calls.count("open ev") != 2 {
		t.Errorf("the card was clicked %d times, want 2", calls.count("open ev"))
	}
}

// A click on a part of a card, such as a check with a Click, calls the part's Click
// and not its card's; a click on the rest of the card is still the card's.
func TestPartClicks(t *testing.T) {
	gui(t)
	testBar.Render([]components.Card{components.Checks{
		Basic: components.Basic{ID: "ci", Title: "CI", Click: record("open ci")},
		Checks: []components.Check{
			{Name: "build", State: "passed"},
			{Name: "test", State: "failed", Click: record("open ci test")},
		}}})
	t.Cleanup(func() {
		testBar.Render(nil)
		waitFor(t, `document.querySelector("#cards > *") === null`, "true")
	})
	waitFor(t, `document.querySelectorAll(".check").length`, "2")
	eval(t, `document.querySelector(".check.clickable .check-mark").click(), true`)
	eventually(t, "open ci test", func() bool { return calls.has("open ci test") })
	time.Sleep(200 * time.Millisecond)
	if calls.has("open ci") {
		t.Error("clicking a check with a Click also opened its card")
	}
	eval(t, `document.querySelector(".check:not(.clickable)").click(), true`)
	eventually(t, "open ci", func() bool { return calls.has("open ci") })
}

// While a Click runs, what was clicked is busy, through renders too, and a click on it
// meanwhile is dropped; once it's done, it isn't, and it can be clicked again.
func TestBusy(t *testing.T) {
	gui(t)
	release := make(chan bool)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	slow := func(name string) components.Click {
		return func(components.ClickEvent) { calls.add(name); <-release }
	}
	cards := []components.Basic{{ID: "busy", Title: "Busy", Click: slow("open busy")}}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "1")
	eval(t, `document.getElementById("card-busy").click(), true`)
	waitFor(t, `document.getElementById("card-busy").hasAttribute("data-busy")`, "true")

	// A render keeps it busy, and a click on it, even one posted past the page, is
	// dropped.
	testBar.Render(components.Cards(cards))
	before := logs.Len()
	eval(t, `window.webkit.messageHandlers.ledge.postMessage({action: "click", id: "card/busy/click"}), true`)
	eventually(t, "the click to be dropped", func() bool { return strings.Contains(logs.String()[before:], "still running") })
	if got := eval(t, `document.getElementById("card-busy").hasAttribute("data-busy")`); got != "true" {
		t.Error("a render took the card's busy mark off")
	}
	close(release)
	waitFor(t, `document.getElementById("card-busy").hasAttribute("data-busy")`, "false")
	if n := calls.count("open busy"); n != 1 {
		t.Errorf("the card's Click ran %d times while it was busy, want 1", n)
	}
	eval(t, `document.getElementById("card-busy").click(), true`)
	eventually(t, "a second open", func() bool { return calls.count("open busy") == 2 })
}

// While an action chosen from a card's menu runs, the card's ⋯ is busy, and the card
// isn't.
func TestBusyMenu(t *testing.T) {
	gui(t)
	withTheme(t, "native")
	real := showMenu
	showMenu = func([]shim.MenuItem) shim.Chosen { return shim.Chosen{} }
	t.Cleanup(func() { showMenu = real })
	release := make(chan bool)
	t.Cleanup(func() { close(release) })
	show(t, []components.Basic{{ID: "acts", Title: "Acts", Click: record("open acts"), Actions: []components.Action{
		{ID: "slow", Title: "Slow", Click: func(components.ClickEvent) { calls.add("run slow"); <-release }},
	}}})
	waitFor(t, `document.querySelectorAll("li").length`, "1")
	eval(t, `document.querySelector("#card-acts .menu").click(), true`)
	waitFor(t, `document.querySelector("#card-acts .menu").hasAttribute("data-busy")`, "true")
	if got := eval(t, `document.getElementById("card-acts").hasAttribute("data-busy")`); got != "false" {
		t.Error("an action running made its card busy")
	}
}

// In a theme without a menu of its own, a right-click on a card with actions, or a
// click on its ⋯, pops up the native menu of them, each with its colour, and greyed
// out without a Click. Choosing one calls its Click, and not its card's nor another
// card's action with the same id; choosing nothing does nothing; and choosing one a
// render has since taken off is dropped.
// WebKit's own menu never shows. No real menu opens: one that records it, and chooses
// what it's told to, stands in.
func TestNativeMenus(t *testing.T) {
	gui(t)
	withTheme(t, "native")
	var mu sync.Mutex
	var menus [][]shim.MenuItem
	choose, then := 0, func() {}
	opened := func() int { mu.Lock(); defer mu.Unlock(); return len(menus) }
	real := showMenu
	showMenu = func(items []shim.MenuItem) shim.Chosen {
		mu.Lock()
		menus = append(menus, items)
		chosen, before := choose, then
		mu.Unlock()
		before()
		return shim.Chosen{Index: chosen, Modifiers: shim.Modifiers{Cmd: true}}
	}
	t.Cleanup(func() { showMenu = real })
	cards := []components.Basic{
		{ID: "m", Title: "Menu", Click: record("open m"), Actions: []components.Action{
			{ID: "run", Title: "Run", Color: "green", Click: record("run run")}, {ID: "info", Title: "Info"},
		}},
		{ID: "plain", Title: "Plain"},
		{ID: "other", Title: "Other", Actions: []components.Action{{ID: "run", Title: "Run", Click: record("run other")}}},
	}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "3")

	// A right-click chooses Run.
	if got := rightClick(t, "card-m"); got != "false" {
		t.Error("WebKit's menu wasn't stopped")
	}
	eventually(t, "run run", func() bool { return calls.has("run run") })
	if got, want := calls.event("run run"), (components.ClickEvent{Cmd: true}); got != want {
		t.Errorf("Run, chosen with ⌘ held, was clicked %+v, want %+v", got, want)
	}
	want := []shim.MenuItem{{Title: "Run", Color: "green", Enabled: true}, {Title: "Info", Color: "gray"}}
	if mu.Lock(); len(menus) == 0 || !slices.Equal(menus[0], want) {
		t.Errorf("the menu was %+v, want %+v", menus[0], want)
	}
	mu.Unlock()
	if time.Sleep(200 * time.Millisecond); calls.has("open m") || calls.has("run other") {
		t.Error("choosing from the menu also opened its card, or ran another card's action")
	}

	// The ⋯ opens it too; choosing nothing does nothing.
	mu.Lock()
	choose = -1
	mu.Unlock()
	eval(t, `document.querySelector("#card-m .menu").click(), true`)
	eventually(t, "the ⋯ menu", func() bool { return opened() == 2 })

	// A card without actions has no menu, nor WebKit's.
	if got := rightClick(t, "card-plain"); got != "false" {
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
	rightClick(t, "card-m")
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

// rightClick right-clicks element id on the bar, and says whether WebKit's own menu
// was let show.
func rightClick(t *testing.T, id string) string {
	t.Helper()
	return eval(t, `document.getElementById("`+id+`").dispatchEvent(new MouseEvent("contextmenu", {bubbles: true, cancelable: true}))`)
}

// In a theme with a menu of its own, a right-click on a card with actions, or a click
// on its ⋯, shows its page at the mouse, an item for each action with its colour, and
// greyed out without a Click. Clicking one, or Return on one the arrow keys are on,
// calls its Click, not its card's, and hides it; Escape hides it without; and one
// whose card a render has since taken off is dropped.
func TestMenus(t *testing.T) {
	gui(t)
	cards := []components.Basic{
		{ID: "m", Title: "Menu", Click: record("open m"), Actions: []components.Action{
			{ID: "info", Title: "<Info>"}, {ID: "go", Title: "Go", Color: "green", Click: record("go go")},
		}},
		{ID: "plain", Title: "Plain"},
	}
	show(t, cards)
	waitFor(t, `document.querySelectorAll("li").length`, "2")
	// opens opens the card's menu with open, and waits for it to show. If it doesn't,
	// it says what was logged meanwhile, such as why the menu wasn't shown.
	opens := func(t *testing.T, open func()) {
		t.Helper()
		logged := logs.Len()
		defer func() {
			if t.Failed() {
				t.Logf("logged while opening the menu:\n%s", logs.String()[logged:])
			}
		}()
		open()
		eventually(t, "the menu to show", func() bool { return shim.WindowVisible(MenuWindow) })
		waitIn(t, MenuWindow, `document.querySelectorAll(".menu-item").length`, "2")
	}
	hidden := func(t *testing.T) {
		t.Helper()
		eventually(t, "the menu to hide", func() bool { return !shim.WindowVisible(MenuWindow) })
	}

	// A right-click shows the menu, and a click on Go chooses it.
	opens(t, func() {
		if got := rightClick(t, "card-m"); got != "false" {
			t.Error("WebKit's menu wasn't stopped")
		}
	})
	for js, want := range map[string]string{
		`[...document.querySelectorAll(".menu-title")].map(e => e.textContent).join("|")`: `"<Info>|Go"`,
		`document.querySelector(".menu-title b") === null`:                                "true",
		`document.querySelector(".menu-item").hasAttribute("aria-disabled")`:              "true",
		`document.querySelectorAll(".menu-item")[1].dataset.color`:                        `"green"`,
		`!!window.webkit.messageHandlers.ledge`:                                           "true",
	} {
		if got := evalIn(t, MenuWindow, js); got != want {
			t.Errorf("%s = %s, want %s", js, got, want)
		}
	}
	// Info can't be chosen.
	evalIn(t, MenuWindow, `document.querySelectorAll(".menu-item")[0].click(), true`)
	if time.Sleep(200 * time.Millisecond); !shim.WindowVisible(MenuWindow) {
		t.Fatal("clicking an item that can't be chosen closed the menu")
	}
	evalIn(t, MenuWindow, `document.querySelectorAll(".menu-item")[1].dispatchEvent(new MouseEvent("click", {bubbles: true, altKey: true})), true`)
	eventually(t, "go go", func() bool { return calls.has("go go") })
	if got, want := calls.event("go go"), (components.ClickEvent{Alt: true}); got != want {
		t.Errorf("Go, chosen with ⌥ held, was clicked %+v, want %+v", got, want)
	}
	hidden(t)
	if time.Sleep(200 * time.Millisecond); calls.has("open m") {
		t.Error("choosing from the menu also opened its card")
	}

	// The ⋯ shows it too; Escape hides it without choosing.
	before := calls.count("go go")
	key := func(key string) {
		evalIn(t, MenuWindow, `document.dispatchEvent(new KeyboardEvent("keydown", {key: "`+key+`"})), true`)
	}
	opens(t, func() { eval(t, `document.querySelector("#card-m .menu").click(), true`) })
	key("Escape")
	hidden(t)

	// The arrow keys skip what can't be chosen, and Return chooses.
	opens(t, func() { rightClick(t, "card-m") })
	key("ArrowDown")
	if got := evalIn(t, MenuWindow, `document.querySelector(".menu-item.active .menu-title")?.textContent`); got != `"Go"` {
		t.Errorf("the arrow went to %s, want Go", got)
	}
	evalIn(t, MenuWindow, `document.dispatchEvent(new KeyboardEvent("keydown", {key: "Enter", shiftKey: true})), true`)
	eventually(t, "go go again", func() bool { return calls.count("go go") == before+1 })
	if got, want := calls.event("go go"), (components.ClickEvent{Shift: true}); got != want {
		t.Errorf("Go, chosen by ⇧-Return, was clicked %+v, want %+v", got, want)
	}
	hidden(t)

	// A card without actions has no menu, nor WebKit's.
	if got := rightClick(t, "card-plain"); got != "false" {
		t.Error("WebKit's menu wasn't stopped on a card without actions")
	}
	if time.Sleep(300 * time.Millisecond); shim.WindowVisible(MenuWindow) {
		t.Error("a card without actions had a menu")
	}

	// Go, chosen once a render has taken its card off, is dropped.
	opens(t, func() { rightClick(t, "card-m") })
	testBar.Render(components.Cards(cards[1:]))
	logged := logs.Len()
	evalIn(t, MenuWindow, `document.querySelectorAll(".menu-item")[1].click(), true`)
	eventually(t, "the choice to be dropped", func() bool { return strings.Contains(logs.String()[logged:], "isn't on the bar") })
	hidden(t)
}

// One menu is open at a time: opening another closes the first without a choice. A
// choice of what can't be chosen is refused, and leaves it open; with none open,
// nothing's chosen.
func TestPopup(t *testing.T) {
	var p popup
	item := func(i int) shim.Chosen { return shim.Chosen{Index: i} }
	if p.isOpen() || p.finish(noChoice) {
		t.Fatal("a menu is open before any's opened")
	}
	first := p.open([]bool{true, false})
	second := p.open([]bool{false, true})
	if got := <-first; got != noChoice {
		t.Errorf("the first menu, closed by the second, chose %+v", got)
	}
	if p.finish(item(0)) || p.finish(item(2)) || !p.isOpen() {
		t.Error("an item that can't be chosen was chosen")
	}
	// What's chosen goes with the keys held as it was.
	shifted := shim.Chosen{Index: 1, Modifiers: shim.Modifiers{Shift: true}}
	if !p.finish(shifted) || <-second != shifted || p.isOpen() {
		t.Error("choosing an item didn't close the menu with it")
	}
	if p.finish(item(1)) {
		t.Error("a closed menu was chosen from")
	}
}
