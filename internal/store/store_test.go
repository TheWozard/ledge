package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheWozard/ledge/internal/schema"
)

var received = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func decode(t *testing.T, text string) any {
	t.Helper()
	value, err := schema.Decode([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestNormalize(t *testing.T) {
	event, err := Normalize(decode(t, `{"kind": "timer", "title": "AWS", "expires": "2d", "end": "2h",
		"schedule": [{"at": "1h", "color": "red"}, {"at": "2026-10-14T14:30:00+02:00", "color": "yellow"}]}`), "", received)
	if err != nil {
		t.Fatal(err)
	}
	if len(event.ID) != 12 {
		t.Errorf("id = %q, want 12 random hex characters", event.ID)
	}
	if event.Source != "manual" || event.Kind != "timer" || !event.Received.Equal(received) ||
		!event.Expires.Equal(received.Add(48*time.Hour)) || !event.End.Equal(received.Add(2*time.Hour)) {
		t.Errorf("event = %+v", event)
	}
	// Sorted by time, and an ISO time keeps its offset.
	if at := event.Schedule[0].At.Format(time.RFC3339); at != "2026-10-14T14:30:00+02:00" || event.Schedule[0].Color != "yellow" {
		t.Errorf("first step = %s %s", at, event.Schedule[0].Color)
	}
	if at := event.Schedule[1].At.Format(time.RFC3339); at != "2026-10-14T13:00:00Z" {
		t.Errorf("second step at %s", at)
	}
}

func TestNormalizeRefuses(t *testing.T) {
	for body, want := range map[string]string{
		`[1]`:                              "expected a JSON object",
		`{"kind": "status", "title": "x"}`: "kind: value must be one of 'basic', 'timer'",
		`{"title": "x", "url": "https://a", "command": "ls"}`:                         "only one of `url`, `report` and `command` can be set",
		`{"title": "x", "value": 3}`:                                                  "value: not a property this object can have",
		`{"kind": "timer", "title": "x"}`:                                             "a timer needs `start`, `end` or both",
		`{"kind": "timer", "title": "x", "start": "soon"}`:                            `start "soon" isn't an ISO 8601 time`,
		`{"kind": "progress", "title": "x", "value": 2}`:                              "value: maximum: got 2, want 1",
		`{"kind": "stat", "title": "x"}`:                                              "missing property 'value'",
		`{"kind": "checks", "title": "x", "checks": [{"name": "a", "state": "ok"}]}`:  "checks/0/state: value must be one of",
		`{"kind": "message", "title": "x", "from": "a", "avatar": "https://a/b.png"}`: "avatar: 'https://a/b.png' does not match pattern",
		`{"tittle": "x"}`:                                                                         "event schema: event: missing property 'title'",
		`{"title": "x", "expires": "no"}`:                                                         `expires "no" isn't an ISO 8601 time`,
		`{"title": "x", "url": "https://a", "report": []}`:                                        "only one of `url`, `report` and `command` can be set",
		`{"title": "x", "report": "notes.html"}`:                                                  "report: got string, want array",
		`{"title": "x", "report": []}`:                                                            "report: minItems: got 0, want 1",
		`{"title": "x", "report": [{"text": "t"}]}`:                                               "report/0: missing property 'type'",
		`{"title": "x", "report": [{"type": "chart"}]}`:                                           "report/0/type: value must be one of 'heading', 'text'",
		`{"title": "x", "report": [{"type": "heading"}]}`:                                         "report/0: missing property 'text'",
		`{"title": "x", "report": [{"type": "text", "text": "t", "color": "red"}]}`:               "report/0: additional properties 'color' not allowed",
		`{"title": "x", "report": [{"type": "note", "text": "t", "color": "pink"}]}`:              "report/0/color: 'pink' does not match pattern",
		`{"title": "x", "report": [{"type": "link", "text": "t", "url": "javascript:alert(1)"}]}`: "report/0/url: 'javascript:alert(1)' does not match pattern",
		`{"title": "x", "report": [{"type": "table", "columns": ["a"], "rows": [[{}]]}]}`:         "report/0/rows/0/0: got object, want number or string",
		`{"title": "x", "report": [{"type": "fields", "items": [{"label": "a"}]}]}`:               "report/0/items/0: missing property 'value'",
	} {
		_, err := Normalize(decode(t, body), "", received)
		if _, ok := err.(*Error); !ok || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want an *Error containing %q", body, err, want)
		}
	}
}

// Every kind, and a report with every block, decodes into an Event and encodes back
// to what was sent, give or take what Normalize adds.
func TestEventJSON(t *testing.T) {
	for _, body := range []string{
		`{"kind": "basic", "title": "<b>x</b>", "url": "https://a"}`,
		`{"kind": "basic", "title": "x", "report": [{"type": "heading", "text": "h"}, {"type": "text", "text": "t"},
			{"type": "fields", "items": [{"label": "a", "value": 2}, {"label": "b", "value": "2"}]},
			{"type": "table", "columns": ["a", "b"], "rows": [["x", 1.50], []]}, {"type": "table", "columns": ["a"], "rows": []},
			{"type": "list", "items": ["one"], "ordered": true}, {"type": "code", "text": "c"},
			{"type": "note", "text": "n", "color": "red"}, {"type": "link", "text": "l", "url": "https://a"}]}`,
		`{"kind": "progress", "title": "x", "value": 0.25, "label": "1/4"}`,
		`{"kind": "stat", "title": "x", "value": "2.3%", "change": "↑1", "change_color": "red", "history": [1, 2.5]}`,
		`{"kind": "stat", "title": "x", "value": 7}`,
		`{"kind": "checks", "title": "x", "checks": [{"name": "a", "state": "passed"}]}`,
		`{"kind": "message", "title": "x", "from": "a", "avatar": "data:image/png;base64,AA=="}`,
	} {
		event, err := Normalize(decode(t, body), "", received)
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		data, err := marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		got := decode(t, string(data)).(map[string]any)
		for _, added := range []string{"id", "source", "received"} {
			delete(got, added)
		}
		gotJSON, _ := marshal(got)
		wantJSON, _ := marshal(decode(t, body))
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("round trip:\n got %s\nwant %s", gotJSON, wantJSON)
		}
	}
}

func TestParseTime(t *testing.T) {
	for text, want := range map[string]string{
		"2030-01-02T03:04:05Z":        "2030-01-02T03:04:05Z",
		"2030-01-02T03:04:05.5+05:30": "2030-01-02T03:04:05+05:30",
		"2030-01-02 03:04:05-0700":    "2030-01-02T03:04:05-07:00",
		"2030-01-02T03:04+01:00":      "2030-01-02T03:04:00+01:00",
	} {
		moment, err := resolveTime(text, "t", received)
		if err != nil || moment.Format(time.RFC3339) != want {
			t.Errorf("resolveTime(%q) = %v, %v; want %s", text, moment.Format(time.RFC3339), err, want)
		}
	}
	local, err := parseTime("2030-01-02", "t")
	if err != nil || local.Location() != time.Local || local.Hour() != 0 {
		t.Errorf("a date is midnight local time, got %v, %v", local, err)
	}
}

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	add := func(body string) string {
		t.Helper()
		id, err := s.Add(decode(t, body), received)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	add(`{"id": "low", "title": "l", "priority": "low"}`)
	add(`{"id": "plain", "title": "p"}`)
	add(`{"id": "old", "title": "o", "expires": "2020-01-01T00:00:00Z"}`)
	ids, err := s.ReplaceSource("ci", []any{decode(t, `{"id": "c1", "title": "a"}`), decode(t, `{"id": "c2", "title": "b", "priority": "high"}`)}, received)
	if err != nil || strings.Join(ids, ",") != "c1,c2" {
		t.Fatalf("ReplaceSource = %v, %v", ids, err)
	}
	add(`{"id": "low", "title": "replaced", "priority": "low"}`)

	order := func(s *Store) string {
		var ids []string
		for _, e := range s.Visible() {
			ids = append(ids, e.ID)
		}
		return strings.Join(ids, ",")
	}
	// By priority, with none last; same-second ties stay in the order first added,
	// and a replaced event keeps its place.
	if got, want := order(s), "c2,low,plain,old,c1"; got != want {
		t.Errorf("Visible = %s, want %s", got, want)
	}

	if _, err := s.ReplaceSource("ci", []any{decode(t, `{"title": "c"}`), decode(t, `{"nope": 1}`)}, received); err == nil || !strings.HasPrefix(err.Error(), "items/1: ") {
		t.Errorf("a bad item: %v", err)
	}
	if _, ok := s.Get("c1"); !ok {
		t.Error("a refused section changed the source")
	}

	if pruned, err := s.Prune(received); !pruned || err != nil {
		t.Errorf("Prune = %v, %v", pruned, err)
	}
	if removed, err := s.Remove("plain"); !removed || err != nil {
		t.Errorf("Remove = %v, %v", removed, err)
	}
	if removed, _ := s.Remove("plain"); removed {
		t.Error("removed twice")
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := order(reopened), "c2,low,c1"; got != want {
		t.Errorf("after reopening, Visible = %s, want %s", got, want)
	}
	if event, _ := reopened.Get("low"); event.Title != "replaced" {
		t.Errorf("after reopening, low = %v", event)
	}
}

// An id is the source's that sent it: another source can't replace it, alone or in a
// section, and a section can't use an id twice.
func TestIDsBelongToTheirSource(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(decode(t, `{"id": "x", "title": "a's", "source": "a"}`), received); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(decode(t, `{"id": "x", "title": "a's, again", "source": "a"}`), received); err != nil {
		t.Errorf("the same source replacing its event: %v", err)
	}
	if _, err := s.Add(decode(t, `{"id": "x", "title": "manual"}`), received); err == nil || !strings.Contains(err.Error(), `source "a"'s event`) {
		t.Errorf("another source's Add = %v", err)
	}
	if _, err := s.ReplaceSource("b", []any{decode(t, `{"id": "x", "title": "b's"}`)}, received); err == nil || !strings.HasPrefix(err.Error(), "items/0: ") {
		t.Errorf("another source's section = %v", err)
	}
	if _, err := s.ReplaceSource("b", []any{decode(t, `{"id": "y", "title": "1"}`), decode(t, `{"id": "y", "title": "2"}`)}, received); err == nil || !strings.Contains(err.Error(), "items/1: id \"y\" is items/0's too") {
		t.Errorf("a section using an id twice = %v", err)
	}
	if event, _ := s.Get("x"); event.Title != "a's, again" || event.Source != "a" {
		t.Errorf("x = %+v", event)
	}
}

// A file that isn't this version's, such as one from before the version, is set aside.
func TestOpenSetsAsideAnOldFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.json")
	old := `{"events": {"a": {"title": "A", "report": "notes.html"}}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if visible := s.Visible(); len(visible) != 0 {
		t.Errorf("Visible = %v, want none", visible)
	}
	if kept, err := os.ReadFile(path + ".bad"); err != nil || string(kept) != old {
		t.Errorf("%s.bad = %q, %v", path, kept, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the old file is still at %s", path)
	}
}
