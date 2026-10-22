package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// Bar is the bar window and the report windows.
type Bar struct {
	options Options

	mu      sync.Mutex
	cards   []components.Card
	clicks  *components.Clicks          // the Clicks on the cards as last drawn
	running map[running]string          // the Clicks running, each with the id its page marks busy for it
	shown   map[string]bool             // the element ids of the cards on the bar, to tell which are new
	ready   bool                        // the page has loaded and can take updates
	endings endings                     // the cards that end, and when each has completed

	reports *reportWindows
	popup   popup // the theme's menu that's open, if any
}

// opened is whether Open has made the bar: there's one, and the shim's handlers are its.
var opened atomic.Bool

// Open creates the bar, places it and loads its page. Call it once, after Init and
// before Run, on the main goroutine.
func Open(options Options) (*Bar, error) {
	if !opened.CompareAndSwap(false, true) {
		return nil, errors.New("ui: the bar is already open")
	}
	page, err := barPage(&options)
	if err != nil {
		opened.Store(false)
		return nil, err
	}
	b := &Bar{options: options, clicks: &components.Clicks{}, shown: map[string]bool{}, reports: newReportWindows()}
	shim.CreateBar(BarWindow)
	shim.SetLevel(BarWindow, options.Level)
	b.Place()
	shim.OnScreensChanged(func() { b.Place() })
	shim.OnNavigate(b.navigate)
	shim.OnWindowClosed(b.reports.closed)
	shim.OnMessage(func(window int, message []byte) {
		switch window {
		case BarWindow:
			go b.handle(message)
		case MenuWindow:
			go b.handleMenuPage(message)
		default:
			go b.handleReport(window, message)
		}
	})
	shim.OnPopupDismissed(func(window int) {
		if window == MenuWindow {
			b.popup.finish(noChoice)
		}
	})
	shim.LoadHTML(BarWindow, page)
	return b, nil
}

// barPage is the bar's page for options, giving them the default theme if they have
// none.
func barPage(options *Options) (string, error) {
	if options.Theme == (theme.Theme{}) {
		var err error
		if options.Theme, err = theme.Builtin(theme.Builtins.Default()); err != nil {
			return "", err
		}
	}
	var page bytes.Buffer
	if err := components.BarPage(options.Theme.Bar, options.Edge).Render(context.Background(), &page); err != nil {
		return "", err
	}
	return page.String(), nil
}

// Render shows cards, of any kinds, and their actions, in the order given. From then,
// clicking one calls its Click; a click on what a render took off the bar is dropped.
// A card that ends has its Complete called once its end has passed, if it's still on
// the bar then, and only once for that end, however many renders show it. Safe from
// any goroutine.
func (b *Bar) Render(cards []components.Card) {
	b.mu.Lock()
	b.cards = slices.Clone(cards)
	b.endings.schedule(b.cards, b.complete)
	b.mu.Unlock()
	b.draw()
}

// RenderCard replaces the card on the bar with card's ID by card, in its place, and
// leaves the rest as they are, as a Render of them all with only it changed would.
// It reports whether there was one; if not, nothing changes. Safe from any goroutine.
func (b *Bar) RenderCard(card components.Card) bool {
	b.mu.Lock()
	id := card.Frame().ID
	i := slices.IndexFunc(b.cards, func(c components.Card) bool { return c.Frame().ID == id })
	if i < 0 {
		b.mu.Unlock()
		return false
	}
	b.cards[i] = card
	b.endings.schedule(b.cards, b.complete)
	b.mu.Unlock()
	b.draw()
	return true
}

// draw renders what the bar has into its page, once the page is ready, and from then
// its clicks call the Clicks it was drawn with.
func (b *Bar) draw() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready {
		return
	}
	var fresh map[string]int
	fresh, b.shown = arrivals(b.cards, b.shown)
	var html strings.Builder
	clicks := &components.Clicks{}
	ctx := components.WithClicks(context.Background(), clicks)
	if err := components.BarContent(b.cards, fresh).Render(ctx, &html); err != nil {
		log.Printf("ui: rendering the bar: %v", err)
		return
	}
	b.clicks = clicks
	// Marshalled, the html is a JS string; <, > and & are escaped in it.
	js, _ := json.Marshal(html.String())
	// The busy marks go with it, in one go, so what's busy never shows otherwise.
	shim.Eval(BarWindow, "update("+string(js)+"); "+b.busyJS(BarWindow))
}

// running is a Click that's running: the window it was clicked in, and its id there.
type running struct {
	window  int
	clickID string
}

// run calls click, the Click with id clickID in window, the bar or a report, with
// how it was clicked, unless it's already running: then the click is dropped. While
// it runs, the window's page marks busy what has the id busyID: what was clicked, or
// the ⋯ of a card whose action was chosen.
func (b *Bar) run(window int, clickID, busyID string, click components.Click, event components.ClickEvent) {
	key := running{window, clickID}
	b.mu.Lock()
	if _, ok := b.running[key]; ok {
		b.mu.Unlock()
		log.Printf("ui: %q is still running, so the click on it is dropped", clickID)
		return
	}
	if b.running == nil {
		b.running = map[running]string{}
	}
	b.running[key] = busyID
	b.markBusy(window)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.running, key)
		b.markBusy(window)
		b.mu.Unlock()
	}()
	click(event)
}

// markBusy has window's page mark busy what's running in it: the bar's once it's
// ready, and a report's whenever, as a page that isn't there ignores it. Call it with
// the bar's lock held.
func (b *Bar) markBusy(window int) {
	if window != BarWindow || b.ready {
		shim.Eval(window, b.busyJS(window))
	}
}

// busyJS is the JS that has window's page mark busy what's running in it, and
// nothing else. Call it with the bar's lock held.
func (b *Bar) busyJS(window int) string {
	var ids []string
	for key, id := range b.running {
		if key.window == window {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if ids == nil {
		ids = []string{} // [], not null
	}
	// Marshalled, the ids are a JS array of strings; <, > and & are escaped in them.
	js, _ := json.Marshal(ids)
	return "setBusy(" + string(js) + ")"
}

// arrivals is what's new among cards since the bar showed shown, the element ids of
// the cards on it then. fresh gives each card new since then its place among the new
// ones, for the theme's entrance animation. now is the element ids on the bar with
// cards.
func arrivals(cards []components.Card, shown map[string]bool) (fresh map[string]int, now map[string]bool) {
	fresh, now = map[string]int{}, map[string]bool{}
	for _, card := range cards {
		id := card.Frame().ElementID()
		if !shown[id] {
			fresh[id] = len(fresh)
		}
		now[id] = true
	}
	return fresh, now
}

// Place puts the bar along its edge of its screen, and returns where it went. The bar
// does this itself when it opens and when the screens change.
func (b *Bar) Place() shim.Rect {
	return shim.PlaceBar(BarWindow, b.options.Screen, b.options.Edge == "top", float64(b.options.Height))
}
