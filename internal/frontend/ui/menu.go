package ui

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
)

// menuMessages is what a theme's menu page can post, each action with whether it
// takes an id: "ready", once the page has loaded; "choose", with the index of the item
// chosen; and "close", to close it without choosing.
var menuMessages = map[string]bool{"ready": false, "choose": true, "close": false}

// popup is the theme's menu that's open in MenuWindow, if any: one at a time. What
// can be chosen from it is the menu's Click's to say, which has its items.
type popup struct {
	mu   sync.Mutex
	done chan choice  // where what's chosen goes, or nil with none open
	at   *shim.Anchor // what it opens under, or nil for the mouse
}

// open makes a menu, to open under at, the one that's open, closing any that was
// without a choice, and returns where what's chosen from it will go.
func (p *popup) open(at *shim.Anchor) chan choice {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done != nil {
		p.done <- noChoice
	}
	p.done, p.at = make(chan choice, 1), at
	return p.done
}

// opened is whether a menu is open, and what it opens under.
func (p *popup) opened() (bool, *shim.Anchor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done != nil, p.at
}

// finish closes the open menu with chosen, an item or noChoice, and says whether
// there was one open.
func (p *popup) finish(chosen choice) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil {
		return false
	}
	p.done <- chosen
	p.done = nil
	return true
}

// popupMenu shows items in the theme's menu, styled by css, at the mouse, or under at,
// and returns what was chosen: an item, or -1 if none was: it's closed, a click
// elsewhere dismissed it, or another menu took its place.
func (b *Bar) popupMenu(css string, items []components.MenuItem, at *shim.Anchor) choice {
	var page strings.Builder
	if err := components.MenuPage(css, items).Render(context.Background(), &page); err != nil {
		log.Printf("ui: rendering a menu: %v", err)
		return noChoice
	}
	done := b.popup.open(at)
	b.windows.CreatePopup(MenuWindow)
	b.windows.LoadHTML(MenuWindow, page.String())
	return <-done
}

// measureMenu is the size of the menu page's #menu, as [width, height].
const measureMenu = `(r => [Math.ceil(r.width), Math.ceil(r.height)])(document.getElementById("menu").getBoundingClientRect())`

// menuMessage carries out a message from the theme's menu page: once it's loaded,
// it's measured and shown where it opens, and a choice, or closing it, hides it.
func (b *Bar) menuMessage(_ *page, m message) {
	switch m.Action {
	case "ready":
		open, at := b.popup.opened()
		if !open {
			return
		}
		value, err := b.windows.EvalResult(MenuWindow, measureMenu)
		var size [2]float64
		if err == nil {
			err = json.Unmarshal(value, &size)
		}
		if err != nil || size[0] <= 0 || size[1] <= 0 {
			log.Printf("ui: measuring the menu: %s, %v", value, err)
			b.popup.finish(noChoice)
			return
		}
		b.windows.ShowPopup(MenuWindow, size[0], size[1], at)
	case "choose":
		i, err := strconv.Atoi(m.ID)
		event := m.clickEvent()
		event.Middle = false // a menu's items are chosen with the main button, or a key
		if err != nil || i < 0 || !b.popup.finish(choice{i, event}) {
			log.Printf("ui: the menu chose %q, which isn't an item of an open menu", m.ID)
			return
		}
		b.windows.HidePopup(MenuWindow)
	case "close":
		if b.popup.finish(noChoice) {
			b.windows.HidePopup(MenuWindow)
		}
	}
}
