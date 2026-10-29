package bar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/components"

	"github.com/TheWozard/ledge/internal/schema"
	"github.com/TheWozard/ledge/internal/store"
)

// shown keeps the last html each run Block was shown with, as the report window would.
type shown struct {
	mu     sync.Mutex
	blocks map[string]string
}

func (s *shown) show(id string, block components.Block) (bool, error) {
	var html strings.Builder
	err := block.Render(context.Background(), &html)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blocks == nil {
		s.blocks = map[string]string{}
	}
	s.blocks[block.ID] = html.String()
	return true, err
}

func (s *shown) get(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blocks[id]
}

// runBar is a Bar with an event whose report has a run block, at index 1, and what
// it shows of the run as its output comes.
func runBar(t *testing.T, commands bool, command string) (*Bar, *shown, string) {
	t.Helper()
	events, err := store.Open(filepath.Join(t.TempDir(), "events.json"))
	if err != nil {
		t.Fatal(err)
	}
	commandJSON, _ := json.Marshal(command)
	value, err := schema.Decode([]byte(`{"id": "build", "title": "Build", "report": [{"type": "heading", "text": "Build"},
		{"type": "run", "command": ` + string(commandJSON) + `, "title": "Make"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := events.Add(value, time.Now()); err != nil {
		t.Fatal(err)
	}
	s := &shown{}
	return &Bar{store: events, commands: commands, showBlock: s.show}, s, runID(1, command)
}

// reportBlocks renders the event's report as the window would, with its Clicks.
func reportBlocks(t *testing.T, b *Bar) (string, *components.Clicks) {
	t.Helper()
	event, _ := b.store.Get("build")
	clicks := &components.Clicks{}
	var html strings.Builder
	ctx := components.WithClicks(context.Background(), clicks)
	if err := components.ReportContent((&report{bar: b, event: event}).blocks(event.Report)).Render(ctx, &html); err != nil {
		t.Fatal(err)
	}
	return html.String(), clicks
}

// A run block's button runs its command, and the report shows its output, stdout and
// stderr, as it comes, and how it ended. Opened again, the report shows the last run;
// run again, the run is replaced.
func TestRun(t *testing.T) {
	b, s, id := runBar(t, true, "echo out; echo err >&2; exit 3")
	html, clicks := reportBlocks(t, b)
	if !strings.Contains(html, "Not run") || strings.Contains(html, "<time") {
		t.Errorf("before it's run, the report shows:\n%s", html)
	}
	click := clicks.Click("block/" + id + "/run")
	if click == nil {
		t.Fatalf("the run's button isn't clickable; the Clicks are %q", clicks.IDs())
	}
	click(components.ClickEvent{})
	eventually(t, "the run to end", func() bool { return strings.Contains(s.get(id), `data-state="failed"`) })
	shownHTML := s.get(id)
	for _, want := range []string{"out\nerr\n", "exit status 3 after", "<time", "Make</button>"} {
		if !strings.Contains(shownHTML, want) {
			t.Errorf("missing %q in what was shown:\n%s", want, shownHTML)
		}
	}

	// Opened again, the report has the last run, and can run it again.
	html, clicks = reportBlocks(t, b)
	if !strings.Contains(html, "out\nerr\n") || !strings.Contains(html, "exit status 3 after") {
		t.Errorf("opened again, the report shows:\n%s", html)
	}
	key := runKey{"build", "echo out; echo err >&2; exit 3"}
	b.runs.mu.Lock()
	first := b.runs.last[key]
	b.runs.mu.Unlock()
	clicks.Click("block/" + id + "/run")(components.ClickEvent{})
	eventually(t, "the second run to end", func() bool {
		b.runs.mu.Lock()
		defer b.runs.mu.Unlock()
		r := b.runs.last[key]
		return r != first && !r.running()
	})
	if html, _ := reportBlocks(t, b); strings.Count(html, "out\nerr\n") != 1 {
		t.Errorf("run again, the output wasn't replaced:\n%s", html)
	}
}

// While a run's command runs, its button is disabled, and starting it again does
// nothing, but it can be stopped; a command that passes says so.
func TestRunWhileRunning(t *testing.T) {
	b, s, id := runBar(t, true, "echo started; sleep 0.5")
	_, clicks := reportBlocks(t, b)
	clicks.Click("block/" + id + "/run")(components.ClickEvent{})
	eventually(t, "the output", func() bool { return strings.Contains(s.get(id), "started\n") })
	html, clicks := reportBlocks(t, b)
	if !strings.Contains(html, `data-state="running"`) || !strings.Contains(html, "disabled") ||
		!slices.Equal(clicks.IDs(), []string{"block/" + id + "/run%2Fstop"}) {
		t.Errorf("while running, the report shows (with clicks %q):\n%s", clicks.IDs(), html)
	}
	b.startRun(runKey{"build", "echo started; sleep 0.5"})
	eventually(t, "the run to pass", func() bool { return strings.Contains(s.get(id), `data-state="passed"`) })
	if html := s.get(id); strings.Count(html, "started") != 1 || !strings.Contains(html, "exit status 0 after") {
		t.Errorf("a second start while running changed the run:\n%s", html)
	}
}

// Stopping a run stops its command and whatever that started, and says so; it can
// then run again, and can't be stopped.
func TestRunStop(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "finished")
	b, s, id := runBar(t, true, "echo started; (sleep 1; touch "+marker+") & wait")
	_, clicks := reportBlocks(t, b)
	clicks.Click("block/" + id + "/run")(components.ClickEvent{})
	eventually(t, "the output", func() bool { return strings.Contains(s.get(id), "started\n") })
	_, clicks = reportBlocks(t, b)
	clicks.Click("block/" + id + "/run%2Fstop")(components.ClickEvent{})
	eventually(t, "the run to stop", func() bool { return strings.Contains(s.get(id), "stopped after") })
	if html := s.get(id); !strings.Contains(html, `data-state="failed"`) {
		t.Errorf("stopped, the run shows:\n%s", html)
	}
	if _, clicks := reportBlocks(t, b); !slices.Equal(clicks.IDs(), []string{"block/" + id + "/run"}) {
		t.Errorf("stopped, the clicks are %q", clicks.IDs())
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("what the command started kept running")
	}
}

// Without commands allowed, a run's buttons are disabled.
func TestRunWithoutCommands(t *testing.T) {
	b, _, _ := runBar(t, false, "ls")
	if html, clicks := reportBlocks(t, b); !strings.Contains(html, "disabled") || len(clicks.IDs()) != 0 {
		t.Errorf("without commands, the report shows (with clicks %q):\n%s", clicks.IDs(), html)
	}
}

// Only the last of a long output is kept, from the start of a line.
func TestRunOutputCut(t *testing.T) {
	b := &Bar{}
	r := &run{redrawing: true} // so it isn't drawn
	out := runOutput{b, runKey{"x", "y"}, r}
	line := strings.Repeat("x", 99) + "\n"
	for range maxOutput/len(line) + 10 {
		out.Write([]byte(line))
	}
	b.runs.mu.Lock()
	defer b.runs.mu.Unlock()
	if !r.cut || len(r.output) > maxOutput || !strings.HasPrefix(string(r.output), line) {
		t.Errorf("cut %v, kept %d bytes, starting %q", r.cut, len(r.output), r.output[:10])
	}
}
