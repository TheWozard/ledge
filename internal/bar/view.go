package bar

import (
	"fmt"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/a-h/templ"

	"github.com/TheWozard/ledge/internal/store"
)

// view is events as the bar shows them: statuses as pills, the rest as cards, in the
// store's order. The events have been checked against the schemas.
func view(events []store.Event) ([]components.Pill, []components.Card) {
	var pills []components.Pill
	var cards []components.Card
	for _, event := range events {
		if event["kind"] == "status" {
			pills = append(pills, pill(event))
		} else {
			cards = append(cards, card(event))
		}
	}
	return pills, cards
}

func card(event store.Event) components.Card {
	url := text(event, "url")
	return components.Card{
		ID:          text(event, "id"),
		Title:       text(event, "title"),
		Description: text(event, "description"),
		Source:      text(event, "source"),
		Received:    when(event["received"]),
		Tags:        texts(event["tags"]),
		Priority:    text(event, "priority"),
		Opens:       event["report"] != nil || url != "",
		Hint:        url,
	}
}

func pill(event store.Event) components.Pill {
	p := components.Pill{
		ID:      text(event, "id"),
		Title:   text(event, "title"),
		Color:   text(event, "color"),
		Detail:  text(event, "detail"),
		Command: text(event, "command"),
		// Only an http(s) url makes the label a link; open checks again.
		Link: web.MatchString(text(event, "url")),
	}
	steps, _ := event["schedule"].([]any)
	for _, step := range steps {
		step, _ := step.(map[string]any)
		p.Schedule = append(p.Schedule, components.Step{At: when(step["at"]), Color: fmt.Sprint(step["color"])})
	}
	return p
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
