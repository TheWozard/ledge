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
// A page posts it as JSON, with the names it has in its tags.
type ClickEvent struct {
	Alt    bool `json:"alt"`
	Cmd    bool `json:"cmd"`
	Shift  bool `json:"shift"`
	Ctrl   bool `json:"ctrl"`
	Middle bool `json:"middle"`
}

// Clicks is the Clicks on a page, put there by ClickID as it's rendered with
// WithClicks, each by the id the page has for it, and on the bar the Clicks of the
// cards' actions, by their menu items' ids. The page can click only the first: an
// action is only ever chosen from its card's menu. The zero Clicks is empty and ready
// to use.
type Clicks struct {
	byID    map[string]Click
	actions map[string]Click
}

// scope is what the Clicks rendered in it belong to: a card's frame ("card") or its
// body ("body"), or a report's Block ("block"), with the id of the card or block. The
// zero scope is what's in none, such as a report's blocks without an id.
type scope struct{ kind, id string }

// Click is the Click the page has with id, or nil.
func (c *Clicks) Click(id string) Click { return c.byID[id] }

// Action is the Click of the action whose menu item has id, or nil.
func (c *Clicks) Action(id string) Click { return c.actions[id] }

// IDs is the ids of the Clicks the page has, sorted.
func (c *Clicks) IDs() []string { return slices.Sorted(maps.Keys(c.byID)) }

// add puts click in c as id.
func (c *Clicks) add(id string, click Click) {
	if c.byID == nil {
		c.byID = map[string]Click{}
	}
	c.byID[id] = click
}

// addActions puts the Clicks of the actions of the card with frame f in c, by their
// menu items' ids.
func (c *Clicks) addActions(f Frame) {
	for i, item := range f.Menu() {
		if click := f.Actions[i].Click; click != nil {
			if c.actions == nil {
				c.actions = map[string]Click{}
			}
			c.actions[item.ID] = click
		}
	}
}

// clicksKey and scopeKey are the context keys of the Clicks a page is rendered with,
// and of the scope what's in it is rendered in.
type (
	clicksKey struct{}
	scopeKey  struct{}
)

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
		clicks.add(full, click)
	}
	return full
}

// Clickable is the attributes that make an element clicked on its own, for a kind of
// card's body, or a report's block, to spread on it: data-click, with the id ClickID
// gives click as id, or none if click is nil, so it isn't clickable.
func Clickable(ctx context.Context, id string, click Click) templ.Attributes {
	if click == nil {
		return templ.Attributes{}
	}
	return templ.Attributes{"data-click": ClickID(ctx, id, click)}
}

// pageID is the page's id for id in s: kind/scope/id, each escaped so none has a /
// in it, or only id, escaped, for what's in no scope. No two are the same.
func pageID(s scope, id string) string {
	if s == (scope{}) {
		return url.PathEscape(id)
	}
	return url.PathEscape(s.kind) + "/" + url.PathEscape(s.id) + "/" + url.PathEscape(id)
}

// inScope is c rendered in s, so the ids of the Clicks in it are s's.
func inScope(s scope, c templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return c.Render(context.WithValue(ctx, scopeKey{}, s), w)
	})
}
