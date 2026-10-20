package bar

import (
	"log"

	"github.com/TheWozard/ledge/internal/store"
)

// showReport shows the event's report in the report window. The report is the
// event's own blocks, checked by the event schema when it was sent.
func (b *Bar) showReport(event store.Event) {
	// An event saved before reports were blocks has a file path instead; it's skipped.
	report, ok := event["report"].([]any)
	if !ok {
		log.Printf("ledge: the report of %s isn't a list of blocks; send the event again to show it", event["id"])
		return
	}
	if err := b.ui.ShowReport(text(event, "id"), text(event, "title"), blocks(report)); err != nil {
		log.Printf("ledge: showing the report of %s: %v", event["id"], err)
	}
}
