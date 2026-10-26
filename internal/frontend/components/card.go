package components

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
)

// Card is something on the bar, of any kind. The bar draws every card's frame the
// same way, from its Frame: its colour, its ×, what clicking it does, the menu of
// its actions, and its arrival and leaving. What's inside
// the frame is the card's Body, which is up to its kind. Basic is the built-in kind.
//
// The bar calls Frame once, when it's given the card, and Body each time it draws it,
// both while it holds its lock, so neither may call back into the bar (Render,
// ShowReport): that would deadlock.
type Card interface {
	Frame() Frame
	// Body is what's in the card, above its actions. Like everything on the bar, it
	// should show strings as text, which templ does unless told otherwise. A click on
	// it is a click on the card, but on a part of it the body marks with
	// { Clickable(ctx, id, click)... }, it's a click on that part alone: id is the
	// part's, unique within its card.
	Body() templ.Component
}

// Frame is what the bar draws of every card, whatever its kind.
type Frame struct {
	ID      string // the app's, the same from one Render to the next, and unique across the bar
	Kind    string // a class on the card, for a theme to style its kind: lower case letters, digits and -
	Color   string // the card's own colour, #rrggbb, or "" for none
	Width   int    // the card's width in pixels, or 0 for its theme's for its kind
	Click   Click  // clicking the card, anywhere but its × and ⋯
	Dismiss Click  // clicking the card's ×, which shows only if this is set
	// Actions are the card's menu, in order: right-clicking the card, or clicking
	// its ⋯, which shows only if it has some, pops it up.
	Actions []Action
	// Ends is when the card's time is up, for one that ends, such as a Timer; then
	// the bar calls Complete, once, if it's set and the card is still on the bar.
	Ends     time.Time
	Complete func()
	// Progress is how far along the card is, from 0 to 1, such as a Progress's. It's
	// set on the card as --progress, for a theme to fill it; 0 is none.
	Progress float64
}

// Cards is cards of one kind as Cards, to Render.
func Cards[T Card](cards []T) []Card {
	all := make([]Card, len(cards))
	for i, c := range cards {
		all[i] = c
	}
	return all
}

// ElementID is the card's id on the page.
func (f Frame) ElementID() string { return "card-" + f.ID }

// className is what a Kind must be to be a class on its card.
var className = regexp.MustCompile(`^[a-z0-9-]+$`)

// class is the card's class: its kind, if that's a class name, or none.
func (f Frame) class() string {
	if className.MatchString(f.Kind) {
		return f.Kind
	}
	return ""
}

// color is the card's colour, if it has one that's #rrggbb.
func (f Frame) color() (string, bool) { return f.Color, hex.MatchString(f.Color) }

// style is the card's inline style: its colour, width and progress, if it has them.
func (f Frame) style() string {
	var style []string
	if color, ok := f.color(); ok {
		style = append(style, "--card-color: "+color)
	}
	if f.Width > 0 {
		style = append(style, fmt.Sprintf("--card-width: %dpx", f.Width))
	}
	if f.Progress > 0 {
		style = append(style, "--progress: "+strconv.FormatFloat(min(f.Progress, 1), 'f', -1, 64))
	}
	return strings.Join(style, "; ")
}

// Action is something to do about a card without opening it: an item in the card's
// menu, its title with a dot in its colour (a theme's named colour or #rrggbb;
// anything else is gray). Its ID is the app's, unique within its card.
type Action struct {
	ID    string
	Title string
	Color string
	// Click is choosing the action from the menu. Without one, it's greyed out: it
	// says something about the card, but there's nothing to do.
	Click Click
}

// MenuItem is an action as an item in its card's menu.
type MenuItem struct {
	Title   string
	Color   string // a named colour or #rrggbb, as colorName has it
	Enabled bool   // whether it can be chosen: it has a Click
	ID      string // its action's, with its card's, so it's unique on the bar
}

// Menu is the card's menu: an item for each of its actions, in order.
func (f Frame) Menu() []MenuItem {
	items := make([]MenuItem, len(f.Actions))
	for i, q := range f.Actions {
		items[i] = MenuItem{Title: q.Title, Color: colorName(q.Color), Enabled: q.Click != nil,
			ID: pageID(scope{"action", f.ID}, q.ID)}
	}
	return items
}

// card is a card's frame around its body: its buttons, ⋯ if it has actions and × if
// it can be dismissed, then the body. One with actions has data-menu, the id of the
// Click that shows its menu, from menu, for a right-click on it to post; its ⋯ has it
// as its data-click. The frame's Clicks are in the card's scope, and the body's in a
// scope of their own, so the ids of its parts can be anything. Its actions' Clicks
// are put in the Clicks too, for its menu to choose from.
func card(f Frame, body templ.Component, menu func(Frame) Click) templ.Component {
	var showMenu Click
	if menu != nil && len(f.Actions) > 0 {
		showMenu = menu(f)
	}
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if clicks, ok := ctx.Value(clicksKey{}).(*Clicks); ok {
			clicks.addActions(f)
		}
		return inScope(scope{"card", f.ID}, frame(f, body, showMenu)).Render(ctx, w)
	})
}
