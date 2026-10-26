// Package ui is the bar and its report window: native windows (through the shim)
// showing pages built from components, in a theme. An app opens a Bar with Options,
// gives it what to show with Render and ShowReport, and is called back through the
// Clicks on what it shows. Each report has its own window.
//
// AppKit needs the main goroutine, so an app calls Init, then Open, then Run on the
// main goroutine, and does its own work on others. Stop makes Run return.
//
// The bar renders what it's given with its lock held: a card's Frame, once, when it's
// given it, its Body each time it's drawn, and a report's blocks each time it's shown.
// None of them may call back into the bar (Render, ShowReport and the rest): that
// would deadlock. What the bar calls back, the Clicks and a card's Complete, it calls
// without its lock, so they may.
package ui

import (
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// BarWindow is the bar's id in the shim, and MenuWindow the pop-up a theme's menu is
// drawn in. Report windows take the ids after them, one per report, never reused.
const (
	BarWindow  = 1
	MenuWindow = 2
)

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

// Init sets up the app. Call it once, on the main goroutine, before Open.
func Init() { shim.Init() }

// Run runs the main loop until Stop. Main goroutine only.
func Run() { shim.Run() }

// Stop makes Run return. Safe from any goroutine.
func Stop() { shim.Stop() }
