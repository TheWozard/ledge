package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// Bar is the bar window and the report windows.
type Bar struct {
	options  Options
	handlers Handlers

	mu     sync.Mutex
	pills  []components.Pill
	cards  []components.Card
	shown  map[string]bool // the element ids of the pills and cards on the bar, to tell which are new
	ready  bool            // the page has loaded and can take updates
	redraw *time.Timer     // for the next relative time or scheduled colour to change

	reports    map[string]int // each open report's window, by the id it was shown with
	lastWindow int            // the last window id given out
}

// opened is whether Open has made the bar: there's one, and the shim's handlers are its.
var opened atomic.Bool

// Open creates the bar, places it and loads its page. Call it once, after Init and
// before Run, on the main goroutine.
func Open(options Options, handlers Handlers) (*Bar, error) {
	if opened.Load() {
		return nil, errors.New("ui: the bar is already open")
	}
	if options.Theme == (theme.Theme{}) {
		var err error
		if options.Theme, err = theme.Builtin(theme.Builtins.Default()); err != nil {
			return nil, err
		}
	}
	var page bytes.Buffer
	if err := components.BarPage(options.Theme.Bar, options.Edge).Render(context.Background(), &page); err != nil {
		return nil, err
	}
	opened.Store(true)
	b := &Bar{options: options, handlers: handlers, shown: map[string]bool{}, reports: map[string]int{}, lastWindow: BarWindow}
	shim.CreateBar(BarWindow)
	shim.SetLevel(BarWindow, options.Level)
	b.Place()
	shim.OnScreensChanged(func() { b.Place() })
	shim.OnNavigate(b.navigate)
	shim.OnWindowClosed(b.closed)
	shim.OnMessage(func(window int, message []byte) {
		if window == BarWindow {
			go b.handle(message)
		}
	})
	shim.LoadHTML(BarWindow, page.String())
	return b, nil
}

// Render shows pills (in order of title) and cards (in the order given). Safe from
// any goroutine.
func (b *Bar) Render(pills []components.Pill, cards []components.Card) {
	pills = slices.Clone(pills)
	slices.SortStableFunc(pills, func(a, b components.Pill) int { return strings.Compare(a.Title, b.Title) })
	b.mu.Lock()
	b.pills, b.cards = pills, slices.Clone(cards)
	b.mu.Unlock()
	b.draw()
}

// draw renders what the bar has into its page, once the page is ready, and sets a
// timer for the next time it would look different: a card's relative time going stale
// (once a minute) or a pill's scheduled colour change. With neither, it waits for the
// next Render.
func (b *Bar) draw() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready {
		return
	}
	now := time.Now()
	// Each pill and card new since the last draw gets its place among the new ones of
	// its kind, for the theme's entrance animation.
	fresh := map[string]int{}
	shown := map[string]bool{}
	var newPills, newCards int
	mark := func(id string, count *int) {
		if !b.shown[id] {
			fresh[id] = *count
			*count++
		}
		shown[id] = true
	}
	for _, pill := range b.pills {
		mark(pill.ElementID(), &newPills)
	}
	for _, card := range b.cards {
		mark(card.ElementID(), &newCards)
	}
	b.shown = shown

	var html strings.Builder
	if err := components.BarContent(b.pills, b.cards, now, fresh).Render(context.Background(), &html); err != nil {
		log.Printf("ui: rendering the bar: %v", err)
		return
	}
	// Marshalled, the html is a JS string; <, > and & are escaped in it.
	js, _ := json.Marshal(html.String())
	shim.Eval(BarWindow, "update("+string(js)+")")

	var next time.Time
	if len(b.cards) > 0 {
		next = now.Add(time.Minute)
	}
	for _, pill := range b.pills {
		if at, ok := pill.NextChange(now); ok && (next.IsZero() || at.Before(next)) {
			next = at.Add(50 * time.Millisecond)
		}
	}
	if b.redraw != nil {
		b.redraw.Stop()
		b.redraw = nil
	}
	if !next.IsZero() {
		b.redraw = time.AfterFunc(time.Until(next), b.draw)
	}
}

// Place puts the bar along its edge of its screen, and returns where it went. The bar
// does this itself when it opens and when the screens change.
func (b *Bar) Place() shim.Rect {
	return shim.PlaceBar(BarWindow, b.options.Screen, b.options.Edge == "top", float64(b.options.Height))
}

// message is what the bar page posts: an action, and the id it applies to.
type message struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

// readMessage decodes one message from the page, refusing anything else: an unknown
// action or property, an id with "ready", or none with anything else.
func readMessage(data []byte) (message, error) {
	var m message
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, err
	}
	if decoder.More() {
		return m, errors.New("more than one value")
	}
	switch {
	case !slices.Contains([]string{"ready", "dismiss", "open", "run"}, m.Action):
		return m, fmt.Errorf("no action %q", m.Action)
	case m.Action == "ready" && m.ID != "":
		return m, errors.New("ready takes no id")
	case m.Action != "ready" && m.ID == "":
		return m, fmt.Errorf("%s needs an id", m.Action)
	}
	return m, nil
}

// handle carries out one message from the page.
func (b *Bar) handle(data []byte) {
	m, err := readMessage(data)
	if err != nil {
		log.Printf("ui: the bar sent %s: %v", data, err)
		return
	}
	call := func(handle func(string)) {
		if handle != nil {
			handle(m.ID)
		}
	}
	switch m.Action {
	case "ready":
		b.mu.Lock()
		b.ready = true
		b.shown = map[string]bool{} // a fresh page shows every pill and card arriving
		b.mu.Unlock()
		b.draw()
	case "dismiss":
		call(b.handlers.Dismiss)
	case "open":
		call(b.handlers.Open)
	case "run":
		call(b.handlers.Run)
	}
}
