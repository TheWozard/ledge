package bar

import (
	"log"
	"net/url"
	"strings"

	"ledge/internal/pages"
	"ledge/internal/shim"
	"ledge/internal/store"
)

// ReportWindow is the report window's id in the shim. There's one, reused for each
// report.
const ReportWindow = 2

// showReport shows the event's report in the report window, in the bar's theme. The
// report is the event's own blocks, checked by the event schema when it was sent.
func (b *Bar) showReport(event store.Event) {
	title, _ := event["title"].(string)
	// An event saved before reports were blocks has a file path instead; it's skipped.
	blocks, ok := event["report"].([]any)
	if !ok {
		log.Printf("ledge: the report of %s isn't a list of blocks; send the event again to show it", event["id"])
		return
	}
	page, err := pages.Report(b.config.Theme, title, blocks)
	if err != nil {
		log.Printf("ledge: showing the report of %s: %v", event["id"], err)
		return
	}
	shim.CreateWindow(ReportWindow, title, 900, 700)
	shim.SetTitle(ReportWindow, title)
	frame := pages.Frames[b.config.Theme]
	shim.SetAppearance(ReportWindow, frame.Appearance)
	shim.SetFrameless(ReportWindow, frame.Frameless)
	shim.LoadHTML(ReportWindow, page)
	shim.ShowWindow(ReportWindow)
}

// navigate says whether a window may go to target. Both windows only ever show the
// page ledge gave them. A link clicked in a report opens in the browser instead, if
// it's http(s); anything else is blocked.
func (b *Bar) navigate(window int, target string, clicked bool) bool {
	if target == "about:blank" {
		return true
	}
	if window != ReportWindow || !clicked {
		return false
	}
	if u, err := url.Parse(target); err == nil {
		if scheme := strings.ToLower(u.Scheme); scheme == "http" || scheme == "https" {
			go b.openURL(target)
		}
	}
	return false
}
