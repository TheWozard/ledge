package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheWozard/ledge/internal/store"
)

func TestServer(t *testing.T) {
	events, err := store.Open(filepath.Join(t.TempDir(), "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	changes := 0
	server := &Server{Store: events, Changed: func() { changes++ }}

	do := func(method, path, body string, headers map[string]string) (int, string) {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		return recorder.Code, recorder.Body.String()
	}

	for _, c := range []struct {
		method, path, body string
		headers            map[string]string
		status             int
		want               string
	}{
		{"POST", "/event", `{"id": "a", "title": "A"}`, nil, 200, `"id": "a"`},
		{"POST", "/event", `{"source": "ci", "items": [{"id": "b", "title": "B"}]}`, nil, 200, `"ids": [`},
		{"POST", "/event", `{"items": []}`, nil, 400, "section schema: section: missing property 'source'"},
		{"POST", "/event", `{"title": "x"}`, map[string]string{"Origin": "https://example.com"}, 400, "browser page"},
		{"POST", "/event", `{"title": "x"}`, map[string]string{"Content-Type": "text/plain"}, 400, "Content-Type must be application/json"},
		{"POST", "/event", `{"title": "x"} {}`, nil, 400, "body isn't JSON"},
		{"POST", "/event", `{"title": "` + strings.Repeat("x", MaxBody) + `"}`, nil, 400, "body is over"},
		{"POST", "/dismiss", `{"id": "a"}`, nil, 200, `"dismissed": true`},
		{"POST", "/dismiss", `{"id": "a"}`, nil, 200, `"dismissed": false`},
		{"POST", "/dismiss", `{"id": ""}`, nil, 400, "dismiss schema: id"},
		{"GET", "/events", "", nil, 200, `"title": "B"`},
		{"GET", "/schemas/status", "", nil, 200, `"$id": "urn:ledge:status"`},
		{"GET", "/schemas/nope", "", nil, 404, "no schema"},
		{"GET", "/event", "", nil, 404, "no such path"},
	} {
		status, body := do(c.method, c.path, c.body, c.headers)
		if status != c.status || !strings.Contains(body, c.want) {
			t.Errorf("%s %s %.60s: %d %s; want %d containing %q", c.method, c.path, c.body, status, body, c.status, c.want)
		}
	}
	// The two events, the section and both dismisses, refused ones not counted.
	if changes != 4 {
		t.Errorf("Changed called %d times, want 4", changes)
	}
}
