// Package api is ledge's JSON API on a Unix socket in the state dir, and a client
// for it.
//
//	POST   /events           an event; replies with its id
//	PUT    /sources/{source} {"items": [...]}, which become all of that source's
//	                         events; replies with their ids
//	DELETE /events/{id}      dismisses that event
//	DELETE /events           dismisses every event
//	GET    /events           the current events, as the bar shows them
//	GET    /schemas/{name}   a schema, as built in
//	GET    /status           this ledge's version, pid, start time and options
//	POST   /shutdown         stops ledge once it has replied
//
// Only your user can connect to the socket, and a web page can't reach a socket at
// all, so nothing else can post events.
package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TheWozard/ledge/internal/schema"
	"github.com/TheWozard/ledge/internal/store"
	"github.com/TheWozard/ledge/internal/version"
)

// MaxBody is the largest request body accepted.
const MaxBody = 8 << 20

// Server serves the API for one store. Changed, if set, is called after any request
// that changes the store, and Shutdown after replying to POST /shutdown. Unless
// Commands is set, an event with a command is refused. Started and Options are when
// this ledge started and what with, for GET /status.
type Server struct {
	Store    *store.Store
	Changed  func()
	Shutdown func()
	Commands bool
	Started  time.Time
	Options  Options

	once sync.Once
	mux  *http.ServeMux
}

// Listen makes the API socket at path, which only your user can connect to. A socket
// left behind by a ledge that has exited is replaced; one that answers is refused.
func Listen(path string) (net.Listener, error) {
	// macOS holds a socket's path in 104 bytes, its NUL included.
	if len(path) > 103 {
		return nil, fmt.Errorf("the API socket's path, %s, is over 103 bytes; set XDG_STATE_HOME to a shorter directory", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode().Type() != fs.ModeSocket {
			return nil, fmt.Errorf("%s is in the way of the API socket", path)
		}
		if conn, err := net.Dial("unix", path); err == nil {
			conn.Close()
			return nil, fmt.Errorf("a ledge is already running at %s", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("can't listen at %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return bufferedListener{listener}, nil
}

// SocketBuffer is the send and receive buffer of each end of a connection. macOS gives
// a Unix socket 8 KiB, which would move a large body 8 KiB per syscall.
const SocketBuffer = 4 << 20

// bufferedListener widens the buffers of each connection it accepts.
type bufferedListener struct{ net.Listener }

func (l bufferedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		widen(conn)
	}
	return conn, err
}

// widen gives conn SocketBuffer-byte buffers, if it's a Unix socket. A system that
// won't allow that much keeps what it has.
func widen(conn net.Conn) {
	if unix, ok := conn.(*net.UnixConn); ok {
		unix.SetReadBuffer(SocketBuffer)
		unix.SetWriteBuffer(SocketBuffer)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.once.Do(func() {
		s.mux = http.NewServeMux()
		s.mux.HandleFunc("POST /events", s.addEvent)
		s.mux.HandleFunc("PUT /sources/{source}", s.replaceSource)
		s.mux.HandleFunc("DELETE /events/{id}", s.dismiss)
		s.mux.HandleFunc("DELETE /events", s.clear)
		s.mux.HandleFunc("GET /events", s.events)
		s.mux.HandleFunc("GET /schemas/{name}", s.schema)
		s.mux.HandleFunc("GET /status", s.status)
		s.mux.HandleFunc("POST /shutdown", s.shutdown)
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			reply(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("no such path %s %s", r.Method, r.URL.Path)})
		})
	})
	s.mux.ServeHTTP(w, r)
}

func (s *Server) addEvent(w http.ResponseWriter, r *http.Request) {
	body, err := readJSON(r)
	if err == nil {
		err = s.checkCommand(body)
	}
	var id string
	if err == nil {
		id, err = s.Store.Add(body, time.Now())
	}
	s.respond(w, r, map[string]any{"id": id}, true, err)
}

func (s *Server) replaceSource(w http.ResponseWriter, r *http.Request) {
	body, err := readJSON(r)
	if err == nil {
		err = store.Check("section", body)
	}
	var ids []string
	if err == nil {
		items := body.(map[string]any)["items"].([]any)
		for i, item := range items {
			if err = s.checkCommand(item); err != nil {
				err = refuse("items/%d: %v", i, err)
				break
			}
		}
		if err == nil {
			ids, err = s.Store.ReplaceSource(r.PathValue("source"), items, time.Now())
		}
	}
	s.respond(w, r, map[string]any{"ids": ids}, true, err)
}

func (s *Server) dismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dismissed, err := s.Store.Remove(id)
	if err == nil && !dismissed {
		reply(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("no event %q", id)})
		return
	}
	s.respond(w, r, map[string]any{"dismissed": true}, true, err)
}

func (s *Server) clear(w http.ResponseWriter, r *http.Request) {
	dismissed, err := s.Store.Clear()
	s.respond(w, r, map[string]any{"dismissed": dismissed}, dismissed > 0, err)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	reply(w, http.StatusOK, map[string]any{"events": s.Store.Visible()})
}

func (s *Server) schema(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	source, ok := store.Schemas.Source(name)
	if !ok {
		reply(w, http.StatusNotFound, map[string]any{
			"error": fmt.Sprintf("no schema %q; there are %s", name, strings.Join(store.Schemas.Names(), ", "))})
		return
	}
	w.Header().Set("Content-Type", "application/schema+json")
	w.Write(source)
}

// Status is what GET /status replies.
type Status struct {
	Version string    `json:"version"`
	PID     int       `json:"pid"`
	Started time.Time `json:"started,omitzero"`
	Options Options   `json:"options"`
}

// Options are what a ledge runs with: `ledge run`'s flags.
type Options struct {
	Config   string `json:"config,omitempty"` // the --config file's absolute path, or "" for none
	Headless bool   `json:"headless,omitempty"`
}

// Args is the options as `ledge run` arguments.
func (o Options) Args() []string {
	var args []string
	if o.Config != "" {
		args = append(args, "--config", o.Config)
	}
	if o.Headless {
		args = append(args, "--headless")
	}
	return args
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	reply(w, http.StatusOK, Status{Version: version.Tag(), PID: os.Getpid(), Started: s.Started, Options: s.Options})
}

// shutdown replies, then stops ledge.
func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	if s.Shutdown == nil {
		reply(w, http.StatusInternalServerError, map[string]any{"error": "this server can't be shut down"})
		return
	}
	reply(w, http.StatusOK, map[string]any{"shutdown": true})
	s.Shutdown()
}

// respond replies with result, or the error: a 400 for something sent that's refused,
// otherwise a 500. If the request succeeded and changed the store, Changed is called.
func (s *Server) respond(w http.ResponseWriter, r *http.Request, result any, changed bool, err error) {
	var stored *store.Error
	var refused *refusal
	switch {
	case errors.As(err, &stored) || errors.As(err, &refused):
		reply(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	case err != nil:
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		reply(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	default:
		reply(w, http.StatusOK, result)
		if changed && s.Changed != nil {
			s.Changed()
		}
	}
}

// refusal is a request refused for what it sent, before it reaches the store.
type refusal struct{ msg string }

func (r *refusal) Error() string { return r.msg }

func refuse(format string, args ...any) error {
	return &refusal{fmt.Sprintf(format, args...)}
}

// checkCommand refuses an event with a command, unless commands are allowed.
func (s *Server) checkCommand(event any) error {
	if obj, ok := event.(map[string]any); ok && obj["command"] != nil && !s.Commands {
		return refuse(`this ledge doesn't run commands; set "commands": true in its --config to allow them`)
	}
	return nil
}

func readJSON(r *http.Request) (any, error) {
	data, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, MaxBody))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return nil, refuse("body is over %d bytes", MaxBody)
	}
	if err != nil {
		return nil, err
	}
	body, err := schema.Decode(data)
	if err != nil {
		return nil, refuse("body isn't JSON: %v", err)
	}
	return body, nil
}

func reply(w http.ResponseWriter, status int, body any) {
	data, err := store.MarshalIndent(body)
	if err != nil {
		status, data = http.StatusInternalServerError, []byte(`{"error": "can't encode the reply"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(append(data, '\n'))
}
