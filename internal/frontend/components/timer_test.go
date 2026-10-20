package components

import (
	"fmt"
	"testing"
	"time"
)

// A timer is a card of its own kind: its title, and the time it has left, with its
// start and end for the page to count it down. Its frame ends when it does.
func TestTimer(t *testing.T) {
	start := time.Now()
	timer := Timer{ID: "tea", Title: "<b>Tea</b>", Start: start, End: start.Add(90*time.Second + 500*time.Millisecond),
		Color: "#3b82f6", Click: nothing, Dismiss: nothing, Complete: nothing}
	html := render(t, BarContent([]Card{timer}, nil))
	contains(t, html,
		`<li id="card-tea" class="timer clickable" data-click="card-tea" data-color="#3b82f6"`,
		`<span class="title">&lt;b&gt;Tea&lt;/b&gt;</span>`,
		fmt.Sprintf(`<div class="time-left" data-start="%d" data-end="%d">1:31</div>`, start.UnixMilli(), timer.End.UnixMilli()),
		`data-click="dismiss-tea"`)
	if f := timer.Frame(); !f.Ends.Equal(timer.End) || f.Complete == nil {
		t.Errorf("the timer's frame ends %v, with Complete %v", f.Ends, f.Complete != nil)
	}
	if clicks := Clicks([]Card{timer}); len(clicks) != 2 {
		t.Errorf("the timer has %d Clicks, want its own and its ×'s", len(clicks))
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
