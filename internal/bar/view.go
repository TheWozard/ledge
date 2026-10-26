package bar

import (
	"encoding/json"
	"fmt"
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
	id := text(event, "id")
	basic := components.Basic{
		ID:          id,
		Title:       text(event, "title"),
		Description: text(event, "description"),
		Dismiss:     func(components.ClickEvent) { b.dismiss(id) },
	}
	if event["report"] != nil || text(event, "url") != "" || text(event, "command") != "" {
		basic.Click = func(components.ClickEvent) { b.open(id) }
	}
	basic.Color = priorities[text(event, "priority")]
	if color := text(event, "color"); color != "" {
		basic.Color = hex(color)
	}
	var next time.Time
	for _, step := range list(event["schedule"]) {
		step, _ := step.(map[string]any)
		// The store keeps the steps in time order.
		if at := when(step["at"]); at.After(now) {
			next = at
			break
		}
		basic.Color = hex(text(step, "color"))
	}

	var card components.Card = basic
	switch event["kind"] {
	case "timer":
		card = components.Timer{Basic: basic, Start: when(event["start"]), End: when(event["end"])}
	case "progress":
		card = components.Progress{Basic: basic, Value: number(event["value"]), Label: text(event, "label")}
	case "stat":
		stat := components.Stat{Basic: basic, Value: fmt.Sprint(event["value"]), Change: text(event, "change"),
			ChangeColor: text(event, "change_color")}
		for _, value := range list(event["history"]) {
			stat.History = append(stat.History, number(value))
		}
		card = stat
	case "checks":
		checks := components.Checks{Basic: basic}
		for _, check := range list(event["checks"]) {
			check, _ := check.(map[string]any)
			checks.Checks = append(checks.Checks, components.Check{Name: text(check, "name"), State: text(check, "state")})
		}
		card = checks
	case "message":
		card = components.Message{Basic: basic, From: text(event, "from"), Avatar: text(event, "avatar"), Sent: when(event["sent"])}
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

// blocks is a report's blocks, as event.schema.json has them, as components.
func blocks(report []any) []templ.Component {
	var out []templ.Component
	for _, block := range report {
		b, _ := block.(map[string]any)
		switch b["type"] {
		case "heading":
			out = append(out, components.Heading(text(b, "text")))
		case "text":
			out = append(out, components.Paragraph(text(b, "text")))
		case "fields":
			var fields []components.Field
			for _, item := range list(b["items"]) {
				item, _ := item.(map[string]any)
				fields = append(fields, components.Field{Label: text(item, "label"), Value: fmt.Sprint(item["value"])})
			}
			out = append(out, components.Fields(fields))
		case "table":
			var rows [][]string
			for _, row := range list(b["rows"]) {
				rows = append(rows, texts(row))
			}
			out = append(out, components.Table(texts(b["columns"]), rows))
		case "list":
			ordered, _ := b["ordered"].(bool)
			out = append(out, components.List(texts(b["items"]), ordered))
		case "code":
			out = append(out, components.Code(text(b, "text")))
		case "note":
			out = append(out, components.Note(text(b, "text"), text(b, "color")))
		case "link":
			out = append(out, components.Link(text(b, "text"), text(b, "url")))
		}
	}
	return out
}

// text is a string property of an object, or "".
func text(object map[string]any, key string) string {
	s, _ := object[key].(string)
	return s
}

func list(value any) []any {
	l, _ := value.([]any)
	return l
}

// number is a JSON number, which the schema's decoder and the store keep as a
// json.Number.
func number(value any) float64 {
	switch n := value.(type) {
	case json.Number:
		f, _ := n.Float64()
		return f
	case float64:
		return n
	}
	return 0
}

// texts is a list of strings or numbers, as strings.
func texts(value any) []string {
	var out []string
	for _, item := range list(value) {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

// when is a time the store normalized to RFC 3339.
func when(value any) time.Time {
	s, _ := value.(string)
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
