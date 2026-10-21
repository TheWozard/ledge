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
	clicks  map[string]components.Click // the Clicks on cards, by the page's id for them
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
	b := &Bar{options: options, shown: map[string]bool{}, reports: newReportWindows()}
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
		}
	})
	shim.OnPopupDismissed(func(window int) {
		if window == MenuWindow {
			b.popup.finish(-1)
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
	b.clicks = components.Clicks(b.cards)
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
	b.clicks = components.Clicks(b.cards)
	b.endings.schedule(b.cards, b.complete)
	b.mu.Unlock()
	b.draw()
	return true
}

// draw renders what the bar has into its page, once the page is ready.
func (b *Bar) draw() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready {
		return
	}
	var fresh map[string]int
	fresh, b.shown = arrivals(b.cards, b.shown)
	var html strings.Builder
	if err := components.BarContent(b.cards, fresh).Render(context.Background(), &html); err != nil {
		log.Printf("ui: rendering the bar: %v", err)
		return
	}
	// Marshalled, the html is a JS string; <, > and & are escaped in it.
	js, _ := json.Marshal(html.String())
	shim.Eval(BarWindow, "update("+string(js)+")")
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
