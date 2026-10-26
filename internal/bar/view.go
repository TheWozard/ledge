package bar

import (
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

// view is events as the bar shows them, as of now: statuses first, then the rest, in
// the store's order. The events have been checked against the schemas.
func (b *Bar) view(events []store.Event, now time.Time) []components.Card {
	var statuses, cards []components.Card
	for _, event := range events {
		if event["kind"] == "status" {
			statuses = append(statuses, b.status(event, now))
		} else {
			cards = append(cards, b.card(event))
		}
	}
	return append(statuses, cards...)
}

func (b *Bar) card(event store.Event) components.Card {
	id := text(event, "id")
	c := components.Basic{
		ID:          id,
		Title:       text(event, "title"),
		Description: text(event, "description"),
		Color:       priorities[text(event, "priority")],
		Dismiss:     func(components.ClickEvent) { b.dismiss(id) },
	}
	if event["report"] != nil || text(event, "url") != "" {
		c.Click = func(components.ClickEvent) { b.open(id) }
	}
	return c
}

// status is a card in the colour of the latest step of its schedule that's passed
// as of now. One with a schedule counts down to its last step, and is drawn again at
// its next step, for its colour.
func (b *Bar) status(event store.Event, now time.Time) components.Card {
	id := text(event, "id")
	c := components.Basic{ID: id, Title: text(event, "title"), Description: text(event, "detail")}
	switch {
	case text(event, "command") != "":
		c.Click = func(components.ClickEvent) { b.run(id) }
	// Only an http(s) url makes the card a link; open checks again.
	case web.MatchString(text(event, "url")):
		c.Click = func(components.ClickEvent) { b.open(id) }
	}

	color := text(event, "color")
	var current, next, last time.Time
	for _, step := range list(event["schedule"]) {
		step, _ := step.(map[string]any)
		at := when(step["at"])
		if !at.After(now) && !at.Before(current) {
			current, color = at, text(step, "color")
		}
		if at.After(now) && (next.IsZero() || at.Before(next)) {
			next = at
		}
		if at.After(last) {
			last = at
		}
	}
	if color == "" {
		color = "gray"
	}
	c.Color = hex(color)
	if last.IsZero() {
		return c
	}
	return recolored{components.Timer{Basic: c, End: last}, next, b.Render}
}

// recolored is a status's card that's drawn again at next, when its colour changes.
type recolored struct {
	components.Card
	next   time.Time
	redraw func()
}

func (r recolored) Frame() components.Frame {
	f := r.Card.Frame()
	f.Ends, f.Complete = r.next, nil
	if !r.next.IsZero() {
		f.Complete = r.redraw
	}
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
