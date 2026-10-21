package main

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
)

// sample is what the preview shows: basic cards, timers and progress cards, whose
// Clicks open a report, run nothing but a log line, or dismiss the card, and the long
// report. Opening, dismissing and advance go through open and changed, which the
// windows or the browser set before showing it.
type sample struct {
	mu       sync.Mutex
	cards    []components.Basic
	timers   []components.Timer
	progress []components.Progress
	long     []templ.Component

	open    func(id string) // shows id's report
	changed func()          // shows the cards after one's dismissed
}

// longID is the id of the card whose report is the long one, shown when the preview
// opens.
const longID = "report"

// lorem is a description long enough to scroll in its card.
const lorem = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
	"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. " +
	"Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur."

// newSample is one of everything, and enough that the row of cards and the long
// report both scroll, its timers counting from now.
func newSample(now time.Time) *sample {
	s := &sample{}
	s.cards = []components.Basic{
		{ID: "lorem", Title: "Lorem ipsum dolor sit amet", Description: lorem, Color: "#14b8a6"},
		{ID: "lorem-actions", Title: "Lorem ipsum with actions", Description: lorem, Color: "#ec4899", Width: 420, Click: s.opens("lorem-actions"),
			Actions: []components.Action{
				{ID: "lorem-1", Title: "Lorem", Color: "green", Click: s.runs("lorem-1")},
				{ID: "lorem-2", Title: "Ipsum", Color: "yellow", Click: s.opens("lorem-2")},
				{ID: "lorem-3", Title: "Dolor", Color: "red", Click: s.runs("lorem-3")},
				{ID: "lorem-4", Title: "Sit amet", Color: "blue", Click: s.opens("lorem-4")},
				{ID: "lorem-5", Title: "Consectetur", Color: "gray"},
				{ID: "lorem-6", Title: "Adipiscing", Color: "#14b8a6", Click: s.runs("lorem-6")},
				{ID: "lorem-7", Title: "Elit", Color: "#ec4899", Click: s.opens("lorem-7")},
				{ID: "lorem-8", Title: "Sed do eiusmod", Color: "#8b5cf6", Click: s.runs("lorem-8")},
			}},
		{ID: longID, Title: "Lorem ipsum dolor", Description: "Consectetur adipiscing elit, sed do eiusmod.", Color: "#e0362c",
			Click: s.opens(longID),
			Actions: []components.Action{
				{ID: "tempor", Title: "Tempor", Color: "yellow", Click: s.runs("tempor")},
				{ID: "incididunt", Title: "Incididunt", Color: "gray", Click: s.opens("incididunt")},
			}},
		{ID: "voluptate", Title: "In voluptate velit esse cillum dolore eu fugiat nulla", Description: "Excepteur sint occaecat cupidatat non proident, sunt in culpa.",
			Color: "#e0362c", Click: s.opens("voluptate"),
			Actions: []components.Action{
				{ID: "officia", Title: "Officia", Color: "red", Click: s.runs("officia")},
				{ID: "deserunt", Title: "Deserunt", Color: "blue", Click: s.opens("deserunt")},
				{ID: "mollit", Title: "Mollit anim", Color: "yellow", Click: s.runs("mollit")},
			}},
		{ID: "magna", Title: "Magna aliqua", Description: "Ad minim veniam, quis nostrud.", Color: "#e08a00", Click: s.opens("magna"),
			Actions: []components.Action{{ID: "exercitation", Title: "Exercitation", Color: "green", Click: s.runs("exercitation")}}},
		{ID: "perspiciatis", Title: "Sed ut perspiciatis", Description: "Unde omnis iste natus error sit voluptatem.", Color: "#e08a00",
			Actions: []components.Action{{ID: "accusantium", Title: "Accusantium", Color: "green", Click: s.runs("accusantium")}}},
		{ID: "quasi", Title: "Quasi architecto", Description: "Beatae vitae dicta sunt explicabo.", Color: "#e08a00",
			Actions: []components.Action{{ID: "nemo", Title: "Nemo enim", Color: "gray", Click: s.runs("nemo")}}},
		{ID: "ullamco", Title: "Ullamco laboris", Color: "#2f9e5b",
			Actions: []components.Action{{ID: "nisi", Title: "Nisi ut aliquip", Color: "#c060ff"}}},
		{ID: "laborum", Title: "Id est laborum", Color: "#2f9e5b", Click: s.opens("laborum")},
		{ID: "inventore", Title: "Inventore veritatis", Color: "#2f9e5b"},
		{ID: "long", Title: "Ex ea commodo", Description: strings.Repeat("Consequat duis aute irure dolor in reprehenderit. ", 12)},
		{ID: "doloremque", Title: "Doloremque laudantium", Description: "Totam rem aperiam, eaque ipsa quae ab illo.", Click: s.opens("doloremque")},
		{ID: "aspernatur", Title: "Aut odit aut fugit", Color: "#3b82f6", Click: s.opens("aspernatur"),
			Actions: []components.Action{{ID: "consequuntur", Title: "Consequuntur", Color: "green", Click: s.opens("consequuntur")}}},
		{ID: "neque", Title: "Neque porro quisquam"},
		{ID: "dolorem", Title: "Est qui dolorem ipsum", Description: "Quia dolor sit amet, consectetur, adipisci velit."},
		{ID: "numquam", Title: "Sed quia non numquam", Color: "#8b5cf6", Description: "Eius modi tempora incidunt", Click: s.opens("numquam")},
	}
	for i, c := range s.cards {
		s.cards[i].Dismiss = s.dismisses(c.ID)
	}
	s.timers = []components.Timer{
		{Basic: components.Basic{ID: "timer", Title: "Lorem ipsum timer", Color: "#3b82f6", Click: s.opens("timer")},
			Start: now.Add(-40 * time.Second), End: now.Add(80 * time.Second), Complete: s.completes("timer")},
		{Basic: components.Basic{ID: "timer-short", Title: "Dolor sit amet", Click: s.opens("timer-short")},
			Start: now, End: now.Add(25 * time.Second), Complete: s.completes("timer-short")},
		// Something coming up: no start, so no fill, just the countdown.
		{Basic: components.Basic{ID: "standup", Title: "Lorem standup", Description: now.Add(12*time.Minute).Format(time.Kitchen) + " · Room 4",
			Color: "#8b5cf6", Click: s.opens("standup"),
			Actions: []components.Action{{ID: "join", Title: "Join", Color: "blue", Click: s.runs("join")}}},
			End: now.Add(12 * time.Minute), Complete: s.completes("standup")},
		// Something going on: no end, so it counts up from its start.
		{Basic: components.Basic{ID: "incident", Title: "Lorem incident", Description: "Open since " + now.Add(-7*time.Minute).Format(time.Kitchen),
			Color: "#e0362c", Click: s.opens("incident"),
			Actions: []components.Action{{ID: "ack", Title: "Ack", Color: "yellow", Click: s.runs("ack")}}},
			Start: now.Add(-7 * time.Minute)},
	}
	for i, t := range s.timers {
		s.timers[i].Dismiss = s.dismisses(t.ID)
	}
	s.progress = []components.Progress{
		// advance moves this one along.
		{Basic: components.Basic{ID: "deploy", Title: "Lorem deploy", Description: "Building, then testing", Color: "#2f9e5b",
			Click: s.opens("deploy")}, Value: 0.35},
		{Basic: components.Basic{ID: "steps", Title: "Ipsum steps", Click: s.opens("steps")}, Value: 0.4, Label: "2/5"},
	}
	for i, p := range s.progress {
		s.progress[i].Dismiss = s.dismisses(p.ID)
	}
	s.long = longReport(now)
	return s
}

// longReport is one of every block, with a log and a run history long enough to
// scroll, the log sideways too.
func longReport(now time.Time) []templ.Component {
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
	return []templ.Component{
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
		s.progress = slices.DeleteFunc(s.progress, func(p components.Progress) bool { return p.ID == id })
		s.mu.Unlock()
		s.changed()
	}
}

// shown is the cards still on the bar.
func (s *sample) shown() []components.Card {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Concat(components.Cards(s.timers), components.Cards(s.progress), components.Cards(s.cards))
}

// advance moves the deploy along every second, starting again once it's done, and
// shows it: the app rendering a progress card again as it goes. Run it once changed
// is set.
func (s *sample) advance() {
	for range time.Tick(time.Second) {
		s.mu.Lock()
		if i := slices.IndexFunc(s.progress, func(p components.Progress) bool { return p.ID == "deploy" }); i >= 0 {
			if s.progress[i].Value += 0.05; s.progress[i].Value > 1 {
				s.progress[i].Value = 0
			}
		}
		s.mu.Unlock()
		s.changed()
	}
}

// report is the title and blocks of id's report: the long one for the report card,
// and for any other timer, card or action one made from what it shows on the bar.
func (s *sample) report(id string) (string, []templ.Component) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == longID {
		return "Lorem ipsum dolor", s.long
	}
	if i := slices.IndexFunc(s.progress, func(p components.Progress) bool { return p.ID == id }); i >= 0 {
		p := s.progress[i]
		return p.Title, []templ.Component{components.Heading(p.Title), components.Fields([]components.Field{
			{Label: "Done", Value: fmt.Sprintf("%.0f%%", p.Value*100)},
		})}
	}
	if i := slices.IndexFunc(s.timers, func(t components.Timer) bool { return t.ID == id }); i >= 0 {
		t := s.timers[i]
		return t.Title, []templ.Component{components.Heading(t.Title), components.Fields([]components.Field{
			{Label: "Started", Value: t.Start.Format(time.Kitchen)}, {Label: "Ends", Value: t.End.Format(time.Kitchen)},
		})}
	}
	if i := slices.IndexFunc(s.cards, func(c components.Basic) bool { return c.ID == id }); i >= 0 {
		c := s.cards[i]
		blocks := []templ.Component{components.Heading(c.Title)}
		if c.Description != "" {
			blocks = append(blocks, components.Paragraph(c.Description))
		}
		return c.Title, blocks
	}
	for _, c := range s.cards {
		if i := slices.IndexFunc(c.Actions, func(q components.Action) bool { return q.ID == id }); i >= 0 {
			q := c.Actions[i]
			return q.Title, []templ.Component{components.Heading(q.Title), components.Paragraph("On " + c.Title + ".")}
		}
	}
	return id, []templ.Component{components.Paragraph("Nothing to show for " + id + ".")}
}
