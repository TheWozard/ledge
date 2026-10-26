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
		normalize(t, `{"id": "deploy", "title": "Deploy", "description": "done", "source": "ci", "priority": "high",
			"url": "https://example.com/deploy"}`),
		normalize(t, `{"id": "notes", "title": "Notes", "report": [{"type": "text", "text": "x"}]}`),
		normalize(t, `{"id": "plain", "title": "Plain"}`),
		normalize(t, `{"kind": "status", "id": "aws", "title": "AWS", "color": "green", "command": "aws sso login",
			"detail": "Session", "schedule": [{"at": "2026-10-15T11:00:00Z", "color": "yellow"},
			{"at": "2026-10-15T13:00:00Z", "color": "red"}, {"at": "2026-10-15T14:00:00Z", "color": "#123456"}]}`),
		normalize(t, `{"kind": "status", "id": "web", "title": "Web", "url": "https://example.com"}`),
		normalize(t, `{"kind": "status", "id": "file", "title": "File", "url": "file:///etc/hosts"}`),
	}
	cards := (&Bar{}).view(events, now)
	if len(cards) != 6 {
		t.Fatalf("%d cards, want 6", len(cards))
	}
	var ids []string
	for _, c := range cards {
		ids = append(ids, c.Frame().ID)
	}
	// Statuses come first.
	if got := strings.Join(ids, " "); got != "aws web file deploy notes plain" {
		t.Errorf("order = %s", got)
	}

	deploy := cards[3].(components.Basic)
	if deploy.Title != "Deploy" || deploy.Description != "done" || deploy.Color != colors["red"] ||
		deploy.Click == nil || deploy.Dismiss == nil {
		t.Errorf("deploy card = %+v", deploy)
	}
	if cards[4].Frame().Click == nil {
		t.Error("a card with a report should open")
	}
	if plain := cards[5].Frame(); plain.Click != nil || plain.Color != "" {
		t.Errorf("a plain card = %+v", plain)
	}

	// aws is yellow, from its first step, until its second, and counts down to its last.
	aws := cards[0].Frame()
	if aws.Kind != "timer" || aws.Color != colors["yellow"] || aws.Click == nil || aws.Dismiss != nil ||
		!aws.Ends.Equal(now.Add(time.Hour)) || aws.Complete == nil {
		t.Errorf("aws card = %+v", aws)
	}
	if timer := cards[0].(recolored).Card.(components.Timer); !timer.End.Equal(now.Add(2*time.Hour)) || timer.Description != "Session" {
		t.Errorf("aws timer = %+v", timer)
	}
	if last := (&Bar{}).view(events[3:4], now.Add(3*time.Hour))[0].Frame(); last.Color != "#123456" || last.Complete != nil {
		t.Errorf("aws after its last step = %+v", last)
	}
	// Only an http(s) url makes a status clickable; one without a colour is gray.
	if web, file := cards[1].Frame(), cards[2].Frame(); web.Click == nil || file.Click != nil || web.Color != colors["gray"] {
		t.Errorf("web %+v, file %+v", web, file)
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
