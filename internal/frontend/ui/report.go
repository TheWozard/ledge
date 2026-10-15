package ui

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// ShowReport shows title and blocks in id's report window, in the bar's theme, and
// brings it forward. Each id has its own window: showing an id again replaces what
// its window shows, and one that's been closed opens anew. id is the app's, such as
// the card or pill the report is for. A block is any component; components has the
// usual ones.
func (b *Bar) ShowReport(id, title string, blocks []templ.Component) error {
	b.mu.Lock()
	name := b.options.Theme
	b.mu.Unlock()
	css, err := theme.CSS(name, theme.Report)
	if err != nil {
		return err
	}
	var page bytes.Buffer
	if err := components.ReportPage(css, title, blocks).Render(context.Background(), &page); err != nil {
		return err
	}
	window := b.reportWindow(id)
	frame := theme.Frames[name]
	shim.CreateWindow(window, title, 900, 700)
	shim.SetTitle(window, title)
	shim.SetAppearance(window, frame.Appearance)
	shim.SetFrameless(window, frame.Frameless)
	shim.LoadHTML(window, page.String())
	shim.ShowWindow(window)
	return nil
}

// reportWindow is id's report window, given a new one if it hasn't got one open.
func (b *Bar) reportWindow(id string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	window, ok := b.reports[id]
	if !ok {
		b.lastWindow++
		window = b.lastWindow
		b.reports[id] = window
	}
	return window
}

// closed forgets a report window that's been closed, so its id opens a new one.
func (b *Bar) closed(window int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, w := range b.reports {
		if w == window {
			delete(b.reports, id)
		}
	}
}

// navigate says whether a window may go to target. Every window only ever shows the
// page it was given. A link clicked in a report goes to Handlers.OpenURL instead, if
// it's http(s); anything else is blocked.
func (b *Bar) navigate(window int, target string, clicked bool) bool {
	if target == "about:blank" {
		return true
	}
	if window == BarWindow || !clicked || b.handlers.OpenURL == nil {
		return false
	}
	if u, err := url.Parse(target); err == nil {
		if scheme := strings.ToLower(u.Scheme); scheme == "http" || scheme == "https" {
			go b.handlers.OpenURL(target)
		}
	}
	return false
}
