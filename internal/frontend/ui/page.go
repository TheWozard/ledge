package ui

import (
	"encoding/json"
	"log"
	"maps"
	"slices"

	"github.com/TheWozard/ledge-frontend/components"
)

// page is what a window shows, the bar or a report: a page rendered in Go, which posts
// the ids of what's clicked in it. It has the Clicks it was rendered with, which of
// them are running, and whether it's loaded. Its fields are guarded by the bar's lock.
type page struct {
	native  native
	window  int
	name    string   // what the log calls it: "the bar", or "report window 3"
	ready   bool     // it's posted "ready" since it was last loaded
	queued  []string // the JS to run in it once it's ready
	clicks  *components.Clicks
	running map[string]bool // the ids of its Clicks running, which it marks busy
}

// pageMessages is what the bar's and a report's page can post, each action with
// whether it takes an id: "ready", once the page has loaded, and "click", with the id
// of a Click in it.
var pageMessages = map[string]bool{"ready": false, "click": true}

// newPage is window's page, not loaded yet, shown through n.
func newPage(n native, window int, name string) *page {
	return &page{native: n, window: window, name: name, clicks: &components.Clicks{}, running: map[string]bool{}}
}

// load replaces what p shows by html, with clicks, the Clicks in it. Until the new
// page is ready, what's evaluated in it waits.
func (p *page) load(html string, clicks *components.Clicks) {
	p.ready, p.queued, p.clicks = false, nil, clicks
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

// received carries out one message from the page in window, the bar or a report.
func (b *Bar) received(window int, data []byte) {
	b.mu.Lock()
	p := b.pages[window]
	b.mu.Unlock()
	if p == nil {
		log.Printf("ui: window %d, which isn't open, sent %s", window, data)
		return
	}
	m, err := readMessage(data, pageMessages)
	if err != nil {
		log.Printf("ui: %s sent %s: %v", p.name, data, err)
		return
	}
	switch m.Action {
	case "ready":
		b.mu.Lock()
		p.loaded()
		if p == b.bar {
			b.shown = map[string]bool{} // a fresh page shows every card arriving
			b.draw()
		} else {
			p.eval(p.busyJS()) // a fresh page has no busy marks
		}
		b.mu.Unlock()
	case "click":
		b.mu.Lock()
		click := p.clicks.Click(m.ID)
		b.mu.Unlock()
		if click == nil {
			log.Printf("ui: %s clicked %q, which isn't in it", p.name, m.ID)
			return
		}
		b.run(p, m.ID, click, m.clickEvent())
	}
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
