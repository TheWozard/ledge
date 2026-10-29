package bar

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/TheWozard/ledge-frontend/components"

	"github.com/TheWozard/ledge/internal/store"
)

// maxOutput is how much of a run's output is kept: the last of it, from the start of
// a line.
const maxOutput = 256 << 10

// redrawEvery is how often a running command's output is drawn again, at most.
const redrawEvery = 100 * time.Millisecond

// runs is the last run of each event's run blocks' commands, kept while ledge runs,
// so a report opened again shows them.
type runs struct {
	mu   sync.Mutex
	last map[runKey]*run
}

// runKey is a run block's: its event's id and its command. Two blocks with the same
// command in a report share a run.
type runKey struct{ event, command string }

// run is a command's run: when it started and ended, how it ended, and what it
// printed, stdout and stderr together. Its fields are guarded by runs.mu.
type run struct {
	started, ended time.Time
	err            error       // how it ended, if not with status 0
	process        *os.Process // the command's bash, once it's started
	stopped        bool        // whether Stop was clicked
	output         []byte
	cut            bool // whether the start of output was dropped
	redrawing      bool // whether a redraw is due
}

// running is whether the run hasn't ended.
func (r *run) running() bool { return r.ended.IsZero() }

// runView is the event's run block as the report shows it: its command's last run,
// a button that runs it again, unless it's running, and one that stops it while it
// is. Without commands allowed, neither does anything.
func (b *Bar) runView(event store.Event, block store.Block) components.Run {
	key := runKey{event.ID, block.Command}
	view := components.Run{ID: "run", Title: block.Title}
	b.runs.mu.Lock()
	defer b.runs.mu.Unlock()
	r := b.runs.last[key]
	switch {
	case !b.commands:
	case r == nil || !r.running():
		view.Click = func(components.ClickEvent) { b.startRun(key) }
	default:
		view.Stop = func(components.ClickEvent) { b.stopRun(key) }
	}
	if r == nil {
		return view
	}
	view.Started, view.Output = r.started, string(r.output)
	if r.cut {
		view.Output = "[…]\n" + view.Output
	}
	took := r.ended.Sub(r.started).Round(100 * time.Millisecond)
	switch {
	case r.running():
		view.State = "running"
	case r.stopped:
		view.State, view.Status = "failed", fmt.Sprintf("stopped after %v", took)
	case r.err == nil:
		view.State, view.Status = "passed", fmt.Sprintf("exit status 0 after %v", took)
	default:
		view.State, view.Status = "failed", fmt.Sprintf("%v after %v", r.err, took)
	}
	return view
}

// runID is the id of the Block of the run block at index i in a report, with command.
// It has the command in it, so a report window still showing an older report, with
// another command there, isn't sent this one's run.
func runID(i int, command string) string {
	sum := sha256.Sum256([]byte(command))
	return fmt.Sprintf("run-%d-%x", i, sum[:6])
}

// startRun starts key's command with `bash -lc` in $HOME, as an event's command runs,
// replacing its last run, unless it's running. Its output goes to the event's report
// as it comes, and how it ended once it has. It runs in a process group of its own,
// for stopRun to stop it and whatever it started.
func (b *Bar) startRun(key runKey) {
	r := &run{started: time.Now()}
	b.runs.mu.Lock()
	if last := b.runs.last[key]; last != nil && last.running() {
		b.runs.mu.Unlock()
		return
	}
	if b.runs.last == nil {
		b.runs.last = map[runKey]*run{}
	}
	b.runs.last[key] = r
	b.runs.mu.Unlock()
	b.redrawRun(key)

	cmd := exec.Command("/bin/bash", "-lc", key.command)
	cmd.Dir, _ = os.UserHomeDir()
	out := runOutput{b, key, r}
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		b.endRun(key, r, err)
		return
	}
	b.runs.mu.Lock()
	r.process = cmd.Process
	stopped := r.stopped
	b.runs.mu.Unlock()
	if stopped { // Stop was clicked as it started
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	go func() { b.endRun(key, r, cmd.Wait()) }()
}

// stopRun stops key's run, if it's running: SIGTERM to its process group.
func (b *Bar) stopRun(key runKey) {
	b.runs.mu.Lock()
	defer b.runs.mu.Unlock()
	r := b.runs.last[key]
	if r == nil || !r.running() || r.stopped {
		return
	}
	r.stopped = true
	if r.process != nil {
		syscall.Kill(-r.process.Pid, syscall.SIGTERM)
	}
}

// endRun has r, key's run, end with err, and shows how.
func (b *Bar) endRun(key runKey, r *run, err error) {
	b.runs.mu.Lock()
	r.ended, r.err = time.Now(), err
	b.runs.mu.Unlock()
	b.redrawRun(key)
}

// runOutput is where a run's command writes: its output, and a redraw of it soon. It
// has one writer at a time, as os/exec gives stdout and stderr the same one.
type runOutput struct {
	bar *Bar
	key runKey
	run *run
}

func (o runOutput) Write(p []byte) (int, error) {
	o.bar.runs.mu.Lock()
	defer o.bar.runs.mu.Unlock()
	r := o.run
	r.output = append(r.output, p...)
	if len(r.output) > maxOutput {
		kept := r.output[len(r.output)-maxOutput:]
		for i, c := range kept {
			if c == '\n' {
				kept = kept[i+1:]
				break
			}
		}
		r.output, r.cut = append([]byte(nil), kept...), true
	}
	if !r.redrawing {
		r.redrawing = true
		time.AfterFunc(redrawEvery, func() {
			o.bar.runs.mu.Lock()
			r.redrawing = false
			o.bar.runs.mu.Unlock()
			o.bar.redrawRun(o.key)
		})
	}
	return len(p), nil
}

// redrawRun shows key's run in its event's report, if it's open, in each run block
// with its command.
func (b *Bar) redrawRun(key runKey) {
	event, ok := b.store.Get(key.event)
	if !ok {
		return
	}
	for i, block := range event.Report {
		if block.Type != "run" || block.Command != key.command {
			continue
		}
		content := components.RunBlock(b.runView(event, block))
		if _, err := b.showBlock(event.ID, components.Block{ID: runID(i, block.Command), Content: content}); err != nil {
			log.Printf("showing the run of %s: %v", event.ID, err)
		}
	}
}
