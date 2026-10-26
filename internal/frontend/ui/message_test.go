package ui

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// A message is one of the page's actions, with an id if it takes one, an event of
// only the keys and button if it's a click or a choice, and nothing more.
func TestReadMessage(t *testing.T) {
	for _, c := range []struct {
		actions map[string]bool
		body    string
		want    string
	}{
		{pageMessages, `{"action": "ready"}`, ""},
		{pageMessages, `{"action": "click", "id": "a"}`, ""},
		{pageMessages, `{"action": "menu", "id": "card-a"}`, `no action "menu"`},
		{pageMessages, `{"action": "click"}`, "click needs an id"},
		{pageMessages, `{"action": "ready", "id": "a"}`, "ready takes no id"},
		{pageMessages, `{"action": "open", "id": "a"}`, `no action "open"`},
		{pageMessages, `{"action": "choose", "id": "0"}`, `no action "choose"`},
		{pageMessages, `{"action": "click", "id": "a", "path": "/etc"}`, `unknown field "path"`},
		{pageMessages, `"click"`, "cannot unmarshal string"},
		{pageMessages, `{"action": "click", "id": "a"} {}`, "more than one value"},
		{menuMessages, `{"action": "choose", "id": "0"}`, ""},
		{menuMessages, `{"action": "close"}`, ""},
		{menuMessages, `{"action": "close", "id": "0"}`, "close takes no id"},
		{menuMessages, `{"action": "choose"}`, "choose needs an id"},
		{menuMessages, `{"action": "click", "id": "card-a"}`, `no action "click"`},
		{pageMessages, `{"action": "click", "id": "a", "event": {"alt": true, "cmd": false, "shift": true, "ctrl": false, "middle": true}}`, ""},
		{pageMessages, `{"action": "click", "id": "a", "event": {}}`, ""},
		{menuMessages, `{"action": "choose", "id": "0", "event": {"cmd": true}}`, ""},
		{pageMessages, `{"action": "ready", "event": {}}`, "ready takes no event"},
		{pageMessages, `{"action": "click", "id": "a", "event": {"alt": "yes"}}`, "cannot unmarshal string"},
		{pageMessages, `{"action": "click", "id": "a", "event": {"url": "x"}}`, `unknown field "url"`},
		{pageMessages, `{"action": "click", "id": "a", "event": "alt"}`, "cannot unmarshal string"},
	} {
		_, err := readMessage([]byte(c.body), c.actions)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: got %v, want %q", c.body, err, c.want)
		}
	}
	m, err := readMessage([]byte(`{"action": "click", "id": "a", "event": {"alt": true, "middle": true}}`), pageMessages)
	if want := (components.ClickEvent{Alt: true, Middle: true}); err != nil || m.clickEvent() != want {
		t.Errorf("the click's event is %+v, %v; want %+v", m.clickEvent(), err, want)
	}
	if m, _ := readMessage([]byte(`{"action": "click", "id": "a"}`), pageMessages); m.clickEvent() != (components.ClickEvent{}) {
		t.Errorf("a click without an event has %+v", m.clickEvent())
	}
}

// A click posts the id of the innermost thing clicked that has a Click: a card's, or
// its ×'s apart from the card's. A card without a Click posts nothing.
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

// While a Click runs, what was clicked is marked busy, through renders too; once it's
// done, it isn't, and it can be clicked again.
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

	// A render keeps it busy, and a click on it posts nothing: the app would log one
	// it dropped.
	cards[0].Title = "Still busy"
	testBar.Render(components.Cards(cards))
	waitFor(t, `document.querySelector("#card-busy .title").textContent`, `"Still busy"`)
	if got := eval(t, `document.getElementById("card-busy").hasAttribute("data-busy")`); got != "true" {
		t.Error("a render took the card's busy mark off")
	}
	before := logs.Len()
	eval(t, `document.getElementById("card-busy").click(), true`)
	if time.Sleep(200 * time.Millisecond); strings.Contains(logs.String()[before:], "still running") {
		t.Error("a click on what's busy was posted")
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
	standInMenu(t, func([]shim.MenuItem) shim.Chosen { return shim.Chosen{} })
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
// click on its ⋯, pops up the native menu, and what's chosen from it is run, not the
// card's Click. WebKit's own menu never shows, nor does a card without actions have
// one. No real menu opens: one that records it, and chooses Run, stands in.
func TestNativeMenus(t *testing.T) {
	gui(t)
	withTheme(t, "native")
	var mu sync.Mutex
	opened := 0
	standInMenu(t, func([]shim.MenuItem) shim.Chosen {
		mu.Lock()
		defer mu.Unlock()
		opened++
		return shim.Chosen{Index: 0}
	})
	count := func() int { mu.Lock(); defer mu.Unlock(); return opened }
	show(t, []components.Basic{
		{ID: "m", Title: "Menu", Click: record("open m"), Actions: []components.Action{{ID: "run", Title: "Run", Click: record("run run")}}},
		{ID: "plain", Title: "Plain"},
	})
	waitFor(t, `document.querySelectorAll("li").length`, "2")

	if got := rightClick(t, "card-m"); got != "false" {
		t.Error("WebKit's menu wasn't stopped")
	}
	eventually(t, "run run", func() bool { return calls.has("run run") })
	idle(t, "card-m")
	eval(t, `document.querySelector("#card-m .menu").click(), true`)
	eventually(t, "the ⋯ menu", func() bool { return count() == 2 })
	idle(t, "card-m")

	if got := rightClick(t, "card-plain"); got != "false" {
		t.Error("WebKit's menu wasn't stopped on a card without actions")
	}
	if time.Sleep(200 * time.Millisecond); count() != 2 || calls.has("open m") {
		t.Error("a card without actions had a menu, or choosing from a menu opened its card")
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

// idle waits for the ⋯ of the card with element id, if it's there, not to be busy: for
// the Click that shows its menu, and runs what's chosen, to be done, so it can show it
// again.
func idle(t *testing.T, id string) {
	t.Helper()
	waitFor(t, `document.querySelector("#`+id+` .menu")?.hasAttribute("data-busy") === true`, "false")
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
// calls its Click, not its card's, and hides it; Escape hides it without.
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
		idle(t, "card-m")
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
}

// One menu is open at a time: opening another closes the first without a choice. A
// choice of what can't be chosen is refused, and leaves it open; with none open,
// nothing's chosen.
func TestPopup(t *testing.T) {
	var p popup
	item := func(i int) choice { return choice{index: i} }
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
	shifted := choice{1, components.ClickEvent{Shift: true}}
	if !p.finish(shifted) || <-second != shifted || p.isOpen() {
		t.Error("choosing an item didn't close the menu with it")
	}
	if p.finish(item(1)) {
		t.Error("a closed menu was chosen from")
	}
}

// What the bar's page posts reaches the Click it names, with how it was clicked, and
// nothing else: not a message that isn't one of the page's, nor one from a window
// that isn't open, nor a Click a render has since taken off.
func TestClickMessages(t *testing.T) {
	b, _ := fakeBar(t, theme.Theme{})
	b.Render(components.Cards([]components.Basic{
		{ID: "fc", Title: "Clicks", Click: record("fake open fc"), Dismiss: record("fake dismiss fc")},
	}))
	post(b, BarWindow, `{"action": "click", "id": "card/fc/click", "event": {"alt": true, "middle": true}}`)
	if got, want := calls.event("fake open fc"), (components.ClickEvent{Alt: true, Middle: true}); !calls.has("fake open fc") || got != want {
		t.Errorf("the card's Click was clicked %+v, want %+v", got, want)
	}
	post(b, BarWindow, `{"action": "click", "id": "card/fc/dismiss"}`)
	if !calls.has("fake dismiss fc") || calls.count("fake open fc") != 1 {
		t.Error("a click on × didn't call only the card's Dismiss")
	}
	for _, message := range []string{
		`{"action": "click"}`, `{"action": "click", "id": "card/fc/click", "path": "/etc"}`,
		`{"action": "exec", "id": "card/fc/click"}`, `"click"`, `{"action": "click", "id": "card-fc"}`,
	} {
		post(b, BarWindow, message)
	}
	post(b, MenuWindow+40, `{"action": "click", "id": "card/fc/click"}`)
	b.Render(nil)
	post(b, BarWindow, `{"action": "click", "id": "card/fc/click"}`)
	if n := calls.count("fake open fc"); n != 1 {
		t.Errorf("the card's Click ran %d times, want only the once it was clicked", n)
	}
}

// While a Click runs, the page is told it's busy, with every draw too, and a click on
// it meanwhile is dropped; once it's done, the page is told it isn't.
func TestBusyMarks(t *testing.T) {
	b, f := fakeBar(t, theme.Theme{})
	started, release, done := make(chan bool, 2), make(chan bool), make(chan bool)
	t.Cleanup(func() { close(release) })
	cards := components.Cards([]components.Basic{{ID: "fb", Title: "Busy", Click: func(components.ClickEvent) {
		started <- true
		<-release
	}}})
	b.Render(cards)
	go func() { post(b, BarWindow, `{"action": "click", "id": "card/fb/click"}`); done <- true }()
	<-started
	if got := f.last(BarWindow); got != `setBusy(["card/fb/click"])` {
		t.Errorf("while the Click runs, the bar ran %s", got)
	}
	b.Render(cards)
	if got := f.last(BarWindow); !strings.HasPrefix(got, "update(") || !strings.HasSuffix(got, `; setBusy(["card/fb/click"])`) {
		t.Errorf("drawn while the Click runs, the bar ran %s", got)
	}
	again := make(chan bool)
	go func() { post(b, BarWindow, `{"action": "click", "id": "card/fb/click"}`); again <- true }()
	select {
	case <-again:
	case <-time.After(time.Second):
		t.Fatal("a click on a running Click ran it again")
	}
	release <- true
	<-done
	if got := f.last(BarWindow); got != "setBusy([])" {
		t.Errorf("once the Click is done, the bar ran %s", got)
	}
	if len(started) != 0 {
		t.Error("a click on a running Click ran it again")
	}
}

// In a theme that draws a card's menu, its ⋯ loads the menu's page, which is shown
// once it's ready, the ⋯ busy until the menu's closed and what was chosen has run.
// Choosing an item calls its action's Click, with the keys held but never the middle
// button, and hides the menu; one that can't be chosen leaves it open; Escape hides
// it; and one whose card a render has since taken off is dropped. A middle click on
// the ⋯ shows nothing.
func TestThemeMenu(t *testing.T) {
	b, f := fakeBar(t, defaultTheme(t))
	b.Render(components.Cards([]components.Basic{{ID: "fm", Title: "Menu", Click: record("fake open fm"), Actions: []components.Action{
		{ID: "info", Title: "Info"}, {ID: "go", Title: "Go", Color: "green", Click: record("fake go fm")},
	}}}))
	// opens clicks the ⋯ and waits for the menu to show. What it returns is done once
	// the menu's closed and what was chosen has run.
	opens := func(t *testing.T) chan bool {
		t.Helper()
		loads, done := f.loadCount(MenuWindow), make(chan bool)
		go func() { post(b, BarWindow, `{"action": "click", "id": "card/fm/menu"}`); done <- true }()
		eventually(t, "the menu's page to load", func() bool { return f.loadCount(MenuWindow) > loads })
		if page := f.page(MenuWindow); !strings.Contains(page, ">Info<") || !strings.Contains(page, ">Go<") {
			t.Errorf("the menu's page is %s", page)
		}
		post(b, MenuWindow, `{"action": "ready"}`)
		if !f.showing(MenuWindow) {
			t.Fatal("the menu's page, ready, isn't shown")
		}
		if got := f.last(BarWindow); got != `setBusy(["card/fm/menu"])` {
			t.Errorf("with the menu open, the bar ran %s", got)
		}
		return done
	}
	closes := func(t *testing.T, done chan bool) {
		t.Helper()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("the menu's Click didn't finish")
		}
		if f.showing(MenuWindow) {
			t.Error("the menu is still shown")
		}
		if got := f.last(BarWindow); got != "setBusy([])" {
			t.Errorf("with the menu closed, the bar ran %s", got)
		}
	}

	done := opens(t)
	post(b, MenuWindow, `{"action": "choose", "id": "0"}`)
	if !f.showing(MenuWindow) {
		t.Fatal("choosing an item that can't be chosen closed the menu")
	}
	post(b, MenuWindow, `{"action": "choose", "id": "1", "event": {"alt": true, "middle": true}}`)
	closes(t, done)
	if got, want := calls.event("fake go fm"), (components.ClickEvent{Alt: true}); !calls.has("fake go fm") || got != want {
		t.Errorf("Go, chosen with ⌥ held, was clicked %+v, want %+v", got, want)
	}

	done = opens(t)
	post(b, MenuWindow, `{"action": "close"}`)
	closes(t, done)

	done = opens(t)
	b.Render(nil)
	post(b, MenuWindow, `{"action": "choose", "id": "1"}`)
	closes(t, done)
	if n := calls.count("fake go fm"); n != 1 || calls.has("fake open fm") {
		t.Errorf("Go ran %d times, want only the once it was chosen while its card was there", n)
	}

	b.Render(components.Cards([]components.Basic{{ID: "fm", Actions: []components.Action{{ID: "go", Title: "Go"}}}}))
	loads := f.loadCount(MenuWindow)
	post(b, BarWindow, `{"action": "click", "id": "card/fm/menu", "event": {"middle": true}}`)
	if f.loadCount(MenuWindow) != loads {
		t.Error("a middle click on the ⋯ showed the menu")
	}
}

// In a theme without a menu of its own, a card's ⋯ pops up the native menu of its
// actions, each with its colour, and greyed out without a Click. Choosing one calls
// its Click with the keys held, and not its card's nor another card's action with the
// same id; choosing nothing does nothing; and choosing one whose card a render took
// off while the menu was open is dropped. A middle click on the ⋯ shows nothing.
func TestNativeMenu(t *testing.T) {
	b, f := fakeBar(t, theme.Theme{})
	chosen, then := 0, func() {}
	f.choose = func([]shim.MenuItem) shim.Chosen {
		then()
		return shim.Chosen{Index: chosen, Modifiers: shim.Modifiers{Cmd: true}}
	}
	cards := components.Cards([]components.Basic{
		{ID: "nm", Title: "Menu", Click: record("fake open nm"), Actions: []components.Action{
			{ID: "run", Title: "Run", Color: "green", Click: record("fake run nm")}, {ID: "info", Title: "Info"},
		}},
		{ID: "other", Title: "Other", Actions: []components.Action{{ID: "run", Title: "Run", Click: record("fake run other")}}},
	})
	b.Render(cards)
	post(b, BarWindow, `{"action": "click", "id": "card/nm/menu"}`)
	if got, want := calls.event("fake run nm"), (components.ClickEvent{Cmd: true}); !calls.has("fake run nm") || got != want {
		t.Errorf("Run, chosen with ⌘ held, was clicked %+v, want %+v", got, want)
	}
	if calls.has("fake open nm") || calls.has("fake run other") {
		t.Error("choosing from the menu opened its card, or ran another card's action")
	}
	want := [][]shim.MenuItem{{{Title: "Run", Color: "green", Enabled: true}, {Title: "Info", Color: "gray"}}}
	post(b, BarWindow, `{"action": "click", "id": "card/nm/menu", "event": {"middle": true}}`)
	if f.mu.Lock(); !slices.EqualFunc(f.menus, want, slices.Equal) {
		t.Errorf("the menus were %+v, want %+v", f.menus, want)
	}
	f.mu.Unlock()

	chosen = -1
	post(b, BarWindow, `{"action": "click", "id": "card/nm/menu"}`)
	chosen, then = 0, func() { b.Render(cards[1:]) }
	post(b, BarWindow, `{"action": "click", "id": "card/nm/menu"}`)
	if n := calls.count("fake run nm"); n != 1 || calls.has("fake run other") {
		t.Errorf("Run ran %d times, want only the once it was chosen while its card was there", n)
	}
}
