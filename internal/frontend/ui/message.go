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

// message is what a page posts: an action, the id it applies to, and for a click or a
// choice from a menu, how it was made.
type message struct {
	Action string `json:"action"`
	ID     string `json:"id"`
	Event  *event `json:"event"`
}

// event is how something was clicked or chosen, as a page posts it: the modifier keys
// held, and whether with the middle mouse button.
type event struct {
	Alt    bool `json:"alt"`
	Cmd    bool `json:"cmd"`
	Shift  bool `json:"shift"`
	Ctrl   bool `json:"ctrl"`
	Middle bool `json:"middle"`
}

// eventActions is the actions a message can have an event with.
var eventActions = map[string]bool{"click": true, "choose": true}

// clickEvent is how the message's click was made, or none for a message without.
func (m message) clickEvent() components.ClickEvent {
	if m.Event == nil {
		return components.ClickEvent{}
	}
	return components.ClickEvent(*m.Event)
}

// modifiers is the modifier keys held as the message's choice was made.
func (m message) modifiers() shim.Modifiers {
	e := m.clickEvent()
	return shim.Modifiers{Alt: e.Alt, Cmd: e.Cmd, Shift: e.Shift, Ctrl: e.Ctrl}
}

// barMessages is what the bar page can post, each action with whether it takes an
// id: "ready", once the page has loaded; "click", with the id of a Click; and "menu",
// with the element id of a card whose menu of actions to show.
var barMessages = map[string]bool{"ready": false, "click": true, "menu": true}

// readMessage decodes one message from a page, refusing anything else: an action
// that isn't one of actions, an unknown property, an id with an action that takes
// none, or none with one that does, or an event with an action that isn't a click or
// a choice.
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
	case m.Event != nil && !eventActions[m.Action]:
		return m, fmt.Errorf("%s takes no event", m.Action)
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
		click := b.clicks.Click(m.ID)
		b.mu.Unlock()
		if click == nil {
			log.Printf("ui: the bar clicked %q, which isn't on it", m.ID)
			return
		}
		b.run(BarWindow, m.ID, m.ID, click, m.clickEvent())
	case "menu":
		b.menu(m.ID)
	}
}

// chooseFrom pops up a menu of items, in the theme's menu if it has one, or the
// native one, and returns what was chosen: the index of an item, or -1 for none, and
// the keys held as it was.
func (b *Bar) chooseFrom(items []components.MenuItem) shim.Chosen {
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

// showMenu pops up a native menu and returns what was chosen from it: the shim's, or
// a test's in its place.
var showMenu = shim.ShowMenu

// menu shows the menu of the card whose element id is id, as its theme draws it or
// natively for a theme without one, and calls the Click of the action chosen from it,
// its ⋯ busy while it runs. The card's actions are those it had when it was asked for;
// one a render has since taken off the bar is dropped, as a click on it would be, and
// one a render has changed calls its new Click.
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
	choice := b.chooseFrom(items)
	chosen := choice.Index
	if chosen < 0 || chosen >= len(items) {
		return
	}
	item := items[chosen].ID
	b.mu.Lock()
	var click components.Click
	for _, card := range b.cards {
		if frame := card.Frame(); frame.ElementID() == id {
			click = frame.Action(item)
		}
	}
	b.mu.Unlock()
	if click == nil {
		log.Printf("ui: %q was chosen from a menu, but isn't on the bar", item)
		return
	}
	m := choice.Modifiers
	b.run(BarWindow, item, menuBusyID(id), click,
		components.ClickEvent{Alt: m.Alt, Cmd: m.Cmd, Shift: m.Shift, Ctrl: m.Ctrl})
}
