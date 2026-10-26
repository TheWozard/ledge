package ui

import (
	"encoding/json"
	"log"
	"maps"
	"slices"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
)

// page is what a window shows, the bar, a report or a card's menu: a page that posts
// what's done in it. The bar's and a report's are loaded once, what they show,
// rendered in Go, is morphed into them, and they post the ids of what's clicked in
// them; each has the Clicks of what it was last given to show, which of them are
// running, and whether it's loaded. Its fields are guarded by the bar's lock, but
// messages and handle, which never change.
type page struct {
	native native
	window int
	name   string // what the log calls it: "the bar", "report window 3" or "the menu"
	// messages is what the page can post, each action with whether it takes an id,
	// and handle carries out one, once it's been checked against them.
	messages map[string]bool
	handle   func(m message)

	hasPage bool     // a page has been loaded into it, ready or not
	ready   bool     // it's posted "ready" since it was last loaded
	queued  []string // the JS to run in it once it's ready
	clicks  *components.Clicks
	running map[string]bool // the ids of its Clicks running, which it marks busy

	// A report's page has the id it was shown for, what it shows, so a block of it can
	// be replaced, and the stylesheet it was loaded with, so it's loaded again in
	// another.
	report string
	css    string
	title  string
	blocks []templ.Component
}

// pageMessages is what the bar's and a report's page can post, each action with
// whether it takes an id: "ready", once the page has loaded, and "click", with the id
// of a Click in it.
var pageMessages = map[string]bool{"ready": false, "click": true}

// newPage is window's page, not loaded yet, shown through n, which can post messages,
// each carried out by handle.
func newPage(n native, window int, name string, messages map[string]bool, handle func(p *page, m message)) *page {
	p := &page{native: n, window: window, name: name, messages: messages, clicks: &components.Clicks{}, running: map[string]bool{}}
	p.handle = func(m message) { handle(p, m) }
	return p
}

// load replaces p's page by html. Until the new page is ready, what's evaluated in it
// waits; what was waiting for the old one is dropped.
func (p *page) load(html string) {
	p.hasPage, p.ready, p.queued = true, false, nil
	p.native.LoadHTML(p.window, html)
}

// eval runs js in p once it's ready: now if it is.
func (p *page) eval(js string) {
	if p.ready {
		p.native.Eval(p.window, js)
		return
	}
	p.queued = append(p.queued, js)
}

// loaded marks p ready, and runs what was waiting for it.
func (p *page) loaded() {
	p.ready = true
	for _, js := range p.queued {
		p.native.Eval(p.window, js)
	}
	p.queued = nil
}

// busyJS is the JS that has p mark busy what's running in it, and nothing else.
func (p *page) busyJS() string {
	ids := slices.Sorted(maps.Keys(p.running))
	if ids == nil {
		ids = []string{} // [], not null
	}
	// Marshalled, the ids are a JS array of strings; <, > and & are escaped in them.
	js, _ := json.Marshal(ids)
	return "setBusy(" + string(js) + ")"
}

// received carries out one message from the page in window, if it's one the page can
// post.
func (b *Bar) received(window int, data []byte) {
	b.mu.Lock()
	p := b.pages[window]
	b.mu.Unlock()
	if p == nil {
		log.Printf("ui: window %d, which isn't open, sent %s", window, data)
		return
	}
	m, err := readMessage(data, p.messages)
	if err != nil {
		log.Printf("ui: %s sent %s: %v", p.name, data, err)
		return
	}
	p.handle(m)
}

// barMessage carries out a message from the bar's page: once it's ready, it's drawn
// into, and a click calls the Click it names.
func (b *Bar) barMessage(p *page, m message) {
	if m.Action == "ready" {
		b.mu.Lock()
		defer b.mu.Unlock()
		p.loaded()
		b.draw()
		return
	}
	b.click(p, m)
}

// reportMessage carries out a message from a report's page: once it's ready, what
// was waiting for it is run, what it shows and its busy marks among them, and a click
// calls the Click it names.
func (b *Bar) reportMessage(p *page, m message) {
	if m.Action == "ready" {
		b.mu.Lock()
		defer b.mu.Unlock()
		p.loaded()
		return
	}
	b.click(p, m)
}

// click calls the Click in p that m, a click, names, if there is one.
func (b *Bar) click(p *page, m message) {
	b.mu.Lock()
	click := p.clicks.Click(m.ID)
	b.mu.Unlock()
	if click == nil {
		log.Printf("ui: %s clicked %q, which isn't in it", p.name, m.ID)
		return
	}
	b.run(p, m.ID, click, m.clickEvent())
}

// run calls click, the Click with id clickID in p, with how it was clicked, unless
// it's already running: then the click is dropped. While it runs, p marks busy what
// was clicked.
func (b *Bar) run(p *page, clickID string, click components.Click, event components.ClickEvent) {
	b.mu.Lock()
	if _, ok := p.running[clickID]; ok {
		b.mu.Unlock()
		log.Printf("ui: %q is still running, so the click on it is dropped", clickID)
		return
	}
	p.running[clickID] = true
	p.eval(p.busyJS())
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(p.running, clickID)
		p.eval(p.busyJS())
		b.mu.Unlock()
	}()
	click(event)
}
