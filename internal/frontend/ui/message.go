package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
)

// message is what a page posts: an action, and the id it applies to.
type message struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

// barMessages is what the bar page can post, each action with whether it takes an
// id: "ready", once the page has loaded; "click", with the id of a Click; and "menu",
// with the element id of a card whose menu of actions to show.
var barMessages = map[string]bool{"ready": false, "click": true, "menu": true}

// readMessage decodes one message from a page, refusing anything else: an action
// that isn't one of actions, an unknown property, or an id with an action that takes
// none, or none with one that does.
func readMessage(data []byte, actions map[string]bool) (message, error) {
	var m message
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, err
	}
	if decoder.More() {
		return m, errors.New("more than one value")
	}
	takesID, ok := actions[m.Action]
	switch {
	case !ok:
		return m, fmt.Errorf("no action %q", m.Action)
	case !takesID && m.ID != "":
		return m, fmt.Errorf("%s takes no id", m.Action)
	case takesID && m.ID == "":
		return m, fmt.Errorf("%s needs an id", m.Action)
	}
	return m, nil
}

// handle carries out one message from the page.
func (b *Bar) handle(data []byte) {
	m, err := readMessage(data, barMessages)
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
		b.run(BarWindow, m.ID, m.ID, click)
	case "menu":
		b.menu(m.ID)
	}
}

// chooseFrom pops up a menu of items, in the theme's menu if it has one, or the
// native one, and returns the index of the one chosen, or -1 if none was.
func (b *Bar) chooseFrom(items []components.MenuItem) int {
	b.mu.Lock()
	css := b.options.Theme.Menu
	b.mu.Unlock()
	if css != "" {
		return b.popupMenu(css, items)
	}
	shown := make([]shim.MenuItem, len(items))
	for i, item := range items {
		shown[i] = shim.MenuItem{Title: item.Title, Color: item.Color, Enabled: item.Enabled}
	}
	return showMenu(shown)
}

// menuBusyID is the id the page marks busy while an action of the card whose element
// id is id runs: its ⋯. It's apart from every click id, so it's never the card's own.
func menuBusyID(id string) string { return "menu-" + id }

// showMenu pops up a native menu and returns the index of the item chosen, or -1:
// the shim's, or a test's in its place.
var showMenu = shim.ShowMenu

// menu shows the menu of the card whose element id is id, as its theme draws it or
// natively for a theme without one, and calls the Click of the action chosen from it,
// its ⋯ busy while it runs. The card's actions are those it had when it was asked for;
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
	chosen := b.chooseFrom(items)
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
	b.run(BarWindow, items[chosen].ClickID, menuBusyID(id), click)
}
