package bar

import (
	"log"

	"github.com/TheWozard/ledge/internal/store"
)

// showReport shows the event's report in the report window.
func (b *Bar) showReport(event store.Event) {
	if err := b.ui.ShowReport(event.ID, event.Title, (&report{bar: b, event: event}).blocks(event.Report)); err != nil {
		log.Printf("showing the report of %s: %v", event.ID, err)
	}
}

// choose keeps value as chosen in the event's select or checkbox, and shows its
// report again, with it, or, if it couldn't be kept, as it was.
func (b *Bar) choose(id, inputID string, value any) {
	event, err := b.store.Choose(id, inputID, value)
	if err != nil {
		log.Printf("choosing %#v in %s's %q: %v", value, id, inputID, err)
		var ok bool
		if event, ok = b.store.Get(id); !ok {
			return
		}
	}
	b.showReport(event)
}
