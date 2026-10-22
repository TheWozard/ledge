package components

import (
	"fmt"
	"testing"
	"time"
)

// A timer is a basic card of its own kind: its title, and the time it has left, with
// its start and end for the page to count it down. Its frame ends when it does.
func TestTimer(t *testing.T) {
	start := time.Now()
	timer := Timer{Basic: Basic{ID: "tea", Title: "<b>Tea</b>", Color: "#3b82f6", Click: nothing, Dismiss: nothing},
		Start: start, End: start.Add(90*time.Second + 500*time.Millisecond), Complete: func() {}}
	html := render(t, BarContent([]Card{timer}, nil))
	contains(t, html,
		`<li id="card-tea" class="timer clickable" data-click="card/tea/click" data-color="#3b82f6"`,
		`<span class="title">&lt;b&gt;Tea&lt;/b&gt;</span>`,
		fmt.Sprintf(`<div class="reading-value time-left" data-start="%d" data-end="%d">1:31</div>`, start.UnixMilli(), timer.End.UnixMilli()),
		`data-click="card/tea/dismiss"`)
	if f := timer.Frame(); !f.Ends.Equal(timer.End) || f.Complete == nil {
		t.Errorf("the timer's frame ends %v, with Complete %v", f.Ends, f.Complete != nil)
	}
	if _, clicks := renderClicks(t, BarContent([]Card{timer}, nil)); len(clicks.IDs()) != 2 {
		t.Errorf("the timer has Clicks %q, want its own and its ×'s", clicks.IDs())
	}
}

// A timer for something coming up, such as a meeting, has no start, so the page only
// counts it down: it has no data-start to fill it from. Its description, width and
// actions are a basic card's.
func TestUpcomingTimer(t *testing.T) {
	end := time.Now().Add(12*time.Minute + 500*time.Millisecond)
	timer := Timer{Basic: Basic{ID: "standup", Title: "Standup", Description: "10:30 · <Room 4>", Width: 200,
		Actions: []Action{{ID: "join", Title: "Join", Color: "blue", Click: nothing}}}, End: end}
	html := render(t, BarContent([]Card{timer}, nil))
	contains(t, html,
		`<li id="card-standup" class="timer" data-menu="card-standup" style="--card-width: 200px;">`,
		// The title, then the time with the description beside it.
		fmt.Sprintf(`<span class="title">Standup</span><div class="reading"><div class="reading-value time-left" data-end="%d">12:01</div>`, end.UnixMilli()),
		`<div class="description">10:30 · &lt;Room 4&gt;</div></div>`,
		// Its action, to join the meeting, is in its menu.
		`data-menu="card-standup"`)
	lacks(t, html, "data-start")
	if timer.Frame().Action("action/standup/join") == nil {
		t.Error("the timer's action has no Click")
	}
}

// A timer for something going on, such as an incident, has no end, so the page counts
// up from its start: it has no data-end, and its frame doesn't end.
func TestSinceTimer(t *testing.T) {
	start := time.Now().Add(-(12*time.Minute + 500*time.Millisecond))
	timer := Timer{Basic: Basic{ID: "incident", Title: "Incident"}, Start: start, Complete: func() {}}
	html := render(t, BarContent([]Card{timer}, nil))
	contains(t, html, fmt.Sprintf(`<div class="reading-value time-left" data-start="%d">12:00</div>`, start.UnixMilli()))
	lacks(t, html, "data-end")
	if f := timer.Frame(); !f.Ends.IsZero() {
		t.Errorf("a timer without an end ends at %v", f.Ends)
	}
}

func TestTimeSince(t *testing.T) {
	for d, want := range map[time.Duration]string{
		-time.Second: "0:00", 0: "0:00", 999 * time.Millisecond: "0:00", time.Second: "0:01",
		61*time.Second + 999*time.Millisecond: "1:01", time.Hour + 2*time.Minute + 3*time.Second: "1:02:03",
	} {
		if got := timeSince(d); got != want {
			t.Errorf("timeSince(%v) = %s, want %s", d, got, want)
		}
	}
}

func TestTimeLeft(t *testing.T) {
	for d, want := range map[time.Duration]string{
		-time.Second: "0:00", 0: "0:00", time.Millisecond: "0:01", 59 * time.Second: "0:59",
		61 * time.Second: "1:01", 10*time.Minute + 5*time.Second: "10:05", time.Hour + 2*time.Minute + 3*time.Second: "1:02:03",
	} {
		if got := timeLeft(d); got != want {
			t.Errorf("timeLeft(%v) = %s, want %s", d, got, want)
		}
	}
}
