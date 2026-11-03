package ui

import (
	"time"

	"github.com/TheWozard/ledge-frontend/components"
)

// endings is the cards on the bar that end: a timer for each, to complete it when it's
// due, and the end each has completed at, so it completes only once for that end.
// The zero endings is ready to use. Its methods are called with the bar's lock held.
type endings struct {
	timers    map[string]*time.Timer // by card id
	completed map[string]time.Time   // by card id
}

// schedule sets a timer for each of cards that ends and hasn't completed at that end,
// to call complete with its id and end when it's due, stopping those set before. A
// card that's gone is forgotten.
func (e *endings) schedule(cards []components.Card, complete func(id string, ends time.Time)) {
	for _, timer := range e.timers {
		timer.Stop()
	}
	timers, completed := map[string]*time.Timer{}, map[string]time.Time{}
	for _, card := range cards {
		frame := card.Frame()
		if frame.Complete == nil || frame.Ends.IsZero() {
			continue
		}
		if at, ok := e.completed[frame.ID]; ok && at.Equal(frame.Ends) {
			completed[frame.ID] = at
			continue
		}
		id, ends := frame.ID, frame.Ends
		timers[id] = time.AfterFunc(time.Until(ends), func() { complete(id, ends) })
	}
	e.timers, e.completed = timers, completed
}

// finish marks card id completed at ends, and says whether it hadn't been already.
func (e *endings) finish(id string, ends time.Time) bool {
	if _, done := e.completed[id]; done {
		return false
	}
	e.completed[id] = ends
	return true
}

// complete calls the Complete of card id, if it's still on the bar ending at ends and
// hasn't already.
func (b *Bar) complete(id string, ends time.Time) {
	b.mu.Lock()
	var complete func()
	for _, card := range b.cards {
		if frame := card.Frame(); frame.ID == id && frame.Ends.Equal(ends) && b.endings.finish(id, ends) {
			complete = frame.Complete
		}
	}
	b.mu.Unlock()
	if complete != nil {
		complete()
	}
}
