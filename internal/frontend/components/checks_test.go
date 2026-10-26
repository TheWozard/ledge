package components

import (
	"slices"
	"strings"
	"testing"
)

// A checks card shows its title, then each check's mark and name, with its state in
// data-state and its name and state as its tooltip, then its description.
func TestChecks(t *testing.T) {
	card := Checks{Basic: Basic{ID: "ci", Title: "CI", Description: "On main",
		Actions: []Action{{ID: "rerun", Title: "Rerun test", Click: nothing}}},
		Checks: []Check{
			{Name: "build", State: "passed"}, {Name: "<test>", State: "failed"}, {Name: "lint", State: "running"},
			{Name: "deploy", State: "pending"}, {Name: "docs", State: "skipped"}, {Name: "odd", State: "exploded"},
		}}
	html := render(t, BarContent([]Card{card}))
	contains(t, html,
		`<span class="title">CI</span> <div class="check-list">`,
		`<span class="check" data-state="passed" title="build: passed"><span class="check-mark">✓</span>build</span>`,
		`<span class="check" data-state="failed" title="&lt;test&gt;: failed"><span class="check-mark">✗</span>&lt;test&gt;</span>`,
		`data-state="running" title="lint: running"><span class="check-mark">●</span>`,
		`data-state="pending" title="deploy: pending"><span class="check-mark">○</span>`,
		`data-state="skipped" title="docs: skipped"><span class="check-mark">–</span>`,
		// A state that isn't one is pending.
		`data-state="pending" title="odd: pending"><span class="check-mark">○</span>odd</span>`,
		`</div><div class="description">On main</div></li>`,
		// Its action, to rerun the failed one, is in its menu.
		`<li id="card-ci" class="checks" data-menu="card/ci/menu">`)
	lacks(t, html, "<test>", "exploded")

	lacks(t, html, "check clickable", `data-click="body/`)

	// Without checks, there's no row for them.
	empty := render(t, BarContent([]Card{Checks{Basic: Basic{ID: "none", Title: "None"}}}))
	if strings.Contains(empty, "check-list") {
		t.Errorf("a card without checks has a row for them:\n%s", empty)
	}
}

// A check with a Click is clicked on its own, not as its card: it's clickable, its
// name its id in the card's body, and the Clicks have it apart from the card's. One
// without is clicked as the card.
func TestClickableChecks(t *testing.T) {
	var got []string
	click := func(name string) Click { return func(ClickEvent) { got = append(got, name) } }
	card := Checks{Basic: Basic{ID: "ci", Title: "CI", Click: click("card")},
		Checks: []Check{{Name: "build", State: "passed"}, {Name: "test", State: "failed", Click: click("test")}}}
	html, clicks := renderClicks(t, BarContent([]Card{card}))
	contains(t, html,
		`<span class="check" data-state="passed" title="build: passed">`,
		`<span class="check clickable" data-click="body/ci/test" data-state="failed" title="test: failed">`)
	if ids := clicks.IDs(); !slices.Equal(ids, []string{"body/ci/test", "card/ci/click"}) {
		t.Fatalf("Clicks have %q", ids)
	}
	clicks.Click("body/ci/test")(ClickEvent{})
	if strings.Join(got, ",") != "test" {
		t.Errorf("clicking the test check called %v", got)
	}
}
