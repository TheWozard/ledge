// Package components is the bar's and the report window's HTML, as templ components:
// the pages (BarPage, ReportPage), what goes in them (BarContent, and the report
// blocks: Heading, Paragraph, Fields, Table, List, Code, Note, Link), and the data
// they show (Card, Pill).
//
// Every string given to a component is shown as text: templ escapes it, so event and
// report text can't add markup. Colours are a theme's named colours (green, yellow,
// red, blue, gray) or #rrggbb; anything else shows as gray.
package components

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Card is an event on the bar.
type Card struct {
	ID          string
	Title       string
	Description string
	Source      string
	Received    time.Time
	Tags        []string
	Priority    string // "high", "medium", "low", or "" for none
	Opens       bool   // clicking the card opens something: it posts "open" with its ID
	Hint        string // shown when hovering over the card, such as the url it opens
}

// Pill is a status on the bar: a label in a colour that can change on a schedule.
type Pill struct {
	ID       string
	Title    string
	Color    string // until the first step of Schedule has passed
	Schedule []Step // in time order
	Detail   string // shown when hovering over the pill
	Command  string // if set, clicking the pill posts "run" with its ID; shown on hover
	Link     bool   // clicking the label posts "open" with its ID; ignored with a Command
}

// Step is a colour change on a pill's schedule.
type Step struct {
	At    time.Time
	Color string
}

// ColorAt is the pill's colour at now: that of its last step that has passed, or its
// own before any has.
func (p Pill) ColorAt(now time.Time) string {
	color := p.Color
	for _, step := range p.Schedule {
		if !step.At.After(now) {
			color = step.Color
		}
	}
	return color
}

// NextChange is the time of the pill's next colour change after now, if it has one.
func (p Pill) NextChange(now time.Time) (time.Time, bool) {
	for _, step := range p.Schedule {
		if step.At.After(now) {
			return step.At, true
		}
	}
	return time.Time{}, false
}

// hint is the pill's tooltip: its detail, the command a click runs, and the colour
// changes still to come.
func (p Pill) hint(now time.Time) string {
	var lines []string
	if p.Detail != "" {
		lines = append(lines, p.Detail)
	}
	if p.Command != "" {
		lines = append(lines, "Click to run: "+p.Command)
	}
	for _, step := range p.Schedule {
		if step.At.After(now) {
			lines = append(lines, step.Color+" at "+step.At.Local().Format("3:04 PM"))
		}
	}
	return strings.Join(lines, "\n")
}

var named = []string{"green", "yellow", "red", "blue", "gray"}

var hex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// cssColor is a colour as CSS: a named one is the theme's variable for it.
func cssColor(color string) string {
	switch {
	case slices.Contains(named, color):
		return "var(--c-" + color + ")"
	case hex.MatchString(color):
		return color
	}
	return "var(--c-gray)"
}

// ago is how long before now received was, roughly: "now", "5m", "3h", "2d".
func ago(received, now time.Time) string {
	minutes := int(math.Round(now.Sub(received).Minutes()))
	switch {
	case minutes < 1:
		return "now"
	case minutes < 60:
		return fmt.Sprintf("%dm", minutes)
	case minutes < 60*24:
		return fmt.Sprintf("%dh", int(math.Round(float64(minutes)/60)))
	}
	return fmt.Sprintf("%dd", int(math.Round(float64(minutes)/60/24)))
}
