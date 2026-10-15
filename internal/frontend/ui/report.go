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

// ShowReport shows title and blocks in the report window, in the bar's theme, and
// brings it forward. There's one report window, reused for each report. A block is
// any component; components has the usual ones.
func (b *Bar) ShowReport(title string, blocks []templ.Component) error {
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
	frame := theme.Frames[name]
	shim.CreateWindow(ReportWindow, title, 900, 700)
	shim.SetTitle(ReportWindow, title)
	shim.SetAppearance(ReportWindow, frame.Appearance)
	shim.SetFrameless(ReportWindow, frame.Frameless)
	shim.LoadHTML(ReportWindow, page.String())
	shim.ShowWindow(ReportWindow)
	return nil
}

// navigate says whether a window may go to target. Both windows only ever show the
// page they were given. A link clicked in a report goes to Handlers.OpenURL instead,
// if it's http(s); anything else is blocked.
func (b *Bar) navigate(window int, target string, clicked bool) bool {
	if target == "about:blank" {
		return true
	}
	if window != ReportWindow || !clicked || b.handlers.OpenURL == nil {
		return false
	}
	if u, err := url.Parse(target); err == nil {
		if scheme := strings.ToLower(u.Scheme); scheme == "http" || scheme == "https" {
			go b.handlers.OpenURL(target)
		}
	}
	return false
}
