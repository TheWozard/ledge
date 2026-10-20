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
// instead, for its inspector; see serveBrowser. Run from the repository, a built-in
// theme is read from theme/themes rather than what's built in, so an edit to it
// shows on a reload.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/TheWozard/ledge-frontend/ui"
)

// barHeight is the bar's height, in points.
const barHeight = 110

// builtinDir is where the built-in themes are, from the repository's root.
const builtinDir = "theme/themes"

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

	now := time.Now()
	lorem := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
		"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. " +
		"Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur."
	sample := &sample{}
	sample.cards = []components.Basic{
		{ID: "lorem", Title: "Lorem ipsum dolor sit amet", Description: lorem, Priority: 4, Color: "#14b8a6"},
		{ID: "lorem-actions", Title: "Lorem ipsum with actions", Description: lorem, Priority: 4, Color: "#ec4899", Width: 420, Click: sample.opens("lorem-actions"),
			Actions: []components.Action{
				{ID: "lorem-1", Title: "Lorem", Color: "green", Click: sample.runs("lorem-1")},
				{ID: "lorem-2", Title: "Ipsum", Color: "yellow", Click: sample.opens("lorem-2")},
				{ID: "lorem-3", Title: "Dolor", Color: "red", Click: sample.runs("lorem-3")},
				{ID: "lorem-4", Title: "Sit amet", Color: "blue", Click: sample.opens("lorem-4")},
				{ID: "lorem-5", Title: "Consectetur", Color: "gray"},
				{ID: "lorem-6", Title: "Adipiscing", Color: "#14b8a6", Click: sample.runs("lorem-6")},
				{ID: "lorem-7", Title: "Elit", Color: "#ec4899", Click: sample.opens("lorem-7")},
				{ID: "lorem-8", Title: "Sed do eiusmod", Color: "#8b5cf6", Click: sample.runs("lorem-8")},
			}},
		{ID: "report", Title: "Lorem ipsum dolor", Description: "Consectetur adipiscing elit, sed do eiusmod.", Priority: 3, Color: "#e0362c",
			Click: sample.opens("report"),
			Actions: []components.Action{
				{ID: "tempor", Title: "Tempor", Color: "yellow", Click: sample.runs("tempor")},
				{ID: "incididunt", Title: "Incididunt", Color: "gray", Click: sample.opens("incididunt")},
			}},
		{ID: "magna", Title: "Magna aliqua", Description: "Ad minim veniam, quis nostrud.", Priority: 2, Color: "#e08a00", Click: sample.opens("magna"),
			Actions: []components.Action{{ID: "exercitation", Title: "Exercitation", Color: "green", Click: sample.runs("exercitation")}}},
		{ID: "ullamco", Title: "Ullamco laboris", Priority: 1, Color: "#2f9e5b",
			Actions: []components.Action{{ID: "nisi", Title: "Nisi ut aliquip", Color: "#c060ff"}}},
		{ID: "long", Title: "Ex ea commodo", Description: strings.Repeat("Consequat duis aute irure dolor in reprehenderit. ", 12)},
		{ID: "voluptate", Title: "In voluptate velit esse cillum dolore eu fugiat nulla", Description: "Excepteur sint occaecat cupidatat non proident, sunt in culpa.",
			Priority: 3, Color: "#e0362c", Click: sample.opens("voluptate"),
			Actions: []components.Action{
				{ID: "officia", Title: "Officia", Color: "red", Click: sample.runs("officia")},
				{ID: "deserunt", Title: "Deserunt", Color: "blue", Click: sample.opens("deserunt")},
				{ID: "mollit", Title: "Mollit anim", Color: "yellow", Click: sample.runs("mollit")},
			}},
		{ID: "laborum", Title: "Id est laborum", Priority: 1, Color: "#2f9e5b", Click: sample.opens("laborum")},
		{ID: "perspiciatis", Title: "Sed ut perspiciatis", Description: "Unde omnis iste natus error sit voluptatem.", Priority: 2, Color: "#e08a00",
			Actions: []components.Action{{ID: "accusantium", Title: "Accusantium", Color: "green", Click: sample.runs("accusantium")}}},
		{ID: "doloremque", Title: "Doloremque laudantium", Description: "Totam rem aperiam, eaque ipsa quae ab illo.", Click: sample.opens("doloremque")},
		{ID: "inventore", Title: "Inventore veritatis", Priority: 1, Color: "#2f9e5b"},
		{ID: "quasi", Title: "Quasi architecto", Description: "Beatae vitae dicta sunt explicabo.", Priority: 2, Color: "#e08a00",
			Actions: []components.Action{{ID: "nemo", Title: "Nemo enim", Color: "gray", Click: sample.runs("nemo")}}},
		{ID: "aspernatur", Title: "Aut odit aut fugit", Color: "#3b82f6", Click: sample.opens("aspernatur"),
			Actions: []components.Action{{ID: "consequuntur", Title: "Consequuntur", Color: "green", Click: sample.opens("consequuntur")}}},
		{ID: "neque", Title: "Neque porro quisquam"},
		{ID: "dolorem", Title: "Est qui dolorem ipsum", Description: "Quia dolor sit amet, consectetur, adipisci velit."},
		{ID: "numquam", Title: "Sed quia non numquam", Color: "#8b5cf6", Description: "Eius modi tempora incidunt", Click: sample.opens("numquam")},
	}
	for i, c := range sample.cards {
		sample.cards[i].Dismiss = sample.dismisses(c.ID)
	}
	sample.timers = []components.Timer{
		{ID: "timer", Title: "Lorem ipsum timer", Start: now.Add(-40 * time.Second), End: now.Add(80 * time.Second), Priority: 5, Color: "#3b82f6",
			Click: sample.opens("timer"), Dismiss: sample.dismisses("timer"), Complete: sample.completes("timer")},
		{ID: "timer-short", Title: "Dolor sit amet", Start: now, End: now.Add(25 * time.Second), Priority: 5,
			Click: sample.opens("timer-short"), Dismiss: sample.dismisses("timer-short"), Complete: sample.completes("timer-short")},
	}
	// A log and a run history long enough to scroll, the log sideways too.
	var failureLog []string
	for i := range 30 {
		failureLog = append(failureLog, fmt.Sprintf("    ui_test.go:%d: step %02d: got %q, want %q", 200+i, i, "rgba(0, 0, 0, 0)", "rgb(192, 192, 192)"))
	}
	var history [][]string
	for i := range 20 {
		result := "ok"
		if i%4 == 0 || i == 1 {
			result = "failed"
		}
		history = append(history, []string{fmt.Sprintf("#%d", 128-i), fmt.Sprintf("%07x", 0x465882e+i*0x1f3a), result,
			fmt.Sprintf("%dm %02ds", 3+i%3, (i*17)%60), []string{"dan", "sam", "alex"}[i%3]})
	}
	sample.long = []templ.Component{
		components.Heading("Summary"),
		components.Note("Two jobs failed.", "red"),
		components.Paragraph("The run on main failed after a merge.\nThe second line keeps its break."),
		components.Fields([]components.Field{{Label: "Branch", Value: "main"}, {Label: "Commit", Value: "465882e"}}),
		components.Table([]string{"Job", "Result", "Time"}, [][]string{{"build", "ok", "1m"}, {"test", "failed", "3m"}, {"lint", "failed", "20s"}}),
		components.List([]string{"Fix the test", "Fix the lint", "Run it again"}, true),
		components.Code("$ go test ./...\n--- FAIL: TestRender (0.21s)\n    ui_test.go:218: a very long line that scrolls sideways rather than wrapping in the report window"),
		components.Link("The run", "https://example.com/runs/1"),

		components.Heading("Failures"),
		components.Note("TestRender failed on 3 of the last 5 runs; it may be flaky.", "yellow"),
		components.Code(strings.Join(failureLog, "\n")),
		components.Paragraph("The lint failure is a new rule: exported functions need a comment. " +
			"It flags 4 functions, all added in the last week."),
		components.List([]string{"ui/bar.go:61 Render", "ui/report.go:19 ShowReport", "theme/theme.go:57 CSS", "shim/shim.go:150 EvalResult"}, false),

		components.Heading("History"),
		components.Table([]string{"Run", "Commit", "Result", "Time", "Author"}, history),
		components.Note("The last green run was #108, 9 hours ago.", "green"),

		components.Heading("Environment"),
		components.Fields([]components.Field{
			{Label: "Runner", Value: "macos-14-arm64"}, {Label: "Go", Value: "1.26.1"}, {Label: "templ", Value: "v0.3.1001"},
			{Label: "Started", Value: now.Add(-3 * time.Minute).Format(time.Kitchen)}, {Label: "Duration", Value: "4m 21s"},
			{Label: "Trigger", Value: "push to main"},
		}),
		components.Paragraph(strings.Repeat("A longer paragraph that wraps across several lines, so the report runs well past the bottom of its window. ", 6)),
		components.Link("All runs", "https://example.com/runs"),
	}

	if *browser {
		serveBrowser(*addr, name, load, sample)
		return
	}

	ui.Init()
	var bar *ui.Bar
	bar, err = ui.Open(
		ui.Options{Edge: "bottom", Height: barHeight, Level: "normal", Theme: th},
		ui.Handlers{OpenURL: func(url string) { log.Printf("url %s", url) }})
	if err != nil {
		log.Fatal(err)
	}
	sample.open = func(id string) {
		title, blocks := sample.report(id)
		if err := bar.ShowReport(id, title, blocks); err != nil {
			log.Print(err)
		}
	}
	sample.changed = func() { bar.Render(sample.shown()) }

	go func() {
		bar.Render(sample.shown())
		if err := bar.ShowReport("report", "Lorem ipsum dolor", sample.long); err != nil {
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

// sample is what the preview shows: basic cards and timers, whose Clicks open a
// report, run nothing but a log line, or dismiss the card, and the long report. Opening and dismissing go
// through open and changed, which the windows or the browser set before showing it.
type sample struct {
	mu     sync.Mutex
	cards  []components.Basic
	timers []components.Timer
	long   []templ.Component

	open    func(id string) // shows id's report
	changed func()          // shows the cards after one's dismissed
}

// opens is a Click that shows id's report.
func (s *sample) opens(id string) components.Click {
	return func() {
		log.Printf("open %s", id)
		s.open(id)
	}
}

// runs is a Click that logs running id, and does nothing else.
func (s *sample) runs(id string) components.Click {
	return func() { log.Printf("run %s", id) }
}

// completes is a Click for when timer id is up, which logs it.
func (s *sample) completes(id string) components.Click {
	return func() { log.Printf("complete %s", id) }
}

// dismisses is a Click that takes card id off the bar.
func (s *sample) dismisses(id string) components.Click {
	return func() {
		log.Printf("dismiss %s", id)
		s.mu.Lock()
		s.cards = slices.DeleteFunc(s.cards, func(c components.Basic) bool { return c.ID == id })
		s.timers = slices.DeleteFunc(s.timers, func(t components.Timer) bool { return t.ID == id })
		s.mu.Unlock()
		s.changed()
	}
}

// shown is the cards still on the bar.
func (s *sample) shown() []components.Card {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append(components.Cards(s.timers), components.Cards(s.cards)...)
}

// report is the title and blocks of id's report.
func (s *sample) report(id string) (string, []templ.Component) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := slices.IndexFunc(s.timers, func(t components.Timer) bool { return t.ID == id }); i >= 0 {
		t := s.timers[i]
		return t.Title, []templ.Component{components.Heading(t.Title), components.Fields([]components.Field{
			{Label: "Started", Value: t.Start.Format(time.Kitchen)}, {Label: "Ends", Value: t.End.Format(time.Kitchen)},
		})}
	}
	return reportFor(id, s.cards, s.long)
}

// loadTheme is the theme name: a built-in one, or the one whose manifest is the json
// file name. With fromDisk, a built-in one is read from builtinDir when it's there.
func loadTheme(name string, fromDisk bool) (theme.Theme, error) {
	if strings.HasSuffix(name, ".json") {
		// From the root, so its stylesheets and fonts can be anywhere.
		file, err := filepath.Abs(name)
		if err != nil {
			return theme.Theme{}, err
		}
		return theme.Load(os.DirFS("/"), filepath.ToSlash(strings.TrimPrefix(file, "/")))
	}
	set := theme.Builtins
	if _, err := os.Stat(builtinDir); fromDisk && err == nil {
		if set, err = theme.ReadSet(os.DirFS(builtinDir)); err != nil {
			return theme.Theme{}, err
		}
	}
	th, err := set.Theme(name)
	if err != nil {
		err = fmt.Errorf("%w; the themes are %s, or give the path to your own .json", err, strings.Join(set.Names(), ", "))
	}
	return th, err
}

// reportFor is the title and blocks of id's report: the long one for the report card,
// and for any other card or action one made from what it shows on the bar.
func reportFor(id string, cards []components.Basic, long []templ.Component) (string, []templ.Component) {
	if id == "report" {
		return "Lorem ipsum dolor", long
	}
	if i := slices.IndexFunc(cards, func(c components.Basic) bool { return c.ID == id }); i >= 0 {
		c := cards[i]
		blocks := []templ.Component{components.Heading(c.Title)}
		if c.Description != "" {
			blocks = append(blocks, components.Paragraph(c.Description))
		}
		if c.Priority != 0 {
			blocks = append(blocks, components.Fields([]components.Field{{Label: "Priority", Value: strconv.Itoa(c.Priority)}}))
		}
		return c.Title, blocks
	}
	for _, c := range cards {
		if i := slices.IndexFunc(c.Actions, func(q components.Action) bool { return q.ID == id }); i >= 0 {
			q := c.Actions[i]
			blocks := []templ.Component{components.Heading(q.Title), components.Paragraph("On " + c.Title + ".")}
			return q.Title, blocks
		}
	}
	return id, []templ.Component{components.Paragraph("Nothing to show for " + id + ".")}
}
