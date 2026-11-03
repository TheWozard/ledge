package components

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// A Run shows its button in its state's colour, its state's word (or its Status),
// when it started, and its output, escaped. One that hasn't run has no time; one
// without a Click, such as while it runs, has its button disabled. With the theme's
// run icon, the button shows it in place of its dot, then its title; with no title,
// the icon alone, "Run" its tooltip. Its stop button is disabled without a Stop, and
// with one has the Run's ID with /stop after it.
func TestRunBlock(t *testing.T) {
	html := render(t, RunBlock(Run{ID: "go"}))
	contains(t, html, `<div class="run" data-state="">`, `data-color="gray"`, "Run</button>", "disabled", "Not run</span>", "<pre><code></code></pre>")
	lacks(t, html, "<time")

	started := time.Date(2026, 10, 29, 14, 3, 5, 0, time.UTC)
	html, clicks := renderClicks(t, RunBlock(Run{ID: "go", Title: "Deploy", State: "failed",
		Status: "exit status 1 after 3s", Started: started, Output: "<b>oops</b>\n", Click: nothing}))
	contains(t, html, `data-state="failed"`, `data-color="red"`, `data-click="go"`, "Deploy</button>", "exit status 1 after 3s</span>",
		`<time datetime="2026-10-29T14:03:05Z"`, "Started Thu Oct 29 14:03:05</time>", "<pre><code>&lt;b&gt;oops&lt;/b&gt;\n</code></pre>")
	contains(t, html, `class="buttons run-stop"`, "Stop</button>")
	lacks(t, html, "<b>oops")
	if strings.Count(html, "disabled") != 1 {
		t.Errorf("only the stop button should be disabled: %s", html)
	}
	if !slices.Contains(clicks.IDs(), "go") {
		t.Errorf("the Clicks have %q", clicks.IDs())
	}

	contains(t, render(t, RunBlock(Run{State: "running"})), `data-state="running"`, `data-color="blue"`, "Running</span>")
	html, clicks = renderClicks(t, RunBlock(Run{ID: "go", State: "running", Stop: nothing}))
	contains(t, html, `data-click="go%2Fstop"`)
	if !slices.Contains(clicks.IDs(), "go%2Fstop") {
		t.Errorf("the Clicks have %q", clicks.IDs())
	}
	contains(t, render(t, RunBlock(Run{State: "passed"})), `data-color="green"`, "Passed</span>")
	contains(t, render(t, RunBlock(Run{State: "odd"})), `data-state=""`, `data-color="gray"`, "Not run</span>")

	var icon strings.Builder
	svg := "data:image/svg+xml;base64,PHN2Zy8+"
	if err := RunBlock(Run{Title: "Deploy"}).Render(WithIcons(context.Background(), Icons{Run: svg, Stop: svg}), &icon); err != nil {
		t.Fatal(err)
	}
	contains(t, icon.String(), `<span class="icon" aria-hidden="true"></span>Deploy</button>`)
	lacks(t, icon.String(), "button-dot", `title="Deploy"`)

	icon.Reset()
	if err := RunBlock(Run{}).Render(WithIcons(context.Background(), Icons{Run: svg}), &icon); err != nil {
		t.Fatal(err)
	}
	contains(t, icon.String(), `title="Run"`, `<span class="icon" aria-hidden="true"></span></button>`)

	icon.Reset()
	if err := RunBlock(Run{}).Render(WithIcons(context.Background(), Icons{Stop: svg}), &icon); err != nil {
		t.Fatal(err)
	}
	contains(t, icon.String(), `title="Stop"`, `data-color="red"`)
	lacks(t, icon.String(), "Stop</button>")
}
