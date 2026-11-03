package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a-h/templ"

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

// A Click that's running isn't run again until it's done: a click on it meanwhile is
// dropped. While it runs, it's busy, and nothing else is.
func TestRunOnce(t *testing.T) {
	b := newBar(Options{}, &fakeNative{})
	p := b.bar // not ready, so it marks nothing on a page
	busy := func() string { b.mu.Lock(); defer b.mu.Unlock(); return p.busyJS() }
	if got := busy(); got != "setBusy([])" {
		t.Errorf("with nothing running, %s", got)
	}
	var runs atomic.Int32
	started, release, done := make(chan bool), make(chan bool), make(chan bool)
	slow := func(components.ClickEvent) { runs.Add(1); started <- true; <-release }
	go func() { b.run(p, "card/a/click", slow, components.ClickEvent{}); done <- true }()
	<-started
	go func() { b.run(p, "card/a/menu", slow, components.ClickEvent{}); done <- true }()
	<-started
	if got := busy(); got != `setBusy(["card/a/click","card/a/menu"])` {
		t.Errorf("while two run, %s", got)
	}
	b.run(p, "card/a/click", func(components.ClickEvent) { t.Error("a click on a running Click ran it again") }, components.ClickEvent{})
	close(release)
	for range 2 {
		<-done
	}
	if got := busy(); got != "setBusy([])" {
		t.Errorf("once they're done, %s", got)
	}
	b.run(p, "card/a/click", func(e components.ClickEvent) {
		if e.Alt {
			runs.Add(1)
		}
	}, components.ClickEvent{Alt: true})
	if runs.Load() != 3 {
		t.Errorf("ran %d times, want 3", runs.Load())
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
		`document.querySelector("li").className`:           `"basic"`,
		`document.querySelector("li").dataset.color`:       `"#e0362c"`,
		`document.querySelector("li").dataset.menu`:        `"card/deploy/menu"`,
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

	if found, err := testBar.RenderCard(components.Basic{ID: "rc-b", Title: "Changed", Click: record("open rc-b")}); !found || err != nil {
		t.Fatalf("RenderCard(rc-b) = %v, %v", found, err)
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

	if found, _ := testBar.RenderCard(components.Basic{ID: "rc-none", Title: "None"}); found {
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

// A card that comes back while it's leaving stays, and arrives again.
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
	if got := eval(t, `[window.back.hasAttribute("data-new"), document.getElementById("card-other").hasAttribute("data-new")].join()`); got != `"true,false"` {
		t.Errorf("come back, it and the card that stayed are new: %s, want it alone", got)
	}
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

// The bar draws into its page only once the page is ready, and again whenever it's
// given cards, each time with the busy marks, and again once a page loaded afresh
// says it's ready.
func TestDrawsOnceReady(t *testing.T) {
	f := &fakeNative{}
	b := newBar(Options{}, f)
	cards := components.Cards([]components.Basic{{ID: "early", Title: "Early"}})
	b.Render(cards)
	if ran := f.ran(BarWindow); len(ran) != 0 {
		t.Fatalf("before its page was ready, the bar ran %q", ran)
	}
	// drawn is whether the bar's page has had n things run in it, the last drawing
	// the card and marking nothing busy.
	drawn := func(t *testing.T, when string, n int) {
		t.Helper()
		js := f.last(BarWindow)
		if !strings.HasPrefix(js, "update(") || !strings.HasSuffix(js, "; setBusy([])") || !strings.Contains(js, `id=\"card-early\"`) {
			t.Fatalf("%s, the bar ran %s", when, js)
		}
		if got := len(f.ran(BarWindow)); got != n {
			t.Errorf("%s, the bar has run %d things, want %d", when, got, n)
		}
	}
	post(b, BarWindow, `{"action": "ready"}`)
	drawn(t, "once ready", 1)
	b.Render(cards)
	drawn(t, "rendered again", 2)
	post(b, BarWindow, `{"action": "ready"}`)
	drawn(t, "ready again", 3)
}

// The bar takes a card's Frame once, when it's given it, however many times it draws
// it, and goes by that frame: drawing it, and ending it.
func TestFrameOnce(t *testing.T) {
	b, f := fakeBar(t, theme.Theme{})
	card := &framed{Basic: components.Basic{ID: "once", Title: "Once"}}
	b.Render([]components.Card{card})
	b.RenderCard(components.Basic{ID: "other"}) // not on the bar: nothing changes
	post(b, BarWindow, `{"action": "ready"}`)
	if n := card.frames.Load(); n != 1 {
		t.Errorf("the bar took the card's Frame %d times, want once", n)
	}
	if js := f.last(BarWindow); !strings.Contains(js, `data-color=\"#000001\"`) {
		t.Errorf("the bar didn't draw the frame it took: %s", js)
	}
}

// framed is a Basic card that counts the times its Frame is taken, and has a colour
// of its own each time.
type framed struct {
	components.Basic
	frames atomic.Int32
}

func (c *framed) Frame() components.Frame {
	f := c.Basic.Frame()
	f.Color = fmt.Sprintf("#%06d", c.frames.Add(1))
	return f
}

// What can't be rendered is said, and changes nothing: a Render or RenderCard leaves
// the bar's cards, its page and its Clicks as they were, a ShowReport opens nothing,
// and a ShowReportBlock leaves its report as it was.
func TestRenderErrors(t *testing.T) {
	b, f := fakeBar(t, theme.Theme{})
	broken := templ.ComponentFunc(func(context.Context, io.Writer) error { return errors.New("broken") })
	if err := b.Render(components.Cards([]components.Basic{{ID: "ok", Title: "OK", Click: record("fake open ok")}})); err != nil {
		t.Fatal(err)
	}
	drawn := len(f.ran(BarWindow))
	if err := b.Render([]components.Card{failing{Basic: components.Basic{ID: "bad"}, body: broken}}); err == nil {
		t.Error("a card that can't be rendered rendered")
	}
	if found, err := b.RenderCard(failing{Basic: components.Basic{ID: "ok"}, body: broken}); !found || err == nil {
		t.Errorf("RenderCard of a card that can't be rendered = %v, %v", found, err)
	}
	post(b, BarWindow, `{"action": "click", "id": "card/ok/click"}`)
	if n := len(f.ran(BarWindow)); n != drawn+2 || !calls.has("fake open ok") {
		t.Errorf("after what couldn't be rendered, the bar ran %d more things, and its card's Click ran: %v; want only the click's busy marks, and it to",
			n-drawn, calls.has("fake open ok"))
	}

	if err := b.ShowReport("fe", "Broken", []templ.Component{broken}); err == nil {
		t.Error("a report that can't be rendered was shown")
	}
	if f.loadCount(MenuWindow+1) != 0 || f.showing(MenuWindow+1) {
		t.Error("a report that couldn't be rendered opened a window")
	}
	if err := b.ShowReport("fe", "Fine", []templ.Component{components.Block{ID: "x", Content: components.Paragraph(components.Text("Fine"))}}); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	w := b.reports["fe"].window
	b.mu.Unlock()
	ran := len(f.ran(w))
	if found, err := b.ShowReportBlock("fe", components.Block{ID: "x", Content: broken}); !found || err == nil {
		t.Errorf("ShowReportBlock of a block that can't be rendered = %v, %v", found, err)
	}
	b.mu.Lock()
	kept := components.ReportContent(b.reports["fe"].blocks).Render(context.Background(), io.Discard)
	b.mu.Unlock()
	if len(f.ran(w)) != ran || kept != nil {
		t.Error("a block that couldn't be rendered changed its report")
	}
}

// failing is a Basic card with a body of its own, for TestRenderErrors.
type failing struct {
	components.Basic
	body templ.Component
}

func (c failing) Body() templ.Component { return c.body }
