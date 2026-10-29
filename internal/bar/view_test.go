package bar

import (
	"context"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"

	"github.com/TheWozard/ledge/internal/schema"
	"github.com/TheWozard/ledge/internal/store"
)

// reportJSON is a report with one of each block, its text partly HTML-looking, and
// text, links and buttons nested in them.
const reportJSON = `[
	{"type": "heading", "text": "Summary"},
	{"type": "text", "text": ["<b>not bold</b>\nsecond line, ", {"type": "strong", "text": ["really ", {"type": "em", "text": "bold"}]},
		" and ", {"type": "mono", "text": "go test"}]},
	{"type": "fields", "items": [{"label": "Branch", "value": "main"}, {"label": "Failures", "value": 2}]},
	{"type": "table", "columns": ["Job", "Result", "Logs", "Do"], "rows": [["lint", "ok"], ["test"],
		["build", 3, {"type": "link", "text": "logs", "url": "https://example.com/build"},
		 {"type": "buttons", "items": [{"title": "Retry", "color": "red", "command": "retry build"}, {"title": "Open", "url": "https://example.com/open"}]}]]},
	{"type": "list", "items": ["one", {"text": "two", "ordered": true, "items": ["two.a"]}], "ordered": true},
	{"type": "code", "text": "$ go test\nFAIL"},
	{"type": "note", "text": "Flaky", "color": "yellow"},
	{"type": "quote", "text": "said so"},
	{"type": "link", "text": "The run", "url": "https://example.com/from-report"},
	{"type": "buttons", "items": [{"title": "Done", "color": "green", "dismiss": true}, {"title": "Nothing"}]},
	{"type": "image", "src": "data:image/png;base64,AA==", "alt": "a dot"},
	{"type": "details", "summary": "More", "blocks": [{"type": "text", "text": "hidden"}]},
	{"type": "rule"}
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
	cards := (&Bar{commands: true}).view(events, now)
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
	if noCommands := (&Bar{}).view(events[3:4], now)[0]; noCommands.Frame().Click != nil {
		t.Error("a command shouldn't be clickable when commands aren't allowed")
	}
	if last := (&Bar{commands: true}).view(events[3:4], now.Add(90*time.Minute))[0]; last.Frame().Color != colors["red"] || last.Frame().Complete != nil {
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
	blocks := (&report{bar: &Bar{}, event: event}).blocks(event.Report)
	for _, block := range blocks {
		block.Render(context.Background(), &html)
	}
	for _, want := range []string{
		"<h2>Summary</h2>",
		"<p>&lt;b&gt;not bold&lt;/b&gt;\nsecond line, <strong>really <em>bold</em></strong> and <code>go test</code></p>",
		"<dt>Failures</dt><dd>2</dd>",
		"<tr><td>test</td><td></td><td></td><td></td></tr>", "<td>build</td><td>3</td>",
		`<a href="https://example.com/build" title="https://example.com/build">logs</a>`,
		"<ol><li>one</li><li>two<ol><li>two.a</li></ol></li></ol>", "<pre><code>$ go test\nFAIL</code></pre>",
		`data-color="yellow"`, "<blockquote>said so</blockquote>", `<a href="https://example.com/from-report"`,
		"Retry</button>", "Done</button>", `alt="a dot"`, "<summary>More</summary> <p>hidden</p></details>", "<hr>",
	} {
		if !strings.Contains(html.String(), want) {
			t.Errorf("missing %q in:\n%s", want, html.String())
		}
	}
	// Every block is one the frontend renders.
	if len(blocks) != len(event.Report) || len(event.Report) != 13 {
		t.Errorf("%d blocks became %d components", len(event.Report), len(blocks))
	}
}

// A report's buttons open their url, run their command if commands are allowed, or
// dismiss their event.
func TestReportButtons(t *testing.T) {
	event := normalize(t, `{"id": "ci", "title": "CI", "report": `+reportJSON+`}`)
	for _, commands := range []bool{false, true} {
		var started []string
		b := &Bar{commands: commands, stateDir: t.TempDir(), start: func(cmd *exec.Cmd) error {
			started = append(started, strings.Join(cmd.Args, " "))
			return nil
		}}
		clicks := &components.Clicks{}
		ctx := components.WithClicks(context.Background(), clicks)
		for _, block := range (&report{bar: b, event: event}).blocks(event.Report) {
			block.Render(ctx, io.Discard)
		}
		// Retry, Open, Done and Nothing, in order; Retry only with commands, and Nothing never.
		want := []string{"button-2", "button-3"}
		if commands {
			want = []string{"button-1", "button-2", "button-3"}
		}
		if got := clicks.IDs(); !slices.Equal(got, want) {
			t.Fatalf("with commands %v, clickable buttons = %v, want %v", commands, got, want)
		}
		clicks.Click("button-2")(components.ClickEvent{})
		if commands {
			clicks.Click("button-1")(components.ClickEvent{})
		}
		want = []string{"open https://example.com/open"}
		if commands {
			want = append(want, "/bin/bash -lc retry build")
		}
		if !slices.Equal(started, want) {
			t.Errorf("with commands %v, started %q, want %q", commands, started, want)
		}
	}
}
