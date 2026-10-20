package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
)

// message is what the bar page posts: an action, and the id it applies to.
type message struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

// readMessage decodes one message from the page, refusing anything else: an unknown
// action or property, an id with "ready", or none with anything else.
func readMessage(data []byte) (message, error) {
	var m message
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, err
	}
	if decoder.More() {
		return m, errors.New("more than one value")
	}
	switch {
	case !slices.Contains([]string{"ready", "click"}, m.Action):
		return m, fmt.Errorf("no action %q", m.Action)
	case m.Action == "ready" && m.ID != "":
		return m, errors.New("ready takes no id")
	case m.Action != "ready" && m.ID == "":
		return m, fmt.Errorf("%s needs an id", m.Action)
	}
	return m, nil
}

// handle carries out one message from the page.
func (b *Bar) handle(data []byte) {
	m, err := readMessage(data)
	if err != nil {
		log.Printf("ui: the bar sent %s: %v", data, err)
		return
	}
	switch m.Action {
	case "ready":
		b.mu.Lock()
		b.ready = true
		b.shown = map[string]bool{} // a fresh page shows every card arriving
		b.mu.Unlock()
		b.draw()
	case "click":
		b.mu.Lock()
		click := b.clicks[m.ID]
		b.mu.Unlock()
		if click == nil {
			log.Printf("ui: the bar clicked %q, which isn't on it", m.ID)
			return
		}
		click()
	}
}
