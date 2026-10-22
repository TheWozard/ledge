package components

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"
)

// Click is what clicking something on the bar, or in a report, does, given how it was
// clicked. The bar calls it on a goroutine of its own. A nil Click isn't clickable.
//
// The page never holds a Click, only an id for it, made from the ids of the card or
// action it's on, or a report's button's; Clicks and ReportClicks map them back.
type Click func(ClickEvent)

// ClickEvent is how something was clicked: the modifier keys held, ⌥, ⌘, ⇧ and ⌃,
// and whether with the middle mouse button rather than the main one. An action chosen
// from a menu has the keys held as it was chosen, by a click or Return, and never the
// middle button. (A ⌃-click on a card is a right-click: it shows the card's menu.)
type ClickEvent struct {
	Alt, Cmd, Shift, Ctrl bool
	Middle                bool
}

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
		for _, p := range f.Parts {
			if p.Click != nil {
				clicks[f.PartClickID(p.ID)] = p.Click
			}
		}
	}
	return clicks
}

// clickID, dismissID and the action's clickID are the ids for their Clicks. A
// card's is its element id; the ×'s and an action's are its id with a prefix of
// their own, so none can collide. The page posts a card's and its ×'s; an action's is
// chosen from its card's menu, which the bar shows.
func (f Frame) clickID() string   { return f.ElementID() }
func (f Frame) dismissID() string { return "dismiss-" + f.ID }
func (q Action) clickID() string  { return "action-" + q.ID }

// PartClickID is the id for the Click of the card's part partID, for its body to put
// in data-click. It has its card's id's length in it, so no two cards' parts share
// one: card "a-b"'s part "c" is "part-3-a-b-c", and card "a"'s part "b-c" is
// "part-1-a-b-c".
func (f Frame) PartClickID(partID string) string {
	return fmt.Sprintf("part-%d-%s-%s", len(f.ID), f.ID, partID)
}

// ReportClicks is the Clicks in a report, put there by ReportClick as its blocks are
// rendered with WithReportClicks: each by the id the page has for it, and the ids of
// those in each Block, by its id, so a block's can be replaced with it.
type ReportClicks struct {
	Clicks map[string]Click
	Blocks map[string][]string
}

// reportClicksKey and blockKey are the context keys of the ReportClicks a report is
// rendered with, and of the Block what's in it is rendered in.
type (
	reportClicksKey struct{}
	blockKey        struct{}
)

// WithReportClicks is ctx with clicks, for ReportClick to put the Clicks of what's
// rendered with it in.
func WithReportClicks(ctx context.Context, clicks *ReportClicks) context.Context {
	return context.WithValue(ctx, reportClicksKey{}, clicks)
}

// ReportClick puts click in the ReportClicks ctx has, as id, unique in its report,
// and returns the id for the page to have for it, in data-click, such as on a
// Buttons' button. Rendered without any, as on the bar, the click goes nowhere.
func ReportClick(ctx context.Context, id string, click Click) string {
	clicks, ok := ctx.Value(reportClicksKey{}).(*ReportClicks)
	if !ok || click == nil {
		return id
	}
	if clicks.Clicks == nil {
		clicks.Clicks = map[string]Click{}
	}
	clicks.Clicks[id] = click
	if block, ok := ctx.Value(blockKey{}).(string); ok {
		if clicks.Blocks == nil {
			clicks.Blocks = map[string][]string{}
		}
		clicks.Blocks[block] = append(clicks.Blocks[block], id)
	}
	return id
}

// inBlock is block rendered in the Block with id, so the Clicks in it are its.
func inBlock(id string, block templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return block.Render(context.WithValue(ctx, blockKey{}, id), w)
	})
}
