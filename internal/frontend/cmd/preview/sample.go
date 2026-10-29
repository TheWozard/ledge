package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
)

// sample is what the preview shows: basic cards, timers, progress, stat, checks and
// message cards, whose Clicks open a report, run nothing but a log line, or dismiss
// the card, and the long report, whose buttons do the same, or replace its triage
// block. Opening, dismissing, replacing and advance go through open, replace and
// changed, which the windows or the browser set before showing it.
type sample struct {
	mu       sync.Mutex
	cards    []components.Basic
	timers   []components.Timer
	progress []components.Progress
	stats    []components.Stat
	checks   []components.Checks
	messages []components.Message
	started  time.Time // when the long report's run started
	assigned bool      // whether the long report's failures are assigned, as its triage block shows

	open    func(id string)                         // shows id's report
	replace func(id string, block components.Block) // replaces a block in id's report
	changed func()                                  // shows the cards after one's dismissed
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
		{ID: "long", Title: "Ex ea commodo", Description: strings.Repeat("Consequat duis aute irure dolor in reprehenderit. ", 12)},
		{ID: "neque", Title: "Neque porro quisquam"},
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
	s.stats = []components.Stat{
		// advance adds to this one's history.
		{Basic: components.Basic{ID: "errors", Title: "Lorem errors", Description: "Last hour", Color: "#e0362c",
			Click: s.opens("errors")}, Value: "2.3%", Change: "↑0.8", ChangeColor: "red",
			History: []float64{1.2, 1.4, 1.1, 1.5, 1.3, 1.9, 1.7, 2.1, 2.0, 2.3}},
		{Basic: components.Basic{ID: "users", Title: "Ipsum users"}, Value: "1,204", Change: "+12"},
	}
	for i, st := range s.stats {
		s.stats[i].Dismiss = s.dismisses(st.ID)
	}
	s.checks = []components.Checks{
		{Basic: components.Basic{ID: "ci", Title: "Lorem CI", Description: "On main, 465882e", Color: "#e0362c", Click: s.opens("ci"),
			Actions: []components.Action{{ID: "rerun", Title: "Rerun test", Color: "yellow", Click: s.runs("rerun")}}},
			Checks: []components.Check{
				{Name: "build", State: "passed"}, {Name: "test", State: "failed", Click: s.runs("open the test log")}, {Name: "lint", State: "running"},
				{Name: "deploy", State: "pending"}, {Name: "docs", State: "skipped"},
			}},
	}
	for i, c := range s.checks {
		s.checks[i].Dismiss = s.dismisses(c.ID)
	}
	s.messages = []components.Message{
		{Basic: components.Basic{ID: "dm", Title: "Re: Lorem deploy", Description: "Consectetur adipiscing elit, sed do eiusmod tempor. Ut enim ad minim veniam?",
			Click: s.opens("dm"), Actions: []components.Action{
				{ID: "reply", Title: "Reply", Color: "blue", Click: s.runs("reply")},
				{ID: "mark-read", Title: "Mark as read", Click: s.runs("mark-read")},
			}},
			From: "Ada Lovelace", Sent: now.Add(-4 * time.Minute)},
		// A picture for an avatar: a 1×1 teal PNG.
		{Basic: components.Basic{ID: "channel", Title: "#lorem-ipsum", Description: "Duis aute irure dolor in reprehenderit.", Color: "#8b5cf6",
			Click: s.opens("channel")},
			From: "ci-bot", Sent: now.Add(-26 * time.Hour),
			Avatar: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mMQKav8DwADKAGbTX5zawAAAABJRU5ErkJggg=="},
	}
	for i, m := range s.messages {
		s.messages[i].Dismiss = s.dismisses(m.ID)
	}
	s.started = now
	return s
}

// longReport is one of every block, with a log and a run history long enough to
// scroll, the log sideways too, and buttons: some that run, one that can't, and one
// that replaces the triage block it's in. Call it with s's lock held.
func (s *sample) longReport() []templ.Component {
	now := s.started
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
		components.Buttons([]components.Button{
			{ID: "rerun-jobs", Title: "Rerun failed jobs", Color: "yellow", Click: s.slowly("rerun-jobs")},
			{ID: "view-logs", Title: "View logs", Color: "blue", Click: s.runs("view-logs")},
			{ID: "merge", Title: "Merge", Color: "green"},
		}),

		components.Heading("Triage"),
		components.Block{ID: "triage", Content: s.triage()},

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

// triage is the long report's triage block: who has its failures, and a button to
// take them or give them back, which replaces the block with how it is then. Call it
// with s's lock held.
func (s *sample) triage() templ.Component {
	assign := func(components.ClickEvent) {
		s.mu.Lock()
		s.assigned = !s.assigned
		log.Printf("assigned %v", s.assigned)
		block := s.triage()
		s.mu.Unlock()
		s.replace(longID, components.Block{ID: "triage", Content: block})
	}
	if s.assigned {
		return templ.Join(components.Note("Assigned to you.", "blue"),
			components.Buttons([]components.Button{{ID: "unassign", Title: "Unassign", Color: "gray", Click: assign}}))
	}
	return templ.Join(components.Note("Nobody has these failures yet.", "yellow"),
		components.Buttons([]components.Button{{ID: "assign", Title: "Assign to me", Color: "blue", Click: assign}}))
}

// how is how a click was made, as the Clicks log it: "", or such as " with ⌥⌘" or
// " with the middle button".
func how(e components.ClickEvent) string {
	var keys string
	for _, k := range []struct {
		held bool
		key  string
	}{{e.Ctrl, "⌃"}, {e.Alt, "⌥"}, {e.Shift, "⇧"}, {e.Cmd, "⌘"}} {
		if k.held {
			keys += k.key
		}
	}
	switch {
	case e.Middle && keys != "":
		return " with the middle button and " + keys
	case e.Middle:
		return " with the middle button"
	case keys != "":
		return " with " + keys
	}
	return ""
}

// opens is a Click that shows id's report.
func (s *sample) opens(id string) components.Click {
	return func(e components.ClickEvent) {
		log.Printf("open %s%s", id, how(e))
		s.open(id)
	}
}

// runs is a Click that logs running id, and does nothing else.
func (s *sample) runs(id string) components.Click {
	return func(e components.ClickEvent) { log.Printf("run %s%s", id, how(e)) }
}

// slowly is a Click that logs running id, and takes a couple of seconds about it, to
// show it busy.
func (s *sample) slowly(id string) components.Click {
	return func(e components.ClickEvent) {
		log.Printf("run %s%s", id, how(e))
		time.Sleep(2 * time.Second)
		log.Printf("ran %s", id)
	}
}

// completes is what's done when timer id is up: it's logged.
func (s *sample) completes(id string) func() {
	return func() { log.Printf("complete %s", id) }
}

// dismisses is a Click that takes card id off the bar.
func (s *sample) dismisses(id string) components.Click {
	return func(e components.ClickEvent) {
		log.Printf("dismiss %s%s", id, how(e))
		s.mu.Lock()
		s.cards = slices.DeleteFunc(s.cards, func(c components.Basic) bool { return c.ID == id })
		s.timers = slices.DeleteFunc(s.timers, func(t components.Timer) bool { return t.ID == id })
		s.progress = slices.DeleteFunc(s.progress, func(p components.Progress) bool { return p.ID == id })
		s.stats = slices.DeleteFunc(s.stats, func(st components.Stat) bool { return st.ID == id })
		s.checks = slices.DeleteFunc(s.checks, func(c components.Checks) bool { return c.ID == id })
		s.messages = slices.DeleteFunc(s.messages, func(m components.Message) bool { return m.ID == id })
		s.mu.Unlock()
		s.changed()
	}
}

// shown is the cards still on the bar.
func (s *sample) shown() []components.Card {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Concat(components.Cards(s.timers), components.Cards(s.progress), components.Cards(s.stats),
		components.Cards(s.checks), components.Cards(s.messages), components.Cards(s.cards))
}

// advance moves the deploy along every second, starting again once it's done, adds
// to the error rate's history, and shows them: the app rendering cards again as they
// change. Run it once changed is set.
func (s *sample) advance() {
	for range time.Tick(time.Second) {
		s.mu.Lock()
		if i := slices.IndexFunc(s.progress, func(p components.Progress) bool { return p.ID == "deploy" }); i >= 0 {
			if s.progress[i].Value += 0.05; s.progress[i].Value > 1 {
				s.progress[i].Value = 0
			}
		}
		if i := slices.IndexFunc(s.stats, func(st components.Stat) bool { return st.ID == "errors" }); i >= 0 {
			st := &s.stats[i]
			last := st.History[len(st.History)-1]
			next := max(0.1, last+rand.Float64()-0.5)
			st.History = append(st.History[1:], next)
			st.Value = fmt.Sprintf("%.1f%%", next)
			st.Change, st.ChangeColor = fmt.Sprintf("↑%.1f", next-last), "red"
			if next < last {
				st.Change, st.ChangeColor = fmt.Sprintf("↓%.1f", last-next), "green"
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
		return "Lorem ipsum dolor", s.longReport()
	}
	if i := slices.IndexFunc(s.progress, func(p components.Progress) bool { return p.ID == id }); i >= 0 {
		p := s.progress[i]
		return p.Title, []templ.Component{components.Heading(p.Title), components.Fields([]components.Field{
			{Label: "Done", Value: fmt.Sprintf("%.0f%%", p.Value*100)},
		})}
	}
	if i := slices.IndexFunc(s.messages, func(m components.Message) bool { return m.ID == id }); i >= 0 {
		m := s.messages[i]
		return m.Title, []templ.Component{components.Heading(m.Title), components.Fields([]components.Field{
			{Label: "From", Value: m.From}, {Label: "Sent", Value: m.Sent.Format(time.Kitchen)},
		}), components.Paragraph(m.Description)}
	}
	if i := slices.IndexFunc(s.checks, func(c components.Checks) bool { return c.ID == id }); i >= 0 {
		c := s.checks[i]
		var rows [][]string
		for _, check := range c.Checks {
			rows = append(rows, []string{check.Name, check.State})
		}
		return c.Title, []templ.Component{components.Heading(c.Title), components.Table([]string{"Check", "State"}, rows),
			components.Buttons([]components.Button{
				{ID: "rerun-failed", Title: "Rerun failed", Color: "yellow", Click: s.slowly("rerun-failed")},
				{ID: "view-run", Title: "View run", Color: "blue", Click: s.runs("view-run")},
				{ID: "cancel", Title: "Cancel"},
			})}
	}
	if i := slices.IndexFunc(s.stats, func(st components.Stat) bool { return st.ID == id }); i >= 0 {
		st := s.stats[i]
		return st.Title, []templ.Component{components.Heading(st.Title), components.Fields([]components.Field{
			{Label: "Now", Value: st.Value}, {Label: "Change", Value: st.Change},
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
