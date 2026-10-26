package bar

import (
	"log"

	"github.com/TheWozard/ledge/internal/store"
)

// showReport shows the event's report in the report window.
func (b *Bar) showReport(event store.Event) {
	if err := b.ui.ShowReport(event.ID, event.Title, blocks(event.Report)); err != nil {
		log.Printf("showing the report of %s: %v", event.ID, err)
	}
}
