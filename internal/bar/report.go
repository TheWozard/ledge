package bar

import (
	"fmt"
	"html"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ledge/internal/shim"
	"ledge/internal/store"
)

// ReportWindow is the report window's id in the shim. There's one, reused for each
// report.
const ReportWindow = 2

// showReport shows the event's report file in the report window. The file is read
// afresh each time, so a sender can rewrite it between clicks. The page can read files
// in its own directory; its links are checked by navigate.
func (b *Bar) showReport(event store.Event) {
	path := expandHome(event["report"].(string))
	title, _ := event["title"].(string)
	b.mu.Lock()
	b.reportDir = filepath.Dir(path)
	b.reportTitle = title
	b.mu.Unlock()

	shim.CreateWindow(ReportWindow, title, 900, 700)
	shim.SetTitle(ReportWindow, title)
	if err := readable(path); err != nil {
		log.Printf("ledge: showing the report of %s: %v", event["id"], err)
		shim.LoadHTML(ReportWindow, missingReport(path, err))
	} else {
		shim.LoadFile(ReportWindow, path, b.reportDir)
	}
	shim.ShowWindow(ReportWindow)
}

// navigate says whether a window may go to target. The bar only ever shows its own
// page. The report window may go to files in the report's directory; a clicked
// http(s) link opens in the browser instead, and anything else is blocked.
func (b *Bar) navigate(window int, target string, clicked bool) bool {
	if target == "about:blank" {
		return true
	}
	if window != ReportWindow {
		return false
	}
	u, err := url.Parse(target)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "file":
		b.mu.Lock()
		dir := b.reportDir
		b.mu.Unlock()
		return dir != "" && within(u.Path, dir)
	case "http", "https":
		if clicked {
			go b.openURL(target)
		}
	}
	return false
}

// titleChanged keeps the report window's title the page's own, or the event's title
// when the page has none.
func (b *Bar) titleChanged(window int, title string) {
	if window != ReportWindow {
		return
	}
	if title == "" {
		b.mu.Lock()
		title = b.reportTitle
		b.mu.Unlock()
	}
	shim.SetTitle(ReportWindow, title)
}

// within reports whether path is dir or inside it, comparing them both as given and
// with symlinks resolved (a temp dir under /var is really under /private/var).
func within(path, dir string) bool {
	inside := func(path, dir string) bool {
		path, dir = filepath.Clean(path), filepath.Clean(dir)
		return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
	}
	if inside(path, dir) {
		return true
	}
	realPath, err1 := filepath.EvalSymlinks(path)
	realDir, err2 := filepath.EvalSymlinks(dir)
	return err1 == nil && err2 == nil && inside(realPath, realDir)
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// readable checks that path is a file ledge can read.
func readable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s isn't a file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}

// missingReport is the page shown in place of a report that can't be read.
func missingReport(path string, err error) string {
	reason := err.Error()
	if os.IsNotExist(err) {
		reason = "There's no file there."
	}
	return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title></title><style>
body { font: 14px/1.5 -apple-system, sans-serif; margin: 24px; color: #1d1d1f; }
code { word-break: break-all; }
@media (prefers-color-scheme: dark) { body { background: #1d1d1f; color: #ececef; } }
</style></head><body><h1>Couldn't show this report</h1><p><code id="path">` + html.EscapeString(path) +
		`</code></p><p id="reason">` + html.EscapeString(reason) + `</p></body></html>`
}
