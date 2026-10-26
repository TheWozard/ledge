// Package api is ledge's JSON API on a Unix socket in the state dir, and a client
// for it.
//
//	POST /event          one event, or a section ({"source", "items"}) that
//	                     replaces a source's events; replies with the id(s)
//	POST /dismiss        {"id": ...}, deleting that event
//	POST /shutdown       {}, stopping ledge once it has replied
//	GET  /events         the current events, as the bar shows them
//	GET  /schemas/{name} a schema, as built in
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
	"time"

	"github.com/TheWozard/ledge/internal/schema"
	"github.com/TheWozard/ledge/internal/store"
)

// MaxBody is the largest request body accepted.
const MaxBody = 8 << 20

// Server serves the API for one store. Changed, if set, is called after any request
// that changes the store, and Shutdown after replying to POST /shutdown.
type Server struct {
	Store    *store.Store
	Changed  func()
	Shutdown func()
}

// Listen makes the API socket at path, which only your user can connect to. A socket
// left behind by a ledge that has exited is replaced; one that answers is refused.
func Listen(path string) (net.Listener, error) {
	// macOS holds a socket's path in 104 bytes, its NUL included.
	if len(path) > 103 {
		return nil, fmt.Errorf("the API socket's path, %s, is over 103 bytes; set XDG_STATE_HOME to a shorter directory", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
	return listener, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/events":
		reply(w, http.StatusOK, map[string]any{"events": s.Store.Visible()})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/schemas/"):
		name := strings.TrimPrefix(r.URL.Path, "/schemas/")
		source, ok := store.Schemas.Source(name)
		if !ok {
			reply(w, http.StatusNotFound, map[string]any{
				"error": fmt.Sprintf("no schema %q; there are %s", name, strings.Join(store.Schemas.Names(), ", "))})
			return
		}
		w.Header().Set("Content-Type", "application/schema+json")
		w.Write(source)
	case r.Method == http.MethodPost && (r.URL.Path == "/event" || r.URL.Path == "/dismiss"):
		s.post(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/shutdown":
		s.shutdown(w, r)
	default:
		reply(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("no such path %s %s", r.Method, r.URL.Path)})
	}
}

func (s *Server) post(w http.ResponseWriter, r *http.Request) {
	result, err := s.handle(r)
	var refused *store.Error
	switch {
	case errors.As(err, &refused):
		reply(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	case err != nil:
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		reply(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	reply(w, http.StatusOK, result)
	if s.Changed != nil {
		s.Changed()
	}
}

// shutdown replies, then stops ledge.
func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	body, err := readJSON(r)
	if err == nil {
		err = store.Check("shutdown", body)
	}
	switch {
	case err != nil:
		reply(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	case s.Shutdown == nil:
		reply(w, http.StatusInternalServerError, map[string]any{"error": "this server can't be shut down"})
	default:
		reply(w, http.StatusOK, map[string]any{"shutdown": true})
		s.Shutdown()
	}
}

func (s *Server) handle(r *http.Request) (map[string]any, error) {
	body, err := readJSON(r)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if r.URL.Path == "/dismiss" {
		if err := store.Check("dismiss", body); err != nil {
			return nil, err
		}
		dismissed, err := s.Store.Remove(body.(map[string]any)["id"].(string))
		return map[string]any{"dismissed": dismissed}, err
	}
	if obj, ok := body.(map[string]any); ok {
		if _, isSection := obj["items"]; isSection {
			if err := store.Check("section", obj); err != nil {
				return nil, err
			}
			ids, err := s.Store.ReplaceSource(obj["source"].(string), obj["items"].([]any), now)
			return map[string]any{"ids": ids}, err
		}
	}
	id, err := s.Store.Add(body, now)
	return map[string]any{"id": id}, err
}

func readJSON(r *http.Request) (any, error) {
	data, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, MaxBody))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return nil, store.Refuse("body is over %d bytes", MaxBody)
	}
	if err != nil {
		return nil, err
	}
	body, err := schema.Decode(data)
	if err != nil {
		return nil, store.Refuse("body isn't JSON: %v", err)
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
