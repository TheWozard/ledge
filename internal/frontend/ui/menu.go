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

// popup is the theme's menu that's open in MenuWindow, if any: one at a time.
type popup struct {
	mu      sync.Mutex
	done    chan int // where what's chosen goes, or nil with none open
	enabled []bool   // which of the open menu's items can be chosen
}

// open makes a menu of items, enabled as given, the one that's open, closing any that
// was without a choice, and returns where what's chosen from it will go.
func (p *popup) open(enabled []bool) chan int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done != nil {
		p.done <- -1
	}
	p.done, p.enabled = make(chan int, 1), enabled
	return p.done
}

// isOpen is whether a menu is open.
func (p *popup) isOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done != nil
}

// finish closes the open menu with chosen, the index of an item that can be chosen,
// or -1 for none, and says whether it did: not with no menu open, nor with an item
// that can't be chosen.
func (p *popup) finish(chosen int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil || chosen >= len(p.enabled) || chosen >= 0 && !p.enabled[chosen] {
		return false
	}
	p.done <- max(chosen, -1)
	p.done = nil
	return true
}

// popupMenu shows items in the theme's menu, styled by css, at the mouse, and returns
// the index of the one chosen, or -1 if none was: it's closed, a click elsewhere
// dismissed it, or another menu took its place.
func (b *Bar) popupMenu(css string, items []components.MenuItem) int {
	var page strings.Builder
	if err := components.MenuPage(css, items).Render(context.Background(), &page); err != nil {
		log.Printf("ui: rendering a menu: %v", err)
		return -1
	}
	enabled := make([]bool, len(items))
	for i, item := range items {
		enabled[i] = item.Enabled
	}
	done := b.popup.open(enabled)
	shim.CreatePopup(MenuWindow)
	shim.LoadHTML(MenuWindow, page.String())
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
		value, err := shim.EvalResult(MenuWindow, measureMenu)
		var size [2]float64
		if err == nil {
			err = json.Unmarshal(value, &size)
		}
		if err != nil || size[0] <= 0 || size[1] <= 0 {
			log.Printf("ui: measuring the menu: %s, %v", value, err)
			b.popup.finish(-1)
			return
		}
		shim.ShowPopup(MenuWindow, size[0], size[1])
	case "choose":
		chosen, err := strconv.Atoi(m.ID)
		if err != nil || chosen < 0 || !b.popup.finish(chosen) {
			log.Printf("ui: the menu chose %q, which can't be chosen", m.ID)
			return
		}
		shim.HidePopup(MenuWindow)
	case "close":
		if b.popup.finish(-1) {
			shim.HidePopup(MenuWindow)
		}
	}
}
