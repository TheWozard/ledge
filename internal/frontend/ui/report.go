package ui

import (
	"bytes"
	"context"
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
// has the usual ones.
func (b *Bar) ShowReport(id, title string, blocks []templ.Component) error {
	b.mu.Lock()
	t := b.options.Theme
	b.mu.Unlock()
	var page bytes.Buffer
	if err := components.ReportPage(t.Report, title, blocks).Render(context.Background(), &page); err != nil {
		return err
	}
	window := b.reports.window(id)
	shim.CreateWindow(window, title, 900, 700)
	shim.SetTitle(window, title)
	shim.SetAppearance(window, t.Frame.Appearance)
	shim.SetFrameless(window, t.Frame.Frameless)
	shim.LoadHTML(window, page.String())
	shim.ShowWindow(window)
	return nil
}

// reportWindows is the window of each report that's open, by the id it was shown
// with. Each report has its own window, its id never reused.
type reportWindows struct {
	mu   sync.Mutex
	byID map[string]int
	last int // the last window id given out
}

// newReportWindows is no report windows yet; the first will take the id after the
// bar's.
func newReportWindows() *reportWindows {
	return &reportWindows{byID: map[string]int{}, last: BarWindow}
}

// window is id's report window, given a new one if it hasn't got one open.
func (r *reportWindows) window(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	window, ok := r.byID[id]
	if !ok {
		r.last++
		window = r.last
		r.byID[id] = window
	}
	return window
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
