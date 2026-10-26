// Package store holds the current events, in a JSON file in the state dir.
//
// The store does no event management. Every event sent is accepted: one without an
// `id` gets a fresh random id, and one with an `id` replaces that event. Dismissing
// deletes. There's no dedupe, no derived id and no memory of dismissals. An id belongs
// to the source that sent it, so one source can't replace another's events.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// fileVersion is the events file's format. A file of another version is set aside
// rather than read: events are fleeting, so old formats aren't carried forward.
const fileVersion = 1

// file is the events file: the events in store order.
type file struct {
	Version int     `json:"version"`
	Events  []Event `json:"events"`
}

// Store is the current events, kept in the order they were first added, which is
// how events received in the same second are ordered.
type Store struct {
	path string

	mu     sync.Mutex
	events []Event
}

// Open reads the store at path. A missing file is an empty store, and so is one that
// can't be read as this version's: it's kept beside it, as <path>.bad.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil || f.Version != fileVersion {
		if err == nil {
			err = fmt.Errorf("it's version %d, not %d", f.Version, fileVersion)
		}
		log.Printf("starting with no events, as %s can't be read (%v); it's kept as %s.bad", path, err, path)
		return s, os.Rename(path, path+".bad")
	}
	s.events = f.Events
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
	if err := s.claim(event); err != nil {
		return "", err
	}
	if err := s.save(set(slices.Clone(s.events), event)); err != nil {
		return "", err
	}
	return event.ID, nil
}

// ReplaceSource makes items the whole of source's current events. Returns their ids.
func (s *Store) ReplaceSource(source string, items []any, now time.Time) ([]string, error) {
	added := make([]Event, len(items))
	ids := make([]string, len(items))
	for i, item := range items {
		event, err := Normalize(item, source, now)
		if err != nil {
			return nil, refuse("items/%d: %v", i, err)
		}
		if j := slices.Index(ids[:i], event.ID); j >= 0 {
			return nil, refuse("items/%d: id %q is items/%d's too", i, event.ID, j)
		}
		added[i] = event
		ids[i] = event.ID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, event := range added {
		if err := s.claim(event); err != nil {
			return nil, refuse("items/%d: %v", i, err)
		}
	}
	next := slices.DeleteFunc(slices.Clone(s.events), func(e Event) bool { return e.Source == source })
	if err := s.save(append(next, added...)); err != nil {
		return nil, err
	}
	return ids, nil
}

// claim refuses event if its id is another source's event. Callers hold mu.
func (s *Store) claim(event Event) error {
	if old, ok := s.find(event.ID); ok && old.Source != event.Source {
		return refuse("id %q is source %q's event; give it another id", event.ID, old.Source)
	}
	return nil
}

// Get is the event with this id.
func (s *Store) Get(id string) (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.find(id)
}

// find is the event with this id. Callers hold mu.
func (s *Store) find(id string) (Event, bool) {
	if i := slices.IndexFunc(s.events, func(e Event) bool { return e.ID == id }); i >= 0 {
		return s.events[i], true
	}
	return Event{}, false
}

// Remove deletes an event. Returns false if there was no event with that id.
func (s *Store) Remove(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := slices.DeleteFunc(slices.Clone(s.events), func(e Event) bool { return e.ID == id })
	if len(next) == len(s.events) {
		return false, nil
	}
	return true, s.save(next)
}

// Clear removes every event. Returns how many there were.
func (s *Store) Clear() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		return 0, nil
	}
	return len(s.events), s.save(nil)
}

// Prune drops events past their `expires`. Returns true if any went.
func (s *Store) Prune(now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := slices.DeleteFunc(slices.Clone(s.events), func(e Event) bool {
		return !e.Expires.IsZero() && !e.Expires.After(now)
	})
	if len(next) == len(s.events) {
		return false, nil
	}
	return true, s.save(next)
}

// Visible is the current events, highest priority first, then newest first.
func (s *Store) Visible() []Event {
	s.mu.Lock()
	list := slices.Clone(s.events)
	s.mu.Unlock()
	slices.SortStableFunc(list, func(a, b Event) int { return b.Received.Compare(a.Received) })
	rank := func(e Event) int {
		if i := slices.Index(Priorities, e.Priority); i >= 0 {
			return i
		}
		return len(Priorities)
	}
	slices.SortStableFunc(list, func(a, b Event) int { return rank(a) - rank(b) })
	return list
}

// save writes next to disk and, once it's there, makes it the current events.
// Callers hold mu.
func (s *Store) save(next []Event) error {
	if next == nil {
		next = []Event{}
	}
	data, err := MarshalIndent(file{Version: fileVersion, Events: next})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.events = next
	return nil
}

// set adds event to events, or replaces the one with its id in place.
func set(events []Event, event Event) []Event {
	if i := slices.IndexFunc(events, func(e Event) bool { return e.ID == event.ID }); i >= 0 {
		events[i] = event
		return events
	}
	return append(events, event)
}

// MarshalIndent encodes JSON the way ledge prints it: indented by two spaces,
// without escaping <, > and &.
func MarshalIndent(value any) ([]byte, error) {
	return encode(value, "  ")
}

// marshal is json.Marshal without escaping <, > and &.
func marshal(value any) ([]byte, error) {
	return encode(value, "")
}

func encode(value any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
