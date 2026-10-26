package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
)

// ShowReport shows title and blocks in id's report window, in the bar's theme, and
// brings it forward. If they can't be rendered, it says why, and nothing changes. Each id has its own window: showing an id again replaces what
// its window shows, keeping its scroll, and one that's been closed opens anew. id is
// the app's, such as the card or action the report is for. A block is any component;
// components has the usual ones. Clicking what's in them, such as a Buttons' button,
// calls its Click, as one on the bar does, until the report is shown again or closed.
// The blocks are rendered with the bar's lock held, as cards are, so they mustn't
// call back into the bar.
func (b *Bar) ShowReport(id, title string, blocks []templ.Component) error {
	blocks = slices.Clone(blocks)
	b.mu.Lock()
	t := b.options.Theme
	html, clicks, err := renderReport(blocks)
	if err != nil {
		b.mu.Unlock()
		return err
	}
	p := b.report(id)
	b.mu.Unlock()
	// These wait for the main thread, which can be waiting for the lock to say a
	// window closed, so they're called without it.
	b.native.CreateWindow(p.window, title, 900, 700)
	b.native.SetTitle(p.window, title)
	b.native.SetAppearance(p.window, t.Frame.Appearance)
	b.native.SetFrameless(p.window, t.Frame.Frameless)
	b.mu.Lock()
	if !p.hasPage || p.css != t.Report {
		if err := b.loadReport(p, t.Report); err != nil {
			b.mu.Unlock()
			return err
		}
	}
	p.title, p.blocks = title, blocks
	p.show(title, html, clicks)
	b.mu.Unlock()
	b.native.ShowWindow(p.window)
	return nil
}

// ShowReportBlock replaces the components.Block with block's ID in id's report by
// block, leaving the rest of the report, its scroll and where its window is as they
// are, as a ShowReport of it all with only that block changed would, but for bringing
// the window forward. It reports whether id's report is open with such a block; if
// not, or if block can't be rendered, nothing changes. Called just after ShowReport,
// the block is replaced once the new page has loaded.
func (b *Bar) ShowReportBlock(id string, block components.Block) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.reports[id]
	if !ok {
		return false, nil
	}
	blocks, ok := components.ReplaceBlock(p.blocks, block)
	if !ok {
		return false, nil
	}
	html, clicks, err := renderReport(blocks)
	if err != nil {
		return true, err
	}
	p.blocks = blocks
	p.show(p.title, html, clicks)
	return true, nil
}

// renderReport renders blocks as a report shows them, and the Clicks in them.
func renderReport(blocks []templ.Component) (string, *components.Clicks, error) {
	var html strings.Builder
	clicks := &components.Clicks{}
	err := components.ReportContent(blocks).Render(components.WithClicks(context.Background(), clicks), &html)
	return html.String(), clicks, err
}

// loadReport loads p, a report's page, afresh, in css, the theme's stylesheet for it.
// Call it with the bar's lock held.
func (b *Bar) loadReport(p *page, css string) error {
	var html bytes.Buffer
	if err := components.ReportPage(css).Render(context.Background(), &html); err != nil {
		return err
	}
	p.css = css
	p.load(html.String())
	return nil
}

// show has p, a report's page, show title and html, rendered from ReportContent with
// clicks, the Clicks in it, from then; until it's ready, it waits. Call it with the
// bar's lock held.
func (p *page) show(title, html string, clicks *components.Clicks) {
	p.clicks = clicks
	// Marshalled, the title and html are JS strings; <, > and & are escaped in them.
	titleJS, _ := json.Marshal(title)
	htmlJS, _ := json.Marshal(html)
	// The busy marks go with it, in one go, as on the bar.
	p.eval("update(" + string(titleJS) + ", " + string(htmlJS) + "); " + p.busyJS())
}

// report is id's report's page, given a new window if it hasn't one open. Each
// report has its own window, its id never reused. Call it with the bar's lock held.
func (b *Bar) report(id string) *page {
	if p, ok := b.reports[id]; ok {
		return p
	}
	b.last++
	p := newPage(b.native, b.last, fmt.Sprintf("report window %d", b.last), pageMessages, b.reportMessage)
	p.report = id
	b.reports[id], b.pages[p.window] = p, p
	return p
}

// closed forgets a report window that's been closed, so its id opens a new one. The
// shim calls it on the main thread.
func (b *Bar) closed(window int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.pages[window]
	if p == nil || b.reports[p.report] != p {
		return // not a report's
	}
	delete(b.pages, window)
	delete(b.reports, p.report)
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
