package components

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
