// Package api is ledge's JSON API on 127.0.0.1, and a client for it.
//
//	POST /event          one event, or a section ({"source", "items"}) that
//	                     replaces a source's events; replies with the id(s)
//	POST /dismiss        {"id": ...}, deleting that event
//	POST /shutdown       {}, stopping ledge once it has replied
//	GET  /events         the current events, as the bar shows them
//	GET  /schemas/{name} a schema, as built in
//
// Requests must carry no Origin header, and POSTs must be application/json. A web
// page can't send that to localhost without a CORS preflight, which this server never
// answers, so a site you visit can't post events.
package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/TheWozard/ledge/internal/schema"
	"github.com/TheWozard/ledge/internal/store"
)

// MaxBody is the largest request body accepted.
const MaxBody = 1 << 20

// Server serves the API for one store. Changed, if set, is called after any request
// that changes the store, and Shutdown after replying to POST /shutdown.
type Server struct {
	Store    *store.Store
	Changed  func()
	Shutdown func()
}

// Listen binds the API port on 127.0.0.1 only.
func Listen(port int) (net.Listener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("can't listen on %s (%w); is ledge already running?", addr, err)
	}
	return listener, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" {
		reply(w, http.StatusBadRequest, map[string]any{"error": "requests from a browser page aren't accepted"})
		return
	}
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
	if media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); media != "application/json" {
		return nil, store.Refuse("Content-Type must be application/json")
	}
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
