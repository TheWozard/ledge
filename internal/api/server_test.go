package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/TheWozard/ledge/internal/store"
)

func TestServer(t *testing.T) {
	events, err := store.Open(filepath.Join(t.TempDir(), "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	changes, shutdowns := 0, 0
	server := &Server{Store: events, Changed: func() { changes++ }, Shutdown: func() { shutdowns++ }, Commands: true,
		Started: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)}

	for _, c := range []struct {
		method, path, body string
		status             int
		want               string
	}{
		{"POST", "/events", `{"id": "a", "title": "A"}`, 200, `"id": "a"`},
		{"POST", "/events", `{"id": "a/b", "title": "A slash"}`, 200, `"id": "a/b"`},
		{"PUT", "/sources/ci", `{"items": [{"id": "b", "title": "B"}]}`, 200, `"ids": [`},
		{"PUT", "/sources/c%2Fd", `{"items": [{"id": "c", "title": "C"}]}`, 200, `"ids": [`},
		{"PUT", "/sources/ci", `{"source": "ci", "items": []}`, 400, "section schema: section: additional properties 'source'"},
		{"PUT", "/sources/ci", `{"items": [{"nope": 1}]}`, 400, "items/0: event schema"},
		{"POST", "/events", `{"title": "x"} {}`, 400, "body isn't JSON"},
		{"POST", "/events", `{"title": "` + strings.Repeat("x", MaxBody) + `"}`, 400, "body is over"},
		{"DELETE", "/events/a", "", 200, `"dismissed": true`},
		{"DELETE", "/events/a", "", 404, `no event \"a\"`},
		{"DELETE", "/events/a%2Fb", "", 200, `"dismissed": true`},
		{"GET", "/events", "", 200, `"title": "B"`},
		{"GET", "/events", "", 200, `"source": "c/d"`},
		{"GET", "/schemas/event", "", 200, `"$id": "urn:ledge:event"`},
		{"GET", "/schemas/nope", "", 404, "no schema"},
		{"GET", "/status", "", 200, `"started": "2026-10-26T12:00:00Z"`},
		{"POST", "/shutdown", "", 200, `"shutdown": true`},
		{"POST", "/event", `{"title": "x"}`, 404, "no such path POST /event"},
		{"DELETE", "/events", "", 404, "no such path DELETE /events"},
	} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(c.method, c.path, strings.NewReader(c.body)))
		if body := recorder.Body.String(); recorder.Code != c.status || !strings.Contains(body, c.want) {
			t.Errorf("%s %s %.60s: %d %s; want %d containing %q", c.method, c.path, c.body, recorder.Code, body, c.status, c.want)
		}
	}
	// Two events, two sections and two dismisses: refused ones, and a dismiss of
	// nothing, aren't changes.
	if changes != 6 {
		t.Errorf("Changed called %d times, want 6", changes)
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
		for _, request := range []struct{ method, path, body string }{
			{"POST", "/events", `{"title": "Log in", "command": "aws sso login"}`},
			{"PUT", "/sources/s", `{"items": [{"title": "a"}, {"title": "Log in", "command": "aws sso login"}]}`},
		} {
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, strings.NewReader(request.body)))
			refused := recorder.Code == 400 && strings.Contains(recorder.Body.String(), `set \"commands\": true`)
			if refused == commands {
				t.Errorf("commands %v, %s: %d %s", commands, request.body, recorder.Code, recorder.Body)
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

// Both ends of a connection get SocketBuffer-byte buffers: the listener's and the
// client's.
func TestSocketBuffers(t *testing.T) {
	dir, err := os.MkdirTemp("", "ledge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	listener, err := Listen(filepath.Join(dir, "ledge.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()
	client, err := net.Dial("unix", filepath.Join(dir, "ledge.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	widen(client)
	server := <-accepted
	defer server.Close()

	for name, conn := range map[string]net.Conn{"client": client, "server": server} {
		raw, err := conn.(*net.UnixConn).SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		raw.Control(func(fd uintptr) {
			for _, option := range []int{syscall.SO_SNDBUF, syscall.SO_RCVBUF} {
				if size, err := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, option); err != nil || size < SocketBuffer {
					t.Errorf("%s: socket option %d = %d, %v; want at least %d", name, option, size, err, SocketBuffer)
				}
			}
		})
	}
}
