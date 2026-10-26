package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
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
	var html bytes.Buffer
	clicks := &components.Clicks{}
	ctx := components.WithClicks(context.Background(), clicks)
	if err := components.ReportPage(t.Report, title, blocks).Render(ctx, &html); err != nil {
		return err
	}
	b.mu.Lock()
	p := b.report(id)
	b.mu.Unlock()
	// These wait for the main thread, which can be waiting for the lock to say a
	// window closed, so they're called without it.
	b.native.CreateWindow(p.window, title, 900, 700)
	b.native.SetTitle(p.window, title)
	b.native.SetAppearance(p.window, t.Frame.Appearance)
	b.native.SetFrameless(p.window, t.Frame.Frameless)
	// The page and its Clicks change together, so they always match.
	b.mu.Lock()
	p.load(html.String(), clicks)
	b.mu.Unlock()
	b.native.ShowWindow(p.window)
	return nil
}

// ShowReportBlock replaces the block given blockID with components.Block in id's
// report by block, leaving the rest of the report, its scroll and where its window
// is as they are. It reports whether id's report is open; if it is but has no such
// block, nothing changes. Called just after ShowReport, the block is replaced once
// the new page has loaded.
func (b *Bar) ShowReportBlock(id, blockID string, block templ.Component) (bool, error) {
	b.mu.Lock()
	_, ok := b.reports[id]
	b.mu.Unlock()
	if !ok {
		return false, nil
	}
	var html strings.Builder
	clicks := &components.Clicks{}
	ctx := components.WithClicks(context.Background(), clicks)
	if err := components.Block(blockID, block).Render(ctx, &html); err != nil {
		return true, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.reports[id]
	if !ok {
		return false, nil // closed meanwhile
	}
	p.clicks.Replace(clicks)
	// Marshalled, the id and html are JS strings; <, > and & are escaped in them.
	idJS, _ := json.Marshal(blockID)
	htmlJS, _ := json.Marshal(html.String())
	// The busy marks go with it, as the block's new elements haven't any.
	p.eval("replaceBlock(" + string(idJS) + ", " + string(htmlJS) + "); " + p.busyJS())
	return true, nil
}

// report is id's report's page, given a new window if it hasn't one open. Each
// report has its own window, its id never reused. Call it with the bar's lock held.
func (b *Bar) report(id string) *page {
	if p, ok := b.reports[id]; ok {
		return p
	}
	b.last++
	p := newPage(b.native, b.last, fmt.Sprintf("report window %d", b.last))
	b.reports[id], b.pages[p.window] = p, p
	return p
}

// closed forgets a report window that's been closed, so its id opens a new one. The
// shim calls it on the main thread.
func (b *Bar) closed(window int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.pages[window]
	if p == nil || p == b.bar {
		return
	}
	delete(b.pages, window)
	for id, q := range b.reports {
		if q == p {
			delete(b.reports, id)
		}
	}
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
			go b.native.OpenURL(target)
		}
	}
	return false
}
