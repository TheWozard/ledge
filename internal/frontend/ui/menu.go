package ui

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/TheWozard/ledge-frontend/components"
)

// menuMessages is what a theme's menu page can post, each action with whether it
// takes an id: "ready", once the page has loaded; "choose", with the index of the item
// chosen; and "close", to close it without choosing.
var menuMessages = map[string]bool{"ready": false, "choose": true, "close": false}

// popup is the theme's menu that's open in MenuWindow, if any: one at a time. What
// can be chosen from it is the menu's Click's to say, which has its items.
type popup struct {
	mu   sync.Mutex
	done chan choice // where what's chosen goes, or nil with none open
}

// open makes a menu the one that's open, closing any that was without a choice, and
// returns where what's chosen from it will go.
func (p *popup) open() chan choice {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done != nil {
		p.done <- noChoice
	}
	p.done = make(chan choice, 1)
	return p.done
}

// isOpen is whether a menu is open.
func (p *popup) isOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done != nil
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

// popupMenu shows items in the theme's menu, styled by css, at the mouse, and returns
// what was chosen: an item, or -1 if none was: it's closed, a click elsewhere
// dismissed it, or another menu took its place.
func (b *Bar) popupMenu(css string, items []components.MenuItem) choice {
	var page strings.Builder
	if err := components.MenuPage(css, items).Render(context.Background(), &page); err != nil {
		log.Printf("ui: rendering a menu: %v", err)
		return noChoice
	}
	done := b.popup.open()
	b.native.CreatePopup(MenuWindow)
	b.native.LoadHTML(MenuWindow, page.String())
	return <-done
}

// measureMenu is the size of the menu page's #menu, as [width, height].
const measureMenu = `(r => [Math.ceil(r.width), Math.ceil(r.height)])(document.getElementById("menu").getBoundingClientRect())`

// menuMessage carries out a message from the theme's menu page: once it's loaded,
// it's measured and shown at the mouse, and a choice, or closing it, hides it.
func (b *Bar) menuMessage(_ *page, m message) {
	switch m.Action {
	case "ready":
		if !b.popup.isOpen() {
			return
		}
		value, err := b.native.EvalResult(MenuWindow, measureMenu)
		var size [2]float64
		if err == nil {
			err = json.Unmarshal(value, &size)
		}
		if err != nil || size[0] <= 0 || size[1] <= 0 {
			log.Printf("ui: measuring the menu: %s, %v", value, err)
			b.popup.finish(noChoice)
			return
		}
		b.native.ShowPopup(MenuWindow, size[0], size[1])
	case "choose":
		i, err := strconv.Atoi(m.ID)
		event := m.clickEvent()
		event.Middle = false // a menu's items are chosen with the main button, or a key
		if err != nil || i < 0 || !b.popup.finish(choice{i, event}) {
			log.Printf("ui: the menu chose %q, which isn't an item of an open menu", m.ID)
			return
		}
		b.native.HidePopup(MenuWindow)
	case "close":
		if b.popup.finish(noChoice) {
			b.native.HidePopup(MenuWindow)
		}
	}
}
