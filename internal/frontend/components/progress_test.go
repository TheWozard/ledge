package components

import (
	"math"
	"testing"
)

// A progress card shows its title, then how far along it is, as a percent or its
// label, with its description beside it, and has --progress for a theme to fill it.
func TestProgress(t *testing.T) {
	cards := []Progress{
		{Basic: Basic{ID: "deploy", Title: "Deploy", Description: "<push> next", Color: "#2f9e5b"}, Value: 0.6},
		{Basic: Basic{ID: "steps", Title: "Steps"}, Value: 0.4, Label: "2/5"},
		{Basic: Basic{ID: "none", Title: "Not started"}},
	}
	html := render(t, BarContent(Cards(cards), nil))
	contains(t, html,
		`<li id="card-deploy" class="progress" data-color="#2f9e5b" style="--card-color: #2f9e5b; --progress: 0.6;">`,
		`<span class="title">Deploy</span><div class="reading"><div class="reading-value">60%</div><div class="description">&lt;push&gt; next</div></div>`,
		`<li id="card-steps" class="progress" style="--progress: 0.4;">`,
		`<div class="reading-value">2/5</div></div>`,
		// At 0, it's not started: no --progress, so no fill.
		`<li id="card-none" class="progress">`, `<div class="reading-value">0%</div>`)
}

// A card's progress goes on it as --progress, up to 1; none, 0 or less has none.
func TestFrameProgress(t *testing.T) {
	for progress, want := range map[float64]string{
		0.25: "--progress: 0.25", 1: "--progress: 1", 3: "--progress: 1",
		0: "", -1: "", math.NaN(): "",
	} {
		if got := (Frame{ID: "c", Progress: progress}).style(nil); got != want {
			t.Errorf("progress %v: style %q, want %q", progress, got, want)
		}
	}
}

// A percent is rounded down, so it's never done early, and stays within 0 and 100.
func TestPercent(t *testing.T) {
	for value, want := range map[float64]string{
		0: "0%", 0.6: "60%", 0.999: "99%", 1: "100%", 2: "100%", -1: "0%", math.NaN(): "0%",
	} {
		if got := percent(value); got != want {
			t.Errorf("percent(%v) = %s, want %s", value, got, want)
		}
	}
}
