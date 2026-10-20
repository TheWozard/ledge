package bar

import (
	"context"
	"strings"
	"testing"
	"time"

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
			"tags": ["prod"], "url": "https://example.com/deploy"}`),
		normalize(t, `{"id": "notes", "title": "Notes", "report": [{"type": "text", "text": "x"}]}`),
		normalize(t, `{"id": "plain", "title": "Plain"}`),
		normalize(t, `{"kind": "status", "id": "aws", "title": "AWS", "color": "green", "command": "aws sso login",
			"detail": "Session", "schedule": [{"at": "2026-10-15T13:00:00Z", "color": "red"}]}`),
		normalize(t, `{"kind": "status", "id": "web", "title": "Web", "url": "https://example.com"}`),
		normalize(t, `{"kind": "status", "id": "file", "title": "File", "url": "file:///etc/hosts"}`),
	}
	pills, cards := view(events)

	if len(cards) != 3 || len(pills) != 3 {
		t.Fatalf("%d cards and %d pills, want 3 and 3", len(cards), len(pills))
	}
	deploy := cards[0]
	if deploy.ID != "deploy" || deploy.Title != "Deploy" || deploy.Description != "done" || deploy.Source != "ci" ||
		deploy.Priority != "high" || strings.Join(deploy.Tags, ",") != "prod" || !deploy.Opens ||
		deploy.Hint != "https://example.com/deploy" || !deploy.Received.Equal(now) {
		t.Errorf("deploy card = %+v", deploy)
	}
	if !cards[1].Opens || cards[1].Hint != "" {
		t.Errorf("a card with a report should open, with no hint: %+v", cards[1])
	}
	if cards[2].Opens || cards[2].Source != "manual" {
		t.Errorf("a plain card = %+v", cards[2])
	}

	aws := pills[0]
	if aws.Command != "aws sso login" || aws.Detail != "Session" || aws.Color != "green" || len(aws.Schedule) != 1 ||
		aws.Schedule[0].Color != "red" || !aws.Schedule[0].At.Equal(now.Add(time.Hour)) {
		t.Errorf("aws pill = %+v", aws)
	}
	// Only an http(s) url makes a pill a link.
	if !pills[1].Link || pills[2].Link {
		t.Errorf("links: web %v, file %v", pills[1].Link, pills[2].Link)
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
