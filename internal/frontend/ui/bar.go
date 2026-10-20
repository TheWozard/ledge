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
	cards  []components.Card
	clicks map[string]components.Click // the Clicks on cards, by the page's id for them
	shown  map[string]bool             // the element ids of the cards and actions on the bar, to tell which are new
	ready  bool                        // the page has loaded and can take updates

	// endings are the timers for the cards that end, each card's Complete when it's
	// due, by card id; completed is the end each card on the bar has completed at.
	endings   map[string]*time.Timer
	completed map[string]time.Time

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

// Render shows cards, of any kinds, and their actions: highest priority first, and
// those of the same priority in the order given. From then, clicking one calls its
// Click; a click on what a render took off the bar is dropped. A card that ends has
// its Complete called once its end has passed, if it's still on the bar then, and
// only once for that end, however many renders show it. Safe from any goroutine.
func (b *Bar) Render(cards []components.Card) {
	b.mu.Lock()
	b.cards = slices.Clone(cards)
	components.SortByPriority(b.cards)
	b.clicks = components.Clicks(b.cards)
	b.schedule()
	b.mu.Unlock()
	b.draw()
}

// schedule sets a timer for each card on the bar that ends and hasn't completed at
// that end, stopping those it set before. b.mu is held.
func (b *Bar) schedule() {
	for _, timer := range b.endings {
		timer.Stop()
	}
	endings, completed := map[string]*time.Timer{}, map[string]time.Time{}
	for _, card := range b.cards {
		frame := card.Frame()
		if frame.Complete == nil || frame.Ends.IsZero() {
			continue
		}
		if at, ok := b.completed[frame.ID]; ok && at.Equal(frame.Ends) {
			completed[frame.ID] = at
			continue
		}
		id, ends := frame.ID, frame.Ends
		endings[id] = time.AfterFunc(time.Until(ends), func() { b.complete(id, ends) })
	}
	b.endings, b.completed = endings, completed
}

// complete calls the Complete of card id, if it's still on the bar ending at ends and
// hasn't already.
func (b *Bar) complete(id string, ends time.Time) {
	b.mu.Lock()
	var complete components.Click
	for _, card := range b.cards {
		if frame := card.Frame(); frame.ID == id && frame.Ends.Equal(ends) {
			if _, done := b.completed[id]; !done {
				complete, b.completed[id] = frame.Complete, ends
			}
		}
	}
	b.mu.Unlock()
	if complete != nil {
		complete()
	}
}

// draw renders what the bar has into its page, once the page is ready.
func (b *Bar) draw() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready {
		return
	}
	// Each card new since the last draw gets its place among the new ones, for the
	// theme's entrance animation, and so does each action new to a card that was
	// already there. A new card's actions arrive with it.
	fresh := map[string]int{}
	shown := map[string]bool{}
	var newCards, newActions int
	mark := func(id string, count *int) {
		if !b.shown[id] {
			fresh[id] = *count
			*count++
		}
		shown[id] = true
	}
	for _, card := range b.cards {
		frame := card.Frame()
		arriving := !b.shown[frame.ElementID()]
		mark(frame.ElementID(), &newCards)
		for _, action := range frame.Actions {
			if arriving {
				shown[action.ElementID()] = true
			} else {
				mark(action.ElementID(), &newActions)
			}
		}
	}
	b.shown = shown

	var html strings.Builder
	if err := components.BarContent(b.cards, fresh).Render(context.Background(), &html); err != nil {
		log.Printf("ui: rendering the bar: %v", err)
		return
	}
	// Marshalled, the html is a JS string; <, > and & are escaped in it.
	js, _ := json.Marshal(html.String())
	shim.Eval(BarWindow, "update("+string(js)+")")
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
	case !slices.Contains([]string{"ready", "click"}, m.Action):
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
	switch m.Action {
	case "ready":
		b.mu.Lock()
		b.ready = true
		b.shown = map[string]bool{} // a fresh page shows every card arriving
		b.mu.Unlock()
		b.draw()
	case "click":
		b.mu.Lock()
		click := b.clicks[m.ID]
		b.mu.Unlock()
		if click == nil {
			log.Printf("ui: the bar clicked %q, which isn't on it", m.ID)
			return
		}
		click()
	}
}
