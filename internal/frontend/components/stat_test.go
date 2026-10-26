package components

import (
	"math"
	"strings"
	"testing"
)

// A stat shows its title, then its value with its change beside it, in its colour
// if it has one, and its description beside those, and its history as a sparkline.
func TestStat(t *testing.T) {
	cards := []Stat{
		{Basic: Basic{ID: "errors", Title: "Errors", Description: "last <hour>"}, Value: "2.3%",
			Change: "↑0.8", ChangeColor: "red", History: []float64{1, 3, 2}},
		{Basic: Basic{ID: "users", Title: "Users"}, Value: "1,204", Change: "+12"},
		{Basic: Basic{ID: "plain", Title: "Plain"}, Value: "7", History: []float64{4}},
	}
	html := render(t, BarContent(Cards(cards)))
	contains(t, html,
		`<li id="card-errors" class="stat">`,
		`<span class="title">Errors</span><div class="reading"><div class="reading-value">2.3% `,
		`<span class="change" data-color="red" style="--change: var(--c-red);">↑0.8</span></div><div class="description">last &lt;hour&gt;</div></div>`,
		`<svg class="sparkline" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true"><polyline points="0.00,100.00 50.00,0.00 100.00,50.00"></polyline></svg>`,
		// Without a colour, its change is the text's.
		`<div class="reading-value">1,204 <span class="change">+12</span></div>`,
		`<div class="reading-value">7`)
	// With one value, there's no line: only the errors card has one.
	if n := strings.Count(html, "<svg"); n != 1 {
		t.Errorf("%d sparklines, want 1:\n%s", n, html)
	}
}

// A sparkline spans the box: oldest to newest left to right, lowest to highest bottom
// to top, flat in the middle. What isn't finite is left out, and fewer than two
// values is none.
func TestSparkline(t *testing.T) {
	for _, c := range []struct {
		history []float64
		want    string
	}{
		{nil, ""},
		{[]float64{5}, ""},
		{[]float64{0, 10}, "0.00,100.00 100.00,0.00"},
		{[]float64{10, 5, 0}, "0.00,0.00 50.00,50.00 100.00,100.00"},
		{[]float64{3, 3, 3}, "0.00,50.00 50.00,50.00 100.00,50.00"},
		{[]float64{0, math.NaN(), math.Inf(1), 10}, "0.00,100.00 100.00,0.00"},
		{[]float64{math.NaN(), 1}, ""},
	} {
		if got := sparkline(c.history); got != c.want {
			t.Errorf("sparkline(%v) = %q, want %q", c.history, got, c.want)
		}
	}
}
