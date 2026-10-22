package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"sync"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
)

// ShowReport shows title and blocks in id's report window, in the bar's theme, and
// brings it forward. Each id has its own window: showing an id again replaces what
// its window shows, and one that's been closed opens anew. id is the app's, such as
// the card or action the report is for. A block is any component; components
// has the usual ones. Clicking what's in them, such as a Buttons' button, calls its
// Click, as one on the bar does, until the report is shown again or closed.
func (b *Bar) ShowReport(id, title string, blocks []templ.Component) error {
	b.mu.Lock()
	t := b.options.Theme
	b.mu.Unlock()
	var page bytes.Buffer
	clicks := &components.Clicks{}
	ctx := components.WithClicks(context.Background(), clicks)
	if err := components.ReportPage(t.Report, title, blocks).Render(ctx, &page); err != nil {
		return err
	}
	window := b.reports.show(id, clicks)
	shim.CreateWindow(window, title, 900, 700)
	shim.SetTitle(window, title)
	shim.SetAppearance(window, t.Frame.Appearance)
	shim.SetFrameless(window, t.Frame.Frameless)
	shim.LoadHTML(window, page.String())
	shim.ShowWindow(window)
	return nil
}

// ShowReportBlock replaces the block given blockID with components.Block in id's
// report by block, leaving the rest of the report, its scroll and where its window
// is as they are. It reports whether id's report is open; if it is but has no such
// block, nothing changes. Called just after ShowReport, before the new page has
// loaded, the change can be lost.
func (b *Bar) ShowReportBlock(id, blockID string, block templ.Component) (bool, error) {
	window, ok := b.reports.open(id)
	if !ok {
		return false, nil
	}
	var html strings.Builder
	clicks := &components.Clicks{}
	ctx := components.WithClicks(context.Background(), clicks)
	if err := components.Block(blockID, block).Render(ctx, &html); err != nil {
		return true, err
	}
	b.reports.replaceBlock(window, clicks)
	// Marshalled, the id and html are JS strings; <, > and & are escaped in them.
	idJS, _ := json.Marshal(blockID)
	htmlJS, _ := json.Marshal(html.String())
	// The busy marks go with it, as the block's new elements haven't any.
	b.mu.Lock()
	shim.Eval(window, "replaceBlock("+string(idJS)+", "+string(htmlJS)+"); "+b.busyJS(window))
	b.mu.Unlock()
	return true, nil
}

// reportMessages is what a report's page can post, each action with whether it takes
// an id: only "click", with the id of a Click in it.
var reportMessages = map[string]bool{"click": true}

// handleReport carries out one message from the page in report window window: a click
// calls the Click with that id in it, if it has one.
func (b *Bar) handleReport(window int, data []byte) {
	m, err := readMessage(data, reportMessages)
	if err != nil {
		log.Printf("ui: report window %d sent %s: %v", window, data, err)
		return
	}
	click := b.reports.click(window, m.ID)
	if click == nil {
		log.Printf("ui: report window %d clicked %q, which isn't in it", window, m.ID)
		return
	}
	b.run(window, m.ID, m.ID, click, m.clickEvent())
}

// reportWindows is the window of each report that's open, by the id it was shown
// with, and the Clicks in each. Each report has its own window, its id never reused.
type reportWindows struct {
	mu     sync.Mutex
	byID   map[string]int
	clicks map[int]*components.Clicks // by window
	last   int                              // the last window id given out
}

// newReportWindows is no report windows yet; the first will take the id after the
// menu's.
func newReportWindows() *reportWindows {
	return &reportWindows{byID: map[string]int{}, clicks: map[int]*components.Clicks{}, last: MenuWindow}
}

// show is id's report window, given a new one if it hasn't got one open, now with
// clicks, the Clicks in what it shows, in place of any it had.
func (r *reportWindows) show(id string, clicks *components.Clicks) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	window, ok := r.byID[id]
	if !ok {
		r.last++
		window = r.last
		r.byID[id] = window
	}
	r.clicks[window] = clicks
	return window
}

// replaceBlock gives window's report clicks, the Clicks in a block, in place of those
// the block had.
func (r *reportWindows) replaceBlock(window int, clicks *components.Clicks) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if all := r.clicks[window]; all != nil {
		all.Replace(clicks)
	}
}

// click is the Click with id in window's report, or nil.
func (r *reportWindows) click(window int, id string) components.Click {
	r.mu.Lock()
	defer r.mu.Unlock()
	if all := r.clicks[window]; all != nil {
		return all.Click(id)
	}
	return nil
}

// open is id's report window, if it has one open.
func (r *reportWindows) open(id string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	window, ok := r.byID[id]
	return window, ok
}

// closed forgets a report window that's been closed, so its id opens a new one.
func (r *reportWindows) closed(window int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, w := range r.byID {
		if w == window {
			delete(r.byID, id)
		}
	}
	delete(r.clicks, window)
}

// navigate says whether a window may go to target. Every window only ever shows the
// page it was given. A link clicked in a report opens in the browser instead, if it's
// http(s); anything else is blocked.
func (b *Bar) navigate(window int, target string, clicked bool) bool {
	if target == "about:blank" {
		return true
	}
	if window == BarWindow || window == MenuWindow || !clicked {
		return false
	}
	if u, err := url.Parse(target); err == nil {
		if scheme := strings.ToLower(u.Scheme); scheme == "http" || scheme == "https" {
			go openURL(target)
		}
	}
	return false
}

// openURL opens a link clicked in a report. The tests have it record the link
// instead of opening the browser.
var openURL = shim.OpenURL
