package bar

import (
	"fmt"
	"log"
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

// report is an event's report as components, with what its buttons do.
type report struct {
	bar   *Bar
	event store.Event
	made  int // how many buttons it has made, for each to have its own id
}

// blocks is the report's blocks as components.
func (r *report) blocks(blocks []store.Block) []templ.Component {
	var out []templ.Component
	for _, block := range blocks {
		switch block.Type {
		case "heading":
			level := block.Level
			if level == 0 {
				level = 2
			}
			out = append(out, components.Heading(level, r.inline(block.Text)))
		case "text":
			out = append(out, components.Paragraph(r.inline(block.Text)))
		case "fields":
			fields := make([]components.Field, len(block.Fields))
			for i, field := range block.Fields {
				fields[i] = components.Field{Label: field.Label, Value: r.inline(field.Value)}
			}
			out = append(out, components.Fields(fields))
		case "table":
			rows := make([][]components.Inline, len(block.Rows))
			for i, row := range block.Rows {
				for _, cell := range row {
					rows[i] = append(rows[i], r.inline(cell))
				}
			}
			out = append(out, components.Table(block.Columns, rows))
		case "list":
			out = append(out, components.List(block.Ordered, r.items(block.Items)...))
		case "code":
			out = append(out, components.Code(block.Text.Text))
		case "note":
			out = append(out, components.Note(block.Color, r.inline(block.Text)))
		case "quote":
			out = append(out, components.Quote(r.inline(block.Text)))
		case "link":
			out = append(out, components.Paragraph(components.LinkTo(block.URL, r.inline(block.Text))))
		case "buttons":
			out = append(out, r.buttons(block.Buttons))
		case "image":
			out = append(out, components.Image(block.Src, block.Alt))
		case "details":
			out = append(out, components.Details(r.inline(block.Summary), r.blocks(block.Blocks)...))
		case "rule":
			out = append(out, components.Rule())
		}
	}
	return out
}

// inline is text as a component: a string or number as it was sent, spans as their
// styles, and an array run together.
func (r *report) inline(in store.Inline) components.Inline {
	switch in.Type {
	case "strong":
		return components.Strong(r.inline(*in.Content))
	case "em":
		return components.Em(r.inline(*in.Content))
	case "mono":
		return components.Mono(in.Content.Text)
	case "link":
		return components.LinkTo(in.URL, r.inline(*in.Content))
	case "buttons":
		return r.buttons(in.Buttons)
	}
	if in.Parts != nil {
		parts := make([]templ.Component, len(in.Parts))
		for i, part := range in.Parts {
			parts[i] = r.inline(part)
		}
		return templ.Join(parts...)
	}
	return components.Text(in.Text)
}

// items is a list's items, with the lists under them.
func (r *report) items(items []store.Item) []components.Item {
	out := make([]components.Item, len(items))
	for i, item := range items {
		out[i] = components.Item{Text: r.inline(item.Text), Ordered: item.Ordered, Items: r.items(item.Items)}
	}
	return out
}

// buttons is a row of buttons, each with its own id in the report. One that would run
// a command while commands aren't allowed does nothing, so it's disabled.
func (r *report) buttons(buttons []store.Button) templ.Component {
	out := make([]components.Button, len(buttons))
	for i, button := range buttons {
		r.made++
		out[i] = components.Button{ID: fmt.Sprintf("button-%d", r.made), Title: button.Title, Color: button.Color}
		b, event := r.bar, r.event
		switch {
		case button.URL != "":
			out[i].Click = func(components.ClickEvent) { b.openURL(button.URL) }
		case button.Command != "" && b.commands:
			event.Command = button.Command
			out[i].Click = func(components.ClickEvent) {
				if err := b.runCommand(event); err != nil {
					log.Printf("running a command of %s: %v", event.ID, err)
				}
			}
		case button.Dismiss:
			out[i].Click = func(components.ClickEvent) { b.dismiss(event.ID) }
		}
	}
	return components.Buttons(out)
}
