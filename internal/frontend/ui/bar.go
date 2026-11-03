package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	options  Options
	windows  Windows // what's asked of its windows, once it's open
	onScreen bool    // it's Open's, its windows the screen's, rather than OpenIn's

	mu      sync.Mutex
	cards   []components.Card
	content string  // the cards rendered, for the bar's page once it's ready
	endings endings // the cards that end, and when each has completed
	bar     *page
	pages   map[int]*page    // the bar's and each open report's, by window
	reports map[string]*page // each open report's, by the id it was shown with
	last    int              // the last window id given out

	popup popup // the theme's menu that's open, if any
}

// opened is whether Open has made the bar: there's one, and the shim's handlers are its.
var opened atomic.Bool

// Open creates the bar, places it and loads its page. Call it once, after Init and
// before Run, on the main goroutine.
func Open(options Options) (*Bar, error) { return open(options, shimNative{}) }

// open is Open, with w for what's asked of the windows once the bar is open.
func open(options Options, w Windows) (*Bar, error) {
	if !opened.CompareAndSwap(false, true) {
		return nil, errors.New("ui: the bar is already open")
	}
	html, err := barPage(&options)
	if err != nil {
		opened.Store(false)
		return nil, err
	}
	b := newBar(options, w)
	b.onScreen = true
	shim.CreateBar(BarWindow)
	shim.SetLevel(BarWindow, options.Level)
	b.Place()
	shim.OnScreensChanged(func() { b.Place() })
	shim.OnNavigate(b.navigate)
	shim.OnWindowClosed(b.closed)
	shim.OnMessage(func(window int, message []byte) { go b.received(window, message) })
	shim.OnPopupDismissed(func(window int) {
		if window == MenuWindow {
			b.popup.finish(noChoice)
		}
	})
	b.mu.Lock()
	b.bar.load(html)
	b.mu.Unlock()
	return b, nil
}

// OpenIn is a bar whose windows are w rather than the screen's: nothing is put on
// screen, and the pages w shows post what's done in them with Post. It's for showing
// the bar's pages somewhere else, such as in a browser, as the preview does. Unlike
// Open, it needs no Init or Run, and a program can have any number of them; Place
// does nothing for one.
func OpenIn(options Options, w Windows) (*Bar, error) {
	html, err := barPage(&options)
	if err != nil {
		return nil, err
	}
	b := newBar(options, w)
	b.mu.Lock()
	b.bar.load(html)
	b.mu.Unlock()
	return b, nil
}

// Post carries out message, which the page in window posted, as the bar does what its
// pages on screen post: a click calls its Click before Post returns. It's for a bar
// from OpenIn. Safe from any goroutine.
func (b *Bar) Post(window int, message []byte) { b.received(window, message) }

// newBar is a bar with options and no cards, and no reports yet, shown through w: the
// first report will take the window id after the menu's.
func newBar(options Options, w Windows) *Bar {
	b := &Bar{options: options, windows: w, reports: map[string]*page{}, last: MenuWindow}
	b.bar = newPage(w, BarWindow, "the bar", pageMessages, b.barMessage)
	menu := newPage(w, MenuWindow, "the menu", menuMessages, b.menuMessage)
	b.pages = map[int]*page{BarWindow: b.bar, MenuWindow: menu}
	b.show(nil) // nothing else has b yet, so needs no lock: its page shows no cards once ready
	return b
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
	if err := components.BarPage(options.Theme.Bar, options.Edge, components.Icons(options.Theme.Icons)).Render(context.Background(), &page); err != nil {
		return "", err
	}
	return page.String(), nil
}

// Render shows cards, of any kinds, and their actions, in the order given. From then,
// clicking one calls its Click; a click on what a render took off the bar is dropped.
// A card that ends has its Complete called once its end has passed, if it's still on
// the bar then, and only once for that end, however many renders show it. If the
// cards can't be rendered, it says why, and nothing changes. Safe from any goroutine.
func (b *Bar) Render(cards []components.Card) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	shown := make([]components.Card, len(cards))
	for i, card := range cards {
		shown[i] = given(card)
	}
	return b.show(shown)
}

// RenderCard replaces the card on the bar with card's ID by card, in its place, and
// leaves the rest as they are, as a Render of them all with only it changed would.
// It reports whether there was one; if not, or if card can't be rendered, nothing
// changes. Safe from any goroutine.
func (b *Bar) RenderCard(card components.Card) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	card = given(card)
	id := card.Frame().ID
	i := slices.IndexFunc(b.cards, func(c components.Card) bool { return c.Frame().ID == id })
	if i < 0 {
		return false, nil
	}
	shown := slices.Clone(b.cards)
	shown[i] = card
	return true, b.show(shown)
}

// show renders cards, as the bar keeps them, and has the bar show them from then:
// its page once it's ready, and its clicks call the Clicks they were rendered with.
// If they can't be rendered, nothing changes. Call it with the bar's lock held.
func (b *Bar) show(cards []components.Card) error {
	var html strings.Builder
	clicks := &components.Clicks{}
	ctx := components.WithClicks(context.Background(), clicks)
	ctx = components.WithIcons(ctx, components.Icons(b.options.Theme.Icons))
	if err := components.BarContent(cards, b.menu).Render(ctx, &html); err != nil {
		return err
	}
	b.cards, b.content, b.bar.clicks = cards, html.String(), clicks
	b.endings.schedule(b.cards, b.complete)
	b.draw()
	return nil
}

// shownCard is a card as the bar has it: the card, with the Frame it had when the bar
// was given it, so that everything the bar does with it, drawing it, its menu, and
// ending it, goes by the same frame.
type shownCard struct {
	components.Card
	frame components.Frame
}

func (c shownCard) Frame() components.Frame { return c.frame }

// given is card as the bar keeps it, with its Frame as it is now. Call it with the
// bar's lock held.
func given(card components.Card) components.Card {
	return shownCard{card, card.Frame()}
}

// draw has the bar's page show what the bar was last rendered with, if it's ready;
// it's called again once it is. Call it with the bar's lock held.
func (b *Bar) draw() {
	if !b.bar.ready {
		return
	}
	// Marshalled, the html is a JS string; <, > and & are escaped in it.
	js, _ := json.Marshal(b.content)
	// The busy marks go with it, in one go, so what's busy never shows otherwise.
	b.bar.eval("update(" + string(js) + "); " + b.bar.busyJS())
}

// Place puts the bar along its edge of its screen, and returns where it went. The bar
// does this itself when it opens and when the screens change.
func (b *Bar) Place() shim.Rect {
	if !b.onScreen {
		return shim.Rect{}
	}
	return shim.PlaceBar(BarWindow, b.options.Screen, b.options.Edge == "top", float64(b.options.Height))
}
