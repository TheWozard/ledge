// Package bar connects the store to the bar from ledge-frontend: it turns events into
// the cards and reports the frontend shows, and carries out what clicking them asks
// for, by id: dismissing an event, opening its report or url, running its command,
// or running a report's command and showing its output as it comes.
package bar

import (
	"log"
	"os/exec"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/TheWozard/ledge-frontend/ui"

	"github.com/TheWozard/ledge/internal/config"
	"github.com/TheWozard/ledge/internal/store"
)

// Bar is the bar and report windows, and the store they show.
type Bar struct {
	store    *store.Store
	stateDir string // where commands.log goes
	commands bool   // whether events' commands run
	ui       *ui.Bar
	runs     runs // the last run of each event's run blocks

	// start runs a command in the background: `open` for a url, or an event's
	// command. Tests replace it.
	start func(*exec.Cmd) error
	// showBlock replaces a block in an event's report, if it's open: a run's, as its
	// output comes. Tests replace it.
	showBlock func(id string, block components.Block) (bool, error)
}

// Open creates the bar, places it and loads its page. Call it after ui.Init and
// before ui.Run. Events' commands run only if cfg allows them, with their output in
// commands.log in stateDir.
func Open(events *store.Store, cfg config.Config, stateDir string) (*Bar, error) {
	b := &Bar{store: events, stateDir: stateDir, commands: cfg.Commands, start: startDetached}
	th, err := theme.Builtin(cfg.Theme)
	if err != nil {
		return nil, err
	}
	u, err := ui.Open(ui.Options{Screen: cfg.Screen, Edge: cfg.Edge, Height: cfg.Height, Level: cfg.Level, Theme: th})
	if err != nil {
		return nil, err
	}
	b.ui, b.showBlock = u, u.ShowReportBlock
	b.Render()
	return b, nil
}

// Render shows the store's current events. Safe from any goroutine.
func (b *Bar) Render() {
	if err := b.ui.Render(b.view(b.store.Visible(), time.Now())); err != nil {
		log.Printf("rendering the bar: %v", err)
	}
}

// dismiss removes the event with this id, as its card's × asks.
func (b *Bar) dismiss(id string) {
	if _, err := b.store.Remove(id); err != nil {
		log.Printf("dismissing %s: %v", id, err)
	}
	b.Render()
}
