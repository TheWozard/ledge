// Command preview opens a bar and a report window in a theme, with one of everything
// on them and enough that the pills, the row of cards and the report all scroll, and
// keeps them open until it's interrupted. Clicks are logged; opening a card or a
// pill's link shows its report in its own window, and dismissing a card takes it off
// the bar.
//
//	go run ./cmd/preview [theme]
//
// theme is a built-in theme's name, or the path to a theme's json manifest of your
// own, such as ~/themes/neon.json, loaded as theme.Load does.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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
	name := theme.Builtins.Default()
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	var th theme.Theme
	var err error
	if strings.HasSuffix(name, ".json") {
		// From the root, so its stylesheets and fonts can be anywhere.
		var file string
		if file, err = filepath.Abs(name); err == nil {
			th, err = theme.Load(os.DirFS("/"), filepath.ToSlash(strings.TrimPrefix(file, "/")))
		}
	} else if th, err = theme.Builtin(name); err != nil {
		err = fmt.Errorf("%w; the themes are %s, or give the path to your own .json", err, strings.Join(theme.Builtins.Names(), ", "))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	now := time.Now()
	pills := []components.Pill{
		{ID: "aws", Title: "AWS", Color: "green", Detail: "Signed in", Command: "aws sso login",
			Schedule: []components.Step{{At: now.Add(30 * time.Second), Color: "yellow"}, {At: now.Add(time.Minute), Color: "red"}}},
		{ID: "vpn", Title: "VPN", Color: "blue", Link: true},
		{ID: "disk", Title: "Disk", Color: "gray", Detail: "82% used"},
		{ID: "build", Title: "Build", Color: "#c060ff"},
		{ID: "docker", Title: "Docker", Color: "green", Detail: "4 containers running"},
		{ID: "k8s", Title: "Kubernetes", Color: "yellow", Detail: "Context: staging", Command: "kubectl config use-context prod"},
		{ID: "battery", Title: "Battery", Color: "green", Detail: "91%, charging"},
		{ID: "wifi", Title: "Wi-Fi", Color: "blue", Detail: "office-5g"},
		{ID: "calendar", Title: "Calendar", Color: "yellow", Detail: "Standup in 10 minutes", Link: true},
		{ID: "mail", Title: "Mail", Color: "gray", Detail: "3 unread", Link: true},
		{ID: "backup", Title: "Backup", Color: "red", Detail: "Last backup 3 days ago", Command: "tmutil startbackup"},
		{ID: "updates", Title: "Updates", Color: "blue", Detail: "2 updates available"},
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
		{ID: "alert", Title: "Error rate above 2% on api", Description: "p99 latency 1.8s, 5xx rate 2.4% over 5 minutes", Priority: "high",
			Tags: []string{"prod", "api"}, Opens: true, Hint: "https://example.com/alerts/42", Source: "alerts", Received: now.Add(-30 * time.Second)},
		{ID: "merged", Title: "PR merged: Fix the flaky test", Priority: "low", Tags: []string{"pr"}, Opens: true, Source: "github", Received: now.Add(-12 * time.Minute)},
		{ID: "cert", Title: "Certificate expires in 7 days", Description: "api.example.com, issued by Let's Encrypt", Priority: "medium",
			Tags: []string{"tls"}, Source: "certs", Received: now.Add(-2 * time.Hour)},
		{ID: "comment", Title: "New comment on your PR", Description: "Could this use the existing helper instead?",
			Tags: []string{"pr", "review"}, Opens: true, Source: "github", Received: now.Add(-25 * time.Minute)},
		{ID: "release", Title: "Release v1.4.0 published", Priority: "low", Tags: []string{"release"}, Source: "deploy", Received: now.Add(-26 * time.Hour)},
		{ID: "disk-warn", Title: "Disk 82% full", Description: "Clear caches or old builds to free space", Priority: "medium",
			Source: "system", Received: now.Add(-90 * time.Minute)},
		{ID: "meeting", Title: "Standup in 10 minutes", Tags: []string{"calendar"}, Opens: true, Source: "calendar", Received: now.Add(-1 * time.Minute)},
		{ID: "nightly", Title: "Nightly build passed", Tags: []string{"ci", "nightly"}, Source: "ci", Received: now.Add(-9 * time.Hour)},
		{ID: "oncall", Title: "You're on call from Monday", Description: "Handover with the current on-call at 10:00",
			Source: "pager", Received: now.Add(-3 * 24 * time.Hour)},
		{ID: "deps", Title: "3 dependency updates", Description: "templ, x/net, x/sys", Tags: []string{"deps", "bot", "weekly"},
			Opens: true, Source: "github", Received: now.Add(-4 * 24 * time.Hour)},
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
	report := []templ.Component{
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

	ui.Init()
	var bar *ui.Bar
	bar, err = ui.Open(
		ui.Options{Edge: "bottom", Height: 110, Level: "normal", Theme: th},
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
				mu.Lock()
				title, blocks := reportFor(id, pills, cards, report)
				mu.Unlock()
				if err := bar.ShowReport(id, title, blocks); err != nil {
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
		if err := bar.ShowReport("ci", "CI failed on main", report); err != nil {
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

// reportFor is the title and blocks of id's report: the long one for the CI card, and
// for any other card or pill one made from what it shows on the bar.
func reportFor(id string, pills []components.Pill, cards []components.Card, ci []templ.Component) (string, []templ.Component) {
	if id == "ci" {
		return "CI failed on main", ci
	}
	if i := slices.IndexFunc(cards, func(c components.Card) bool { return c.ID == id }); i >= 0 {
		c := cards[i]
		blocks := []templ.Component{components.Heading(c.Title)}
		if c.Description != "" {
			blocks = append(blocks, components.Paragraph(c.Description))
		}
		fields := []components.Field{{Label: "Source", Value: c.Source}, {Label: "Received", Value: c.Received.Format(time.Kitchen)}}
		if c.Priority != "" {
			fields = append(fields, components.Field{Label: "Priority", Value: c.Priority})
		}
		if len(c.Tags) > 0 {
			fields = append(fields, components.Field{Label: "Tags", Value: strings.Join(c.Tags, ", ")})
		}
		blocks = append(blocks, components.Fields(fields))
		if c.Hint != "" {
			blocks = append(blocks, components.Link(c.Hint, c.Hint))
		}
		return c.Title, blocks
	}
	if i := slices.IndexFunc(pills, func(p components.Pill) bool { return p.ID == id }); i >= 0 {
		p := pills[i]
		return p.Title, []templ.Component{components.Heading(p.Title), components.Note(p.Detail, p.Color)}
	}
	return id, []templ.Component{components.Paragraph("Nothing to show for " + id + ".")}
}
