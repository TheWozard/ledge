// Package bar connects the store to the bar from ledge-frontend: it turns events into
// the cards and reports the frontend shows, and carries out what clicking them asks
// for, by id: dismissing an event, opening its report or url, running its command.
package bar

import (
	"log"
	"os/exec"
	"time"

	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/TheWozard/ledge-frontend/ui"

	"github.com/TheWozard/ledge/internal/config"
	"github.com/TheWozard/ledge/internal/store"
)

// Bar is the bar and report windows, and the store they show.
type Bar struct {
	store    *store.Store
	stateDir string // where commands.log goes
	ui       *ui.Bar

	// start runs a command in the background: `open` for a url, or an event's
	// command. Tests replace it.
	start func(*exec.Cmd) error
}

// Open creates the bar, places it and loads its page. Call it after ui.Init and
// before ui.Run. Events' commands' output goes to commands.log in stateDir.
func Open(events *store.Store, cfg config.Config, stateDir string) (*Bar, error) {
	b := &Bar{store: events, stateDir: stateDir, start: startDetached}
	th, err := theme.Builtin(cfg.Theme)
	if err != nil {
		return nil, err
	}
	u, err := ui.Open(ui.Options{Screen: cfg.Screen, Edge: cfg.Edge, Height: cfg.Height, Level: cfg.Level, Theme: th})
	if err != nil {
		return nil, err
	}
	b.ui = u
	b.Render()
	return b, nil
}

// Render shows the store's current events. Safe from any goroutine.
func (b *Bar) Render() {
	if err := b.ui.Render(b.view(b.store.Visible(), time.Now())); err != nil {
		log.Printf("ledge: rendering the bar: %v", err)
	}
}

// dismiss removes the event with this id, as its card's × asks.
func (b *Bar) dismiss(id string) {
	if _, err := b.store.Remove(id); err != nil {
		log.Printf("ledge: dismissing %s: %v", id, err)
	}
	b.Render()
}
