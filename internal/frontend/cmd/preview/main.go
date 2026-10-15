// Command preview opens a bar and a report window in a theme, with one of everything
// on them, and keeps them open until it's interrupted. Clicks are logged; opening a
// card shows its report again, and dismissing one takes it off the bar.
//
//	go run ./cmd/preview [theme]
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/TheWozard/ledge-frontend/ui"
)

func main() {
	name := theme.Default
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	if !slices.Contains(theme.Names, name) {
		fmt.Fprintf(os.Stderr, "no theme %q; the themes are %s\n", name, strings.Join(theme.Names, ", "))
		os.Exit(2)
	}

	now := time.Now()
	pills := []components.Pill{
		{ID: "aws", Title: "AWS", Color: "green", Detail: "Signed in", Command: "aws sso login",
			Schedule: []components.Step{{At: now.Add(30 * time.Second), Color: "yellow"}, {At: now.Add(time.Minute), Color: "red"}}},
		{ID: "vpn", Title: "VPN", Color: "blue", Link: true},
		{ID: "disk", Title: "Disk", Color: "gray", Detail: "82% used"},
		{ID: "build", Title: "Build", Color: "#c060ff"},
	}
	var mu sync.Mutex
	cards := []components.Card{
		{ID: "ci", Title: "CI failed on main", Description: "2 jobs failed: test, lint", Priority: "high",
			Tags: []string{"ci", "main"}, Opens: true, Hint: "https://example.com/runs/1", Source: "ci", Received: now.Add(-3 * time.Minute)},
		{ID: "review", Title: "Review requested", Description: "Add the preview command", Priority: "medium",
			Tags: []string{"pr"}, Opens: true, Source: "github", Received: now.Add(-40 * time.Minute)},
		{ID: "deploy", Title: "Deploy finished", Priority: "low", Source: "deploy", Received: now.Add(-5 * time.Hour)},
		{ID: "note", Title: "A note", Description: strings.Repeat("A long description that runs on. ", 12),
			Source: "manual", Received: now.Add(-2 * 24 * time.Hour)},
	}
	report := []templ.Component{
		components.Heading("Summary"),
		components.Note("Two jobs failed.", "red"),
		components.Paragraph("The run on main failed after a merge.\nThe second line keeps its break."),
		components.Fields([]components.Field{{Label: "Branch", Value: "main"}, {Label: "Commit", Value: "465882e"}}),
		components.Table([]string{"Job", "Result", "Time"}, [][]string{{"build", "ok", "1m"}, {"test", "failed", "3m"}, {"lint", "failed", "20s"}}),
		components.List([]string{"Fix the test", "Fix the lint", "Run it again"}, true),
		components.Code("$ go test ./...\n--- FAIL: TestRender (0.21s)\n    ui_test.go:218: a very long line that scrolls sideways rather than wrapping in the report window"),
		components.Link("The run", "https://example.com/runs/1"),
	}

	ui.Init()
	var bar *ui.Bar
	bar, err := ui.Open(
		ui.Options{Edge: "bottom", Height: 110, Level: "normal", Theme: name},
		ui.Handlers{
			Dismiss: func(id string) {
				log.Printf("dismiss %s", id)
				mu.Lock()
				cards = slices.DeleteFunc(cards, func(c components.Card) bool { return c.ID == id })
				shown := slices.Clone(cards)
				mu.Unlock()
				bar.Render(pills, shown)
			},
			Open: func(id string) {
				log.Printf("open %s", id)
				if err := bar.ShowReport(id, report); err != nil {
					log.Print(err)
				}
			},
			Run:     func(id string) { log.Printf("run %s", id) },
			OpenURL: func(url string) { log.Printf("url %s", url) },
		})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		bar.Render(pills, cards)
		if err := bar.ShowReport("CI failed on main", report); err != nil {
			log.Print(err)
		}
		log.Printf("showing %s; ctrl-c to quit", name)
	}()
	go func() {
		interrupted := make(chan os.Signal, 1)
		signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM)
		<-interrupted
		ui.Stop()
	}()
	ui.Run()
}
