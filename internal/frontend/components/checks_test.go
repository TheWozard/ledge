package components

import (
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
	html := render(t, BarContent([]Card{card}, nil))
	contains(t, html,
		`<li id="card-ci" class="checks">`,
		`<span class="title">CI</span> <div class="check-list">`,
		`<span class="check" data-state="passed" title="build: passed"><span class="check-mark">✓</span>build</span>`,
		`<span class="check" data-state="failed" title="&lt;test&gt;: failed"><span class="check-mark">✗</span>&lt;test&gt;</span>`,
		`data-state="running" title="lint: running"><span class="check-mark">●</span>`,
		`data-state="pending" title="deploy: pending"><span class="check-mark">○</span>`,
		`data-state="skipped" title="docs: skipped"><span class="check-mark">–</span>`,
		// A state that isn't one is pending.
		`data-state="pending" title="odd: pending"><span class="check-mark">○</span>odd</span>`,
		`</div><div class="description">On main</div><div class="actions"><div id="action-rerun"`)
	lacks(t, html, "<test>", "exploded")

	// Without checks, there's no row for them.
	empty := render(t, BarContent([]Card{Checks{Basic: Basic{ID: "none", Title: "None"}}}, nil))
	if strings.Contains(empty, "check-list") {
		t.Errorf("a card without checks has a row for them:\n%s", empty)
	}
}
