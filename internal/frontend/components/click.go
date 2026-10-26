package components

import (
	"context"
	"io"
	"maps"
	"net/url"
	"slices"

	"github.com/a-h/templ"
)

// Click is what clicking something on the bar, or in a report, does, given how it was
// clicked. The bar calls it on a goroutine of its own. A nil Click isn't clickable.
//
// The page never holds a Click, only an id for it, which ClickID gives out as the page
// is rendered; Clicks maps them back.
type Click func(ClickEvent)

// ClickEvent is how something was clicked: the modifier keys held, ⌥, ⌘, ⇧ and ⌃,
// and whether with the middle mouse button rather than the main one. An action chosen
// from a menu has the keys held as it was chosen, by a click or Return, and never the
// middle button. (A ⌃-click on a card is a right-click: it shows the card's menu.)
type ClickEvent struct {
	Alt, Cmd, Shift, Ctrl bool
	Middle                bool
}

// Clicks is the Clicks on a page, put there by ClickID as it's rendered with
// WithClicks: each by the id the page has for it, and the ids each scope put there,
// a card's or a report's Block's, so a scope's can be replaced with Replace. The zero
// Clicks is empty and ready to use.
type Clicks struct {
	byID    map[string]Click
	byScope map[scope][]string
}

// scope is what the Clicks rendered in it belong to: a card's frame ("card") or its
// body ("body"), or a report's Block ("block"), with the id of the card or block. The
// zero scope is what's in none, such as a report's blocks without an id.
type scope struct{ kind, id string }

// Click is the Click with id, or nil.
func (c *Clicks) Click(id string) Click { return c.byID[id] }

// IDs is the ids of the Clicks, sorted.
func (c *Clicks) IDs() []string { return slices.Sorted(maps.Keys(c.byID)) }

// Replace replaces the Clicks of each scope from was rendered in by from's.
func (c *Clicks) Replace(from *Clicks) {
	for s, ids := range from.byScope {
		for _, id := range c.byScope[s] {
			delete(c.byID, id)
		}
		for _, id := range ids {
			c.add(s, id, from.byID[id])
		}
	}
}

// enter notes that s has been rendered, even if nothing in it is clickable, so
// Replace takes away what it had.
func (c *Clicks) enter(s scope) {
	if c.byScope == nil {
		c.byScope = map[scope][]string{}
	}
	if _, ok := c.byScope[s]; !ok {
		c.byScope[s] = nil
	}
}

// add puts click in c as id, in scope s.
func (c *Clicks) add(s scope, id string, click Click) {
	if c.byID == nil {
		c.byID = map[string]Click{}
	}
	c.enter(s)
	if _, ok := c.byID[id]; !ok {
		c.byScope[s] = append(c.byScope[s], id)
	}
	c.byID[id] = click
}

// clicksKey, scopeKey and menusKey are the context keys of the Clicks a page is
// rendered with, of the scope what's in it is rendered in, and of what opens a
// card's menu.
type (
	clicksKey struct{}
	scopeKey  struct{}
	menusKey  struct{}
)

// WithMenus is ctx with menu, which gives the Click that shows the menu of the card
// with frame f, for the bar to render its ⋯ and a right-click on it with. Rendered
// without it, a card's menu goes nowhere.
func WithMenus(ctx context.Context, menu func(f Frame) Click) context.Context {
	return context.WithValue(ctx, menusKey{}, menu)
}

// menuID is the id of the Click that shows the menu of the card with frame f, put in
// the Clicks ctx has with the Click WithMenus gives for it.
func menuID(ctx context.Context, f Frame) string {
	var click Click
	if menu, ok := ctx.Value(menusKey{}).(func(Frame) Click); ok {
		click = menu(f)
	}
	return ClickID(ctx, "menu", click)
}

// WithClicks is ctx with clicks, for ClickID to put the Clicks of what's rendered with
// it in.
func WithClicks(ctx context.Context, clicks *Clicks) context.Context {
	return context.WithValue(ctx, clicksKey{}, clicks)
}

// ClickID puts click in the Clicks ctx has, as id, unique in what it's rendered in: a
// card's body, a report's Block, or the rest of a report. It returns the id for the
// page to have for it, in data-click, such as on a Buttons' button or a check: id
// with the scope it's in, so two cards, or two blocks, can use the same ids.
// Rendered without any Clicks, the click goes nowhere.
func ClickID(ctx context.Context, id string, click Click) string {
	s, _ := ctx.Value(scopeKey{}).(scope)
	full := pageID(s, id)
	if clicks, ok := ctx.Value(clicksKey{}).(*Clicks); ok && click != nil {
		clicks.add(s, full, click)
	}
	return full
}

// pageID is the page's id for id in s: kind/scope/id, each escaped so none has a /
// in it, or only id, escaped, for what's in no scope. No two are the same.
func pageID(s scope, id string) string {
	if s == (scope{}) {
		return url.PathEscape(id)
	}
	return url.PathEscape(s.kind) + "/" + url.PathEscape(s.id) + "/" + url.PathEscape(id)
}

// inScope is c rendered in s, so the Clicks in it are s's.
func inScope(s scope, c templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if clicks, ok := ctx.Value(clicksKey{}).(*Clicks); ok {
			clicks.enter(s)
		}
		return c.Render(context.WithValue(ctx, scopeKey{}, s), w)
	})
}
