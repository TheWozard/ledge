// Package ui is the bar and its report window: native windows (through the shim)
// showing pages built from components, in a theme. An app opens a Bar with Options
// and Handlers, gives it what to show with Render and ShowReport, and is called back
// with the ids the pages act on. Each report has its own window.
//
// AppKit needs the main goroutine, so an app calls Init, then Open, then Run on the
// main goroutine, and does its own work on others. Stop makes Run return.
package ui

import (
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// BarWindow is the bar's id in the shim. Report windows take the ids after it, one
// per report, never reused.
const BarWindow = 1

// Options is where the bar goes and how it looks.
type Options struct {
	Screen int    // an index into the screens; 0 is the one with the menu bar
	Edge   string // "top" or "bottom"
	Height int    // in points
	Level  string // which windows it stays above: "top", "floating" or "normal"
	// Theme is how the bar and its reports look: theme.Builtin's or theme.Load's, or
	// one made up. The zero Theme is the built-in default.
	Theme theme.Theme
}

// Handlers is what the bar calls back with. Each is called on its own goroutine, and
// may be nil. The pages only ever pass an id, never a url, path or command: the app
// looks up what it holds for that id.
type Handlers struct {
	Dismiss func(id string) // a card's × was clicked
	Open    func(id string) // a card that opens, or a pill's link, was clicked
	Run     func(id string) // a pill with a command was clicked
	// OpenURL is an http(s) link clicked in a report. The report window never goes
	// there itself.
	OpenURL func(url string)
}

// Init sets up the app. Call it once, on the main goroutine, before Open.
func Init() { shim.Init() }

// Run runs the main loop until Stop. Main goroutine only.
func Run() { shim.Run() }

// Stop makes Run return. Safe from any goroutine.
func Stop() { shim.Stop() }
