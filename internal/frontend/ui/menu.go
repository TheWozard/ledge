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

// popup is the theme's menu that's open in MenuWindow, if any: one at a time.
type popup struct {
	mu      sync.Mutex
	done    chan choice // where what's chosen goes, or nil with none open
	enabled []bool      // which of the open menu's items can be chosen
}

// open makes a menu of items, enabled as given, the one that's open, closing any that
// was without a choice, and returns where what's chosen from it will go.
func (p *popup) open(enabled []bool) chan choice {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done != nil {
		p.done <- noChoice
	}
	p.done, p.enabled = make(chan choice, 1), enabled
	return p.done
}

// isOpen is whether a menu is open.
func (p *popup) isOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done != nil
}

// finish closes the open menu with chosen: an item that can be chosen, or -1 for
// none. It says whether it did: not with no menu open, nor with an item that can't be
// chosen.
func (p *popup) finish(chosen choice) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	i := chosen.index
	if p.done == nil || i >= len(p.enabled) || i >= 0 && !p.enabled[i] {
		return false
	}
	if i < 0 {
		chosen = noChoice
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
	enabled := make([]bool, len(items))
	for i, item := range items {
		enabled[i] = item.Enabled
	}
	done := b.popup.open(enabled)
	b.native.CreatePopup(MenuWindow)
	b.native.LoadHTML(MenuWindow, page.String())
	return <-done
}

// measureMenu is the size of the menu page's #menu, as [width, height].
const measureMenu = `(r => [Math.ceil(r.width), Math.ceil(r.height)])(document.getElementById("menu").getBoundingClientRect())`

// handleMenuPage carries out one message from the theme's menu page: once it's
// loaded, it's measured and shown at the mouse, and a choice, or closing it, hides it.
func (b *Bar) handleMenuPage(data []byte) {
	m, err := readMessage(data, menuMessages)
	if err != nil {
		log.Printf("ui: the menu sent %s: %v", data, err)
		return
	}
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
			log.Printf("ui: the menu chose %q, which can't be chosen", m.ID)
			return
		}
		b.native.HidePopup(MenuWindow)
	case "close":
		if b.popup.finish(noChoice) {
			b.native.HidePopup(MenuWindow)
		}
	}
}
