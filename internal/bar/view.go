package bar

import (
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/a-h/templ"

	"github.com/TheWozard/ledge/internal/store"
)

// colors is the hex for each colour the schemas name, as the bar's cards take only
// #rrggbb.
var colors = map[string]string{
	"green": "#34c759", "yellow": "#ffcc00", "red": "#ff3b30", "blue": "#007aff", "gray": "#8e8e93",
}

// priorities is the colour of each priority's cards.
var priorities = map[string]string{"high": colors["red"], "medium": colors["yellow"], "low": colors["blue"]}

// hex is a colour from the schemas as #rrggbb: a named one's, or itself.
func hex(color string) string {
	if h, ok := colors[color]; ok {
		return h
	}
	return color
}

// view is events as the bar shows them, as of now, in the store's order. The events
// have been checked against the schema.
func (b *Bar) view(events []store.Event, now time.Time) []components.Card {
	cards := make([]components.Card, len(events))
	for i, event := range events {
		cards[i] = b.card(event, now)
	}
	return cards
}

// card is the event as a card of its kind, as of now. One with a schedule is drawn
// again at its next step, for its colour.
func (b *Bar) card(event store.Event, now time.Time) components.Card {
	id := event.ID
	basic := components.Basic{
		ID:          id,
		Title:       event.Title,
		Description: event.Description,
		Dismiss:     func(components.ClickEvent) { b.dismiss(id) },
	}
	// A command sent while commands were allowed stays on the card, but doesn't run.
	if len(event.Report) > 0 || event.URL != "" || event.Command != "" && b.commands {
		basic.Click = func(components.ClickEvent) { b.open(id) }
	}
	basic.Color = priorities[event.Priority]
	if event.Color != "" {
		basic.Color = hex(event.Color)
	}
	var next time.Time
	for _, step := range event.Schedule {
		if step.At.After(now) {
			next = step.At
			break
		}
		basic.Color = hex(step.Color)
	}

	var card components.Card = basic
	switch event.Kind {
	case "timer":
		card = components.Timer{Basic: basic, Start: event.Start, End: event.End}
	case "progress":
		card = components.Progress{Basic: basic, Value: event.Value.Float(), Label: event.Label}
	case "stat":
		card = components.Stat{Basic: basic, Value: event.Value.Text, Change: event.Change,
			ChangeColor: event.ChangeColor, History: event.History}
	case "checks":
		checks := components.Checks{Basic: basic}
		for _, check := range event.Checks {
			checks.Checks = append(checks.Checks, components.Check{Name: check.Name, State: check.State})
		}
		card = checks
	case "message":
		card = components.Message{Basic: basic, From: event.From, Avatar: event.Avatar, Sent: event.Sent}
	}
	if next.IsZero() {
		return card
	}
	return recolored{card, next, b.Render}
}

// recolored is a card that's drawn again at next, when its colour changes.
type recolored struct {
	components.Card
	next   time.Time
	redraw func()
}

func (r recolored) Frame() components.Frame {
	f := r.Card.Frame()
	f.Ends, f.Complete = r.next, r.redraw
	return f
}

// blocks is a report's blocks as components.
func blocks(report []store.Block) []templ.Component {
	var out []templ.Component
	for _, block := range report {
		switch block.Type {
		case "heading":
			out = append(out, components.Heading(2, components.Text(block.Text)))
		case "text":
			out = append(out, components.Paragraph(components.Text(block.Text)))
		case "fields":
			fields := make([]components.Field, len(block.Fields))
			for i, field := range block.Fields {
				fields[i] = components.Field{Label: field.Label, Value: components.Text(field.Value.Text)}
			}
			out = append(out, components.Fields(fields))
		case "table":
			rows := make([][]components.Inline, len(block.Rows))
			for i, row := range block.Rows {
				for _, cell := range row {
					rows[i] = append(rows[i], components.Text(cell.Text))
				}
			}
			out = append(out, components.Table(block.Columns, rows))
		case "list":
			items := make([]components.Item, len(block.Items))
			for i, item := range block.Items {
				items[i] = components.Item{Text: components.Text(item)}
			}
			out = append(out, components.List(block.Ordered, items...))
		case "code":
			out = append(out, components.Code(block.Text))
		case "note":
			out = append(out, components.Note(block.Color, components.Text(block.Text)))
		case "link":
			out = append(out, components.Paragraph(components.LinkTo(block.URL, components.Text(block.Text))))
		}
	}
	return out
}
