package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
)

// message is what the bar page posts: an action, and the id it applies to. It's
// "ready", once the page has loaded; "click", with the id of a Click; or "menu", with
// the element id of a card whose menu of actions to show.
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
	case !slices.Contains([]string{"ready", "click", "menu"}, m.Action):
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
	case "menu":
		b.menu(m.ID)
	}
}

// showMenu pops up a menu and returns the index of the item chosen, or -1: the
// shim's, or a test's in its place.
var showMenu = shim.ShowMenu

// menu shows the menu of the card whose element id is id, and calls the Click of the
// action chosen from it. The card's actions are those it had when it was asked for;
// one a render has since taken off the bar is dropped, as a click on it would be.
func (b *Bar) menu(id string) {
	b.mu.Lock()
	var items []components.MenuItem
	for _, card := range b.cards {
		if frame := card.Frame(); frame.ElementID() == id {
			items = frame.Menu()
		}
	}
	b.mu.Unlock()
	if len(items) == 0 {
		log.Printf("ui: the bar asked for the menu of %q, which has none", id)
		return
	}
	shown := make([]shim.MenuItem, len(items))
	for i, item := range items {
		shown[i] = shim.MenuItem{Title: item.Title, Color: item.Color, Enabled: item.Enabled}
	}
	chosen := showMenu(shown)
	if chosen < 0 || chosen >= len(items) {
		return
	}
	b.mu.Lock()
	click := b.clicks[items[chosen].ClickID]
	b.mu.Unlock()
	if click == nil {
		log.Printf("ui: %q was chosen from a menu, but isn't on the bar", items[chosen].ClickID)
		return
	}
	click()
}
