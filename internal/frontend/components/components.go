// Package components is the bar's and the report window's HTML, as templ components:
// the pages (BarPage, ReportPage), what goes in them (BarContent, and the report
// blocks: Heading, Paragraph, Fields, Table, List, Code, Note, Link), and the data
// they show (Card, Action).
//
// Every string given to a component is shown as text: templ escapes it, so event and
// report text can't add markup. A card's colour is #rrggbb; anything else is none.
// An action's or a note's is a theme's named colour (green, yellow, red, blue,
// gray) or #rrggbb; anything else shows as gray.
package components

import (
	"cmp"
	_ "embed"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
)

// idiomorph is Idiomorph 0.8.0 (github.com/bigskysoftware/idiomorph, 0BSD; see
// vendor/idiomorph.LICENSE), dist/idiomorph.min.js from npm. The bar's page inlines
// it, since it can't fetch files, to morph what it shows rather than replace it.
//
//go:embed vendor/idiomorph.min.js
var idiomorph string

// Card is something on the bar, of any kind. The bar draws every card's frame the
// same way, from its Frame: its place in the order, its colour, its ×, what clicking
// it does, its actions along its bottom, and its arrival and leaving. What's inside
// the frame is the card's Body, which is up to its kind. Basic is the built-in kind.
//
// The bar calls Frame and Body while it holds its lock, so neither may call back into
// the bar (Render, ShowReport): that would deadlock.
type Card interface {
	Frame() Frame
	// Body is what's in the card, above its actions. Like everything on the bar, it
	// should show strings as text, which templ does unless told otherwise. It can't
	// be clicked on its own: a click on it is a click on the card.
	Body() templ.Component
}

// Frame is what the bar draws of every card, whatever its kind.
type Frame struct {
	ID       string // the app's, the same from one Render to the next, and unique across the bar
	Kind     string // a class on the card, for a theme to style its kind: lower case letters, digits and -
	Priority int    // for the order: higher first; see SortByPriority
	Color    string // the card's own colour, #rrggbb, or "" for none
	Width    int    // the card's width in pixels, or 0 for its theme's for its kind
	Click    Click  // clicking the card, anywhere but × and a clickable action
	Dismiss  Click  // clicking the card's ×, which shows only if this is set
	// Actions are shown at the card's bottom, in order. Each is clicked on its
	// own, apart from the card.
	Actions []Action
	// Ends is when the card's time is up, for one that ends, such as a Timer; then
	// the bar calls Complete, once, if it's set and the card is still on the bar.
	Ends     time.Time
	Complete Click
}

// Cards is cards of one kind as Cards, to Render.
func Cards[T Card](cards []T) []Card {
	all := make([]Card, len(cards))
	for i, c := range cards {
		all[i] = c
	}
	return all
}

// Basic is the built-in kind of card: a title, and a description under it. Its kind
// is "basic".
type Basic struct {
	ID          string
	Title       string
	Description string
	Priority    int
	Color       string
	Width       int
	Click       Click
	Dismiss     Click
	Actions     []Action
}

// Frame is the card's frame, from its fields of the same names.
func (b Basic) Frame() Frame {
	return Frame{ID: b.ID, Kind: "basic", Priority: b.Priority, Color: b.Color, Width: b.Width, Click: b.Click, Dismiss: b.Dismiss,
		Actions: b.Actions}
}

// Body is the card's title and description.
func (b Basic) Body() templ.Component { return basicBody(b) }

// Timer is a kind of card that counts down from Start to End: a title, and the time
// left under it. Its kind is "timer". The page counts it down itself, and sets
// --progress on the card, from 0 at Start to 1 at End, for a theme to show how far
// along it is; once it's up, the card has data-done. Complete is called when End
// passes.
type Timer struct {
	ID       string
	Title    string
	Start    time.Time
	End      time.Time
	Priority int
	Color    string
	Click    Click
	Dismiss  Click
	Complete Click
}

// Frame is the timer's frame, from its fields of the same names; it ends at End.
func (t Timer) Frame() Frame {
	return Frame{ID: t.ID, Kind: "timer", Priority: t.Priority, Color: t.Color, Click: t.Click, Dismiss: t.Dismiss,
		Ends: t.End, Complete: t.Complete}
}

// Body is the timer's title and the time it has left, as of now; the page keeps that
// up to date.
func (t Timer) Body() templ.Component { return timerBody(t, timeLeft(time.Until(t.End))) }

// timeLeft is d as the page shows a timer's time left, rounded up to the second:
// "4:05", or "1:02:03" from an hour. Nothing is left once it's up.
func timeLeft(d time.Duration) string {
	seconds := int(math.Ceil(max(d, 0).Seconds()))
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// Action is something to do about a card without opening it: a label in a
// colour. Its ID is the app's, like a card's, and is unique across the bar.
type Action struct {
	ID    string
	Title string
	Color string
	// Click is clicking the action. Without one, clicking it is clicking its card.
	Click Click
}

// Click is what clicking something on the bar does. The bar calls it on a goroutine
// of its own. A nil Click isn't clickable.
//
// The page never holds a Click, only an id for it, made from the ids of the card or
// action it's on; Clicks maps them back.
type Click func()

// Clicks is every Click on cards, by the id the page has for it.
func Clicks(cards []Card) map[string]Click {
	clicks := map[string]Click{}
	for _, c := range cards {
		f := c.Frame()
		if f.Click != nil {
			clicks[f.clickID()] = f.Click
		}
		if f.Dismiss != nil {
			clicks[f.dismissID()] = f.Dismiss
		}
		for _, q := range f.Actions {
			if q.Click != nil {
				clicks[q.clickID()] = q.Click
			}
		}
	}
	return clicks
}

// clickID, dismissID and the action's clickID are the ids the page has for
// their Clicks. A card's and an action's are their element ids, which can't
// collide; the ×'s is its card's with a prefix neither has.
func (f Frame) clickID() string   { return f.ElementID() }
func (f Frame) dismissID() string { return "dismiss-" + f.ID }
func (q Action) clickID() string  { return q.ElementID() }

// ElementID is the card's id on the page, and its key in BarContent's fresh.
func (f Frame) ElementID() string { return "card-" + f.ID }

// SortByPriority puts cards in the bar's order: highest priority first, and those of
// the same priority in the order they're in.
func SortByPriority(cards []Card) {
	slices.SortStableFunc(cards, func(a, b Card) int { return cmp.Compare(b.Frame().Priority, a.Frame().Priority) })
}

var kind = regexp.MustCompile(`^[a-z0-9-]+$`)

// classes is the card's classes: its kind, if that's a class name, and clickable if
// it is.
func (f Frame) classes() string {
	var classes []string
	if kind.MatchString(f.Kind) {
		classes = append(classes, f.Kind)
	}
	if f.Click != nil {
		classes = append(classes, "clickable")
	}
	return strings.Join(classes, " ")
}

// color is the card's colour, if it has one that's #rrggbb.
func (f Frame) color() (string, bool) { return f.Color, hex.MatchString(f.Color) }

// style is the card's inline style: its colour and width, if it has them, and its
// place among the new ones if it's in fresh.
func (f Frame) style(fresh map[string]int) string {
	var style []string
	if color, ok := f.color(); ok {
		style = append(style, "--card-color: "+color)
	}
	if f.Width > 0 {
		style = append(style, fmt.Sprintf("--card-width: %dpx", f.Width))
	}
	if i, ok := fresh[f.ElementID()]; ok {
		style = append(style, entering(i))
	}
	return strings.Join(style, "; ")
}

// ElementID is the action's id on the page, and its key in BarContent's fresh.
func (q Action) ElementID() string { return "action-" + q.ID }

var named = []string{"green", "yellow", "red", "blue", "gray"}

var hex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// colorName is a colour as the page names it in data-color: a named one, #rrggbb, or
// gray for anything else, "" included.
func colorName(color string) string {
	if slices.Contains(named, color) || hex.MatchString(color) {
		return color
	}
	return "gray"
}

// cssColor is a colour as CSS: a named one is the theme's variable for it, and
// anything that isn't a colour is gray's.
func cssColor(color string) string {
	if color = colorName(color); hex.MatchString(color) {
		return color
	}
	return "var(--c-" + color + ")"
}
