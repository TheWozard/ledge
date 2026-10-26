package bar

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/TheWozard/ledge/internal/store"
)

// web is the urls a status's card opens: only http(s) ones make it clickable, and
// open checks again.
var web = regexp.MustCompile(`(?i)^https?://`)

// open is what clicking a card does: show the event's report, or open its url.
func (b *Bar) open(id string) {
	event, ok := b.store.Get(id)
	if !ok {
		return
	}
	target, _ := event["url"].(string)
	switch {
	case event["kind"] == "status":
		if web.MatchString(target) {
			b.openURL(target)
		}
	case event["report"] != nil:
		b.showReport(event)
	case target != "":
		b.openURL(target)
	}
}

// openURL runs `open <url>`, so the url opens in whatever app handles it. The event
// schema makes a url start with a scheme, so it can't be taken for an option.
func (b *Bar) openURL(target string) {
	if err := b.start(exec.Command("open", target)); err != nil {
		log.Printf("ledge: opening %s: %v", target, err)
	}
}

// run starts the command of the status with this id: with `bash -lc`, so it gets
// your login PATH, in $HOME, with its output appended to commands.log. It runs in its
// own session, so stopping ledge doesn't stop it. Nothing tracks whether it worked.
func (b *Bar) run(id string) {
	event, ok := b.store.Get(id)
	command, _ := event["command"].(string)
	if !ok || command == "" {
		return
	}
	if err := b.runCommand(event, command); err != nil {
		log.Printf("ledge: running the command of %s: %v", id, err)
	}
}

func (b *Bar) runCommand(event store.Event, command string) error {
	if err := os.MkdirAll(b.stateDir, 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(filepath.Join(b.stateDir, "commands.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer output.Close() // the command has its own copy once started
	fmt.Fprintf(output, "\n--- %s %s (%s): %s\n", time.Now().Format("2006-01-02T15:04:05"), event["title"], event["id"], command)
	cmd := exec.Command("/bin/bash", "-lc", command)
	cmd.Dir, _ = os.UserHomeDir()
	cmd.Stdout, cmd.Stderr = output, output
	return b.start(cmd)
}

// startDetached starts cmd in its own session, so it outlives ledge, and reaps it
// when it exits.
func startDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
