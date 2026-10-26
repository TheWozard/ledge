// Package store holds the current events by id, in a JSON file in the state dir.
//
// The store does no event management. Every event sent is accepted: one without an
// `id` gets a fresh random id, and one with an `id` replaces that event. Dismissing
// deletes. There's no dedupe, no derived id and no memory of dismissals.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Store is the current events, kept in the order they were first added, which is
// how events received in the same second are ordered.
type Store struct {
	path string

	mu     sync.Mutex
	events events
}

type events struct {
	ids  []string
	byID map[string]Event
}

// Open reads the store at path. A missing file is an empty store.
func Open(path string) (*Store, error) {
	s := &Store{path: path, events: events{byID: map[string]Event{}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.events.load(data); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// Add adds one event, or replaces the event with its id. Returns its id.
func (s *Store) Add(raw any, now time.Time) (string, error) {
	event, err := Normalize(raw, "", now)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.events.clone()
	next.set(event)
	if err := s.save(next); err != nil {
		return "", err
	}
	return event["id"].(string), nil
}

// ReplaceSource makes items the whole of source's current events. Returns their ids.
func (s *Store) ReplaceSource(source string, items []any, now time.Time) ([]string, error) {
	added := make([]Event, len(items))
	ids := make([]string, len(items))
	for i, item := range items {
		event, err := Normalize(item, source, now)
		if err != nil {
			return nil, Refuse("items/%d: %v", i, err)
		}
		added[i] = event
		ids[i] = event["id"].(string)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.events.clone()
	next.removeIf(func(e Event) bool { return e["source"] == source })
	for _, event := range added {
		next.set(event)
	}
	if err := s.save(next); err != nil {
		return nil, err
	}
	return ids, nil
}

// Get is the event with this id.
func (s *Store) Get(id string) (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events.byID[id]
	return event, ok
}

// Remove deletes an event. Returns false if there was no event with that id.
func (s *Store) Remove(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.events.byID[id]; !ok {
		return false, nil
	}
	next := s.events.clone()
	next.removeIf(func(e Event) bool { return e["id"] == id })
	return true, s.save(next)
}

// Prune drops events past their `expires`. Returns true if any went.
func (s *Store) Prune(now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.events.clone()
	next.removeIf(func(e Event) bool {
		text, ok := e["expires"].(string)
		if !ok {
			return false
		}
		expires, err := ParseTime(text, "expires")
		return err == nil && !expires.After(now)
	})
	if len(next.ids) == len(s.events.ids) {
		return false, nil
	}
	return true, s.save(next)
}

// Visible is the current events, highest priority first, then newest first.
func (s *Store) Visible() []Event {
	s.mu.Lock()
	list := make([]Event, len(s.events.ids))
	for i, id := range s.events.ids {
		list[i] = s.events.byID[id]
	}
	s.mu.Unlock()
	received := func(e Event) string { text, _ := e["received"].(string); return text }
	slices.SortStableFunc(list, func(a, b Event) int { return strings.Compare(received(b), received(a)) })
	rank := func(e Event) int {
		if i := slices.Index(Priorities, fmt.Sprint(e["priority"])); i >= 0 {
			return i
		}
		return len(Priorities)
	}
	slices.SortStableFunc(list, func(a, b Event) int { return rank(a) - rank(b) })
	return list
}

// save writes next to disk and, once it's there, makes it the current events.
// Callers hold mu.
func (s *Store) save(next events) error {
	data, err := next.marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.events = next
	return nil
}

func (e events) clone() events {
	byID := make(map[string]Event, len(e.byID))
	for id, event := range e.byID {
		byID[id] = event
	}
	return events{ids: slices.Clone(e.ids), byID: byID}
}

// set adds an event, or replaces the one with its id in place.
func (e *events) set(event Event) {
	id := event["id"].(string)
	if _, ok := e.byID[id]; !ok {
		e.ids = append(e.ids, id)
	}
	e.byID[id] = event
}

func (e *events) removeIf(drop func(Event) bool) {
	e.ids = slices.DeleteFunc(e.ids, func(id string) bool {
		if drop(e.byID[id]) {
			delete(e.byID, id)
			return true
		}
		return false
	})
}

// The file is {"events": {"<id>": {...}, ...}}, in store order.
func (e events) marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n  \"events\": {")
	for i, id := range e.ids {
		if i > 0 {
			buf.WriteString(",")
		}
		key, _ := json.Marshal(id)
		value, err := marshalIndent(e.byID[id], "    ")
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&buf, "\n    %s: %s", key, value)
	}
	if len(e.ids) > 0 {
		buf.WriteString("\n  ")
	}
	buf.WriteString("}\n}\n")
	return buf.Bytes(), nil
}

func (e *events) load(data []byte) error {
	var file struct {
		Events json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	if len(file.Events) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(file.Events))
	dec.UseNumber()
	if token, err := dec.Token(); err != nil || token != json.Delim('{') {
		return errors.New(`"events" isn't an object`)
	}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		var event Event
		if err := dec.Decode(&event); err != nil {
			return err
		}
		if event == nil {
			return fmt.Errorf("event %q isn't an object", token)
		}
		event["id"] = token.(string)
		e.set(event)
	}
	return nil
}

// MarshalIndent encodes JSON the way ledge prints it: indented by two spaces,
// without escaping <, > and &.
func MarshalIndent(value any) ([]byte, error) {
	return marshalIndent(value, "")
}

func marshalIndent(value any, prefix string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
