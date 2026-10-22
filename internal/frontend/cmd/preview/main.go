// Command preview opens a bar and a report window in a theme, with one of everything
// on them and enough that the row of cards and the report both scroll, and keeps them
// open until it's interrupted. Clicks are logged; opening a card or an action's
// link shows its report in its own window, and dismissing a card takes it off the bar.
//
//	go run ./cmd/preview [-browser] [-addr host:port] [theme]
//
// theme is a built-in theme's name, or the path to a theme's json manifest of your
// own, such as ~/themes/neon.json, loaded as theme.Load does.
//
// With -browser, it serves the same pages on addr and opens them in the browser
// instead, for its inspector; see browserServer. Run from the repository, a built-in
// theme is read from theme/themes rather than what's built in, so an edit to it
// shows on a reload.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/TheWozard/ledge-frontend/ui"
)

// barHeight is the bar's height, in points.
const barHeight = 110

func main() {
	browser := flag.Bool("browser", false, "serve the pages and open them in the browser, rather than in windows")
	addr := flag.String("addr", "localhost:7337", "with -browser, where to serve the pages")
	flag.Parse()
	name := theme.Builtins.Default()
	if flag.NArg() > 0 {
		name = flag.Arg(0)
	}
	load := func() (theme.Theme, error) { return loadTheme(name, *browser) }
	th, err := load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	sample := newSample(time.Now())
	if *browser {
		serveBrowser(*addr, name, load, sample)
		return
	}
	showWindows(name, th, sample)
}

// showWindows shows sample on a bar and the long report in a window, in th, until
// it's interrupted.
func showWindows(name string, th theme.Theme, sample *sample) {
	ui.Init()
	bar, err := ui.Open(ui.Options{Edge: "bottom", Height: barHeight, Level: "normal", Theme: th})
	if err != nil {
		log.Fatal(err)
	}
	sample.open = func(id string) {
		title, blocks := sample.report(id)
		if err := bar.ShowReport(id, title, blocks); err != nil {
			log.Print(err)
		}
	}
	sample.replace = func(id, blockID string, block templ.Component) {
		if _, err := bar.ShowReportBlock(id, blockID, block); err != nil {
			log.Print(err)
		}
	}
	sample.changed = func() { bar.Render(sample.shown()) }
	go sample.advance()

	go func() {
		bar.Render(sample.shown())
		sample.open(longID)
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
