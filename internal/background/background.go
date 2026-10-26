// Package background runs `ledge run` as a background process, and stops it: what
// `ledge start`, `ledge stop` and `ledge restart` do. The ledge of a state dir is
// whatever answers on its socket, however it was started, so that's what's asked: if
// one is running, its status, and to shut down. Its output goes in a log, and the
// options it was last started with in an options file, both in the state dir.
package background

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/config"
	"github.com/TheWozard/ledge/internal/state"
)

// How long start waits for the API to answer, and stop for it to stop answering
// before killing it.
var (
	StartTimeout = 10 * time.Second
	StopTimeout  = 5 * time.Second
)

// Process is a background ledge: the binary that runs it and the files it keeps.
type Process struct {
	Binary      string // absolute path of the ledge binary
	LogFile     string
	OptionsFile string // the options of the last Start, for restart
	Socket      string // where its API answers once it's up
}

// New is the background ledge for this binary, keeping its files in dir, the state
// directory.
func New(dir string) (*Process, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if binary, err = filepath.EvalSymlinks(binary); err != nil {
		return nil, err
	}
	return &Process{
		Binary:      binary,
		LogFile:     filepath.Join(dir, "ledge.log"),
		OptionsFile: filepath.Join(dir, "ledge.options.json"),
		Socket:      state.Socket(dir),
	}, nil
}

// Status is the status of the ledge answering on the socket, or false if none does.
func (p *Process) Status() (api.Status, bool) {
	var status api.Status
	err := api.Client{Socket: p.Socket}.Call("GET", "/status", nil, &status)
	return status, err == nil
}

// Start runs `<binary> run` with options in the background and waits for its API to
// answer. If a ledge already answers, it does nothing and returns that one's pid. If
// the process exits first, the error carries its log.
func (p *Process) Start(options api.Options) (pid int, started bool, err error) {
	if status, ok := p.Status(); ok {
		return status.PID, false, nil
	}
	if err := state.Make(filepath.Dir(p.LogFile)); err != nil {
		return 0, false, err
	}
	log, err := os.OpenFile(p.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, false, err
	}
	defer log.Close()
	home, _ := os.UserHomeDir()
	cmd := exec.Command(p.Binary, append([]string{"run"}, options.Args()...)...)
	cmd.Dir = home
	// The options are all it runs with: $LEDGE_CONFIG has been read into them already,
	// and --config "" is none, which the variable mustn't undo.
	cmd.Env = slices.DeleteFunc(os.Environ(), func(v string) bool { return strings.HasPrefix(v, config.Env+"=") })
	cmd.Stdout, cmd.Stderr = log, log
	// Its own session, so it outlives the terminal that started it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, false, err
	}
	pid = cmd.Process.Pid
	if err := p.saveOptions(options); err != nil {
		cmd.Process.Kill()
		return 0, false, err
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()

	for deadline := time.Now().Add(StartTimeout); time.Now().Before(deadline); {
		if _, ok := p.Status(); ok {
			return pid, true, nil
		}
		select {
		case <-exited:
			output, _ := os.ReadFile(p.LogFile)
			return 0, false, fmt.Errorf("ledge exited before its API answered:\n%s", strings.TrimRight(string(output), "\n"))
		case <-time.After(100 * time.Millisecond):
		}
	}
	return pid, true, fmt.Errorf("started (pid %d) but the API isn't answering at %s yet; see %s", pid, p.Socket, p.LogFile)
}

// Stop asks the ledge answering on the socket to shut down, and waits StopTimeout for
// it to stop answering. One that doesn't is killed, by the pid it gave. Returns false
// if nothing was running.
func (p *Process) Stop() (bool, error) {
	status, ok := p.Status()
	if !ok {
		return false, nil
	}
	var reply map[string]any
	asked := api.Client{Socket: p.Socket}.Call("POST", "/shutdown", nil, &reply)
	if asked == nil && p.await(StopTimeout) {
		return true, nil
	}
	// A pid of 1, or this process's, can't be a ledge to kill.
	if status.PID <= 1 || status.PID == os.Getpid() {
		return true, fmt.Errorf("the ledge at %s still answers %s after being asked to shut down", p.Socket, StopTimeout)
	}
	if err := syscall.Kill(status.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return true, err
	}
	p.await(StopTimeout)
	return true, fmt.Errorf("pid %d didn't shut down after %s, so it was killed", status.PID, StopTimeout)
}

// await waits up to timeout for the socket to stop answering. Returns false if it
// still answers.
func (p *Process) await(timeout time.Duration) bool {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if _, ok := p.Status(); !ok {
			return true
		}
	}
	return false
}

// LastOptions is what the last Start ran with, or false if nothing has been started
// yet.
func (p *Process) LastOptions() (api.Options, bool, error) {
	var options api.Options
	data, err := os.ReadFile(p.OptionsFile)
	if errors.Is(err, os.ErrNotExist) {
		return options, false, nil
	}
	if err != nil {
		return options, false, err
	}
	if err := json.Unmarshal(data, &options); err != nil {
		return options, false, fmt.Errorf("%s: %w", p.OptionsFile, err)
	}
	return options, true, nil
}

func (p *Process) saveOptions(options api.Options) error {
	data, err := json.Marshal(options)
	if err != nil {
		return err
	}
	return os.WriteFile(p.OptionsFile, append(data, '\n'), 0o600)
}
