package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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
	changes, shutdowns := 0, 0
	server := &Server{Store: events, Changed: func() { changes++ }, Shutdown: func() { shutdowns++ }, Commands: true}

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
		{"POST", "/event", `{"title": "x"} {}`, nil, 400, "body isn't JSON"},
		{"POST", "/event", `{"title": "` + strings.Repeat("x", MaxBody) + `"}`, nil, 400, "body is over"},
		{"POST", "/dismiss", `{"id": "a"}`, nil, 200, `"dismissed": true`},
		{"POST", "/dismiss", `{"id": "a"}`, nil, 200, `"dismissed": false`},
		{"POST", "/dismiss", `{"id": ""}`, nil, 400, "dismiss schema: id"},
		{"GET", "/events", "", nil, 200, `"title": "B"`},
		{"GET", "/schemas/event", "", nil, 200, `"$id": "urn:ledge:event"`},
		{"POST", "/shutdown", `{"now": true}`, nil, 400, "shutdown schema"},
		{"POST", "/shutdown", `{}`, nil, 200, `"shutdown": true`},
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
	if shutdowns != 1 {
		t.Errorf("Shutdown called %d times, want 1", shutdowns)
	}
}

func TestCommandsAreOptIn(t *testing.T) {
	events, err := store.Open(filepath.Join(t.TempDir(), "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, commands := range []bool{false, true} {
		server := &Server{Store: events, Commands: commands}
		for _, body := range []string{
			`{"title": "Log in", "command": "aws sso login"}`,
			`{"source": "s", "items": [{"title": "a"}, {"title": "Log in", "command": "aws sso login"}]}`,
		} {
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, httptest.NewRequest("POST", "/event", strings.NewReader(body)))
			refused := recorder.Code == 400 && strings.Contains(recorder.Body.String(), `set \"commands\": true`)
			if refused == commands {
				t.Errorf("commands %v, %s: %d %s", commands, body, recorder.Code, recorder.Body)
			}
		}
		// Refused, a section doesn't add the items without commands either.
		if visible := events.Visible(); !commands && len(visible) != 0 {
			t.Errorf("refused requests added %v", visible)
		}
	}
}

func TestListen(t *testing.T) {
	// Not t.TempDir(): a socket's path must fit in 104 bytes.
	dir, err := os.MkdirTemp("", "ledge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "ledge.sock")

	// A socket left by a ledge that's gone.
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	stale.Close()

	listener, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen over a stale socket: %v", err)
	}
	defer listener.Close()
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("socket mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}

	events, err := store.Open(filepath.Join(dir, "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(listener, &Server{Store: events})
	var reply struct {
		Events []any `json:"events"`
	}
	if err := (Client{Socket: path}).Call("GET", "/events", nil, &reply); err != nil {
		t.Errorf("calling through the socket: %v", err)
	}

	if _, err := Listen(path); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("Listen over a live socket = %v", err)
	}
}
