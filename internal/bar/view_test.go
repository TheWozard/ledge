package bar

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"

	"github.com/TheWozard/ledge/internal/schema"
	"github.com/TheWozard/ledge/internal/store"
)

// reportJSON is a report with one of each block, its text partly HTML-looking.
const reportJSON = `[
	{"type": "heading", "text": "Summary"},
	{"type": "text", "text": "<b>not bold</b>\nsecond line"},
	{"type": "fields", "items": [{"label": "Branch", "value": "main"}, {"label": "Failures", "value": 2}]},
	{"type": "table", "columns": ["Job", "Result"], "rows": [["lint", "ok"], ["test"], ["build", 3]]},
	{"type": "list", "items": ["one", "two"], "ordered": true},
	{"type": "code", "text": "$ go test\nFAIL"},
	{"type": "note", "text": "Flaky", "color": "yellow"},
	{"type": "link", "text": "The run", "url": "https://example.com/from-report"}
]`

var now = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// normalize is body as the store keeps it.
func normalize(t *testing.T, body string) store.Event {
	t.Helper()
	value, err := schema.Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	event, err := store.Normalize(value, "", now)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestView(t *testing.T) {
	events := []store.Event{
		normalize(t, `{"id": "deploy", "title": "Deploy", "description": "done", "priority": "high", "url": "https://example.com/deploy"}`),
		normalize(t, `{"id": "notes", "title": "Notes", "report": [{"type": "text", "text": "x"}]}`),
		normalize(t, `{"id": "plain", "title": "Plain", "priority": "low", "color": "#123456"}`),
		normalize(t, `{"kind": "timer", "id": "aws", "title": "AWS", "color": "green", "command": "aws sso login",
			"end": "2h", "schedule": [{"at": "2026-10-15T11:00:00Z", "color": "yellow"}, {"at": "1h", "color": "red"}]}`),
		normalize(t, `{"kind": "progress", "id": "progress", "title": "Upload", "value": 0.4, "label": "2/5"}`),
		normalize(t, `{"kind": "stat", "id": "stat", "title": "Errors", "value": 2.5, "change": "↑1", "change_color": "red",
			"history": [1, 2, 2.5]}`),
		normalize(t, `{"kind": "checks", "id": "checks", "title": "CI", "checks": [{"name": "lint", "state": "passed"},
			{"name": "test", "state": "failed"}]}`),
		normalize(t, `{"kind": "message", "id": "message", "title": "#general", "from": "Ada Lovelace", "sent": "2026-10-15T11:55:00Z"}`),
	}
	cards := (&Bar{}).view(events, now)
	if len(cards) != len(events) {
		t.Fatalf("%d cards, want %d", len(cards), len(events))
	}
	var kinds []string
	for _, c := range cards {
		kinds = append(kinds, c.Frame().ID+":"+c.Frame().Kind)
	}
	if got := strings.Join(kinds, " "); got != "deploy:basic notes:basic plain:basic aws:timer progress:progress stat:stat checks:checks message:message" {
		t.Errorf("kinds = %s", got)
	}

	deploy := cards[0].(components.Basic)
	if deploy.Title != "Deploy" || deploy.Description != "done" || deploy.Color != colors["red"] ||
		deploy.Click == nil || deploy.Dismiss == nil {
		t.Errorf("deploy card = %+v", deploy)
	}
	if cards[1].Frame().Click == nil {
		t.Error("a card with a report should open")
	}
	// A colour beats the priority's; nothing to open makes it unclickable.
	if plain := cards[2].Frame(); plain.Click != nil || plain.Color != "#123456" {
		t.Errorf("a plain card = %+v", plain)
	}

	// aws is yellow, from the step that's passed, until the next, and counts down to its end.
	aws := cards[3].Frame()
	if aws.Color != colors["yellow"] || aws.Click == nil || !aws.Ends.Equal(now.Add(time.Hour)) || aws.Complete == nil {
		t.Errorf("aws card = %+v", aws)
	}
	if timer := cards[3].(recolored).Card.(components.Timer); !timer.End.Equal(now.Add(2*time.Hour)) || !timer.Start.IsZero() {
		t.Errorf("aws timer = %+v", timer)
	}
	if last := (&Bar{}).view(events[3:4], now.Add(90*time.Minute))[0]; last.Frame().Color != colors["red"] || last.Frame().Complete != nil {
		t.Errorf("aws after its last step = %+v", last.Frame())
	}

	if p := cards[4].(components.Progress); p.Value != 0.4 || p.Label != "2/5" {
		t.Errorf("progress = %+v", p)
	}
	if s := cards[5].(components.Stat); s.Value != "2.5" || s.Change != "↑1" || s.ChangeColor != "red" || len(s.History) != 3 || s.History[2] != 2.5 {
		t.Errorf("stat = %+v", s)
	}
	if c := cards[6].(components.Checks); len(c.Checks) != 2 || c.Checks[1].Name != "test" || c.Checks[1].State != "failed" {
		t.Errorf("checks = %+v", c)
	}
	if m := cards[7].(components.Message); m.From != "Ada Lovelace" || !m.Sent.Equal(now.Add(-5*time.Minute)) {
		t.Errorf("message = %+v", m)
	}
}

func TestBlocks(t *testing.T) {
	event := normalize(t, `{"title": "x", "report": `+reportJSON+`}`)
	var html strings.Builder
	for _, block := range blocks(event["report"].([]any)) {
		block.Render(context.Background(), &html)
	}
	for _, want := range []string{
		"<h2>Summary</h2>", "<p>&lt;b&gt;not bold&lt;/b&gt;\nsecond line</p>",
		"<dt>Failures</dt><dd>2</dd>",
		"<tr><td>test</td><td></td></tr>", "<tr><td>build</td><td>3</td></tr>",
		"<ol><li>one</li><li>two</li></ol>", "<pre><code>$ go test\nFAIL</code></pre>",
		`data-color="yellow"`, `<a href="https://example.com/from-report"`,
	} {
		if !strings.Contains(html.String(), want) {
			t.Errorf("missing %q in:\n%s", want, html.String())
		}
	}
	// Every block is one the frontend renders.
	if got, want := strings.Count(reportJSON, `"type"`), len(blocks(event["report"].([]any))); got != want {
		t.Errorf("%d blocks became %d components", got, want)
	}
}
