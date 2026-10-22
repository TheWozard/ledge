package components

import "fmt"

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
