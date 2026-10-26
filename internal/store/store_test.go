package store

import (
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
	if id, _ := event["id"].(string); len(id) != 12 {
		t.Errorf("id = %q, want 12 random hex characters", id)
	}
	checks := map[string]any{
		"source":   "manual",
		"received": "2026-10-14T12:00:00+00:00",
		"expires":  "2026-10-16T12:00:00+00:00",
		"end":      "2026-10-14T14:00:00+00:00",
	}
	for key, want := range checks {
		if event[key] != want {
			t.Errorf("%s = %v, want %v", key, event[key], want)
		}
	}
	// Sorted by time, and an ISO time keeps its offset.
	schedule := event["schedule"].([]any)
	if at := schedule[0].(map[string]any)["at"]; at != "2026-10-14T14:30:00+02:00" {
		t.Errorf("first step at %v", at)
	}
	if at := schedule[1].(map[string]any)["at"]; at != "2026-10-14T13:00:00+00:00" {
		t.Errorf("second step at %v", at)
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

func TestParseTime(t *testing.T) {
	for text, want := range map[string]string{
		"2030-01-02T03:04:05Z":        "2030-01-02T03:04:05+00:00",
		"2030-01-02T03:04:05.5+05:30": "2030-01-02T03:04:05+05:30",
		"2030-01-02 03:04:05-0700":    "2030-01-02T03:04:05-07:00",
		"2030-01-02T03:04+01:00":      "2030-01-02T03:04:00+01:00",
	} {
		moment, err := ParseTime(text, "t")
		if err != nil || moment.Format(TimeLayout) != want {
			t.Errorf("ParseTime(%q) = %v, %v; want %s", text, moment.Format(TimeLayout), err, want)
		}
	}
	local, err := ParseTime("2030-01-02", "t")
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
			ids = append(ids, e["id"].(string))
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
	if event, _ := reopened.Get("low"); event["title"] != "replaced" {
		t.Errorf("after reopening, low = %v", event)
	}
}
