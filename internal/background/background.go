// Package background runs `ledge run` as a background process, and stops it: what
// `ledge start`, `ledge stop` and `ledge restart` do. The process's pid goes in a pid
// file, its output in a log, and the arguments it was started with in an args file,
// all in the state dir.
//
// A pid is only acted on while its process is still `<this binary> run`, so a stale
// pid file whose pid has been reused by something else is never signalled.
package background

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/TheWozard/ledge/internal/api"
	"github.com/TheWozard/ledge/internal/state"
)

// How long start waits for the API to answer, and stop for the process to exit
// before killing it.
var (
	StartTimeout = 10 * time.Second
	StopTimeout  = 5 * time.Second
)

// Process is a background ledge: the binary that runs it and the files it keeps.
type Process struct {
	Binary   string // absolute path of the ledge binary
	PIDFile  string
	LogFile  string
	ArgsFile string // the arguments of the last Start, for restart
	Socket   string // where its API answers once it's up
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
		Binary:   binary,
		PIDFile:  filepath.Join(dir, "ledge.pid"),
		LogFile:  filepath.Join(dir, "ledge.log"),
		ArgsFile: filepath.Join(dir, "ledge.args.json"),
		Socket:   state.Socket(dir),
	}, nil
}

// recordedPID is the pid in the pid file, whatever it now is.
func (p *Process) recordedPID() (int, bool) {
	data, err := os.ReadFile(p.PIDFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid, err == nil && pid > 0
}

// Running is the pid of the background ledge, if it's still running.
func (p *Process) Running() (int, bool) {
	pid, ok := p.recordedPID()
	if !ok {
		return 0, false
	}
	command, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(command)), p.Binary+" run") {
		return 0, false
	}
	return pid, true
}

// Start runs `<binary> run args...` in the background and waits for its API to answer.
// It does nothing if it's already running, and refuses if something else already
// answers on the socket. If the process exits first, the error carries its log.
func (p *Process) Start(args []string) (pid int, started bool, err error) {
	if pid, ok := p.Running(); ok {
		return pid, false, nil
	}
	// Another ledge (run in a terminal, or by another build) would answer the readiness
	// check below and make a failed start look like a success.
	if p.answers() {
		return 0, false, fmt.Errorf("something not started by `ledge start` already answers at %s; stop it first", p.Socket)
	}
	if err := os.MkdirAll(filepath.Dir(p.PIDFile), 0o755); err != nil {
		return 0, false, err
	}
	log, err := os.Create(p.LogFile)
	if err != nil {
		return 0, false, err
	}
	defer log.Close()
	home, _ := os.UserHomeDir()
	cmd := exec.Command(p.Binary, append([]string{"run"}, args...)...)
	cmd.Dir = home
	cmd.Stdout, cmd.Stderr = log, log
	// Its own session, so it outlives the terminal that started it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, false, err
	}
	pid = cmd.Process.Pid
	if err := os.WriteFile(p.PIDFile, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		cmd.Process.Kill()
		return 0, false, err
	}
	if err := p.saveArgs(args); err != nil {
		cmd.Process.Kill()
		os.Remove(p.PIDFile)
		return 0, false, err
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()

	for deadline := time.Now().Add(StartTimeout); time.Now().Before(deadline); {
		if p.answers() {
			return pid, true, nil
		}
		select {
		case <-exited:
			os.Remove(p.PIDFile)
			output, _ := os.ReadFile(p.LogFile)
			return 0, false, fmt.Errorf("ledge exited before its API answered:\n%s", strings.TrimRight(string(output), "\n"))
		case <-time.After(100 * time.Millisecond):
		}
	}
	return pid, true, fmt.Errorf("started (pid %d) but the API isn't answering at %s yet; see %s", pid, p.Socket, p.LogFile)
}

// Stop ends the background ledge: SIGTERM, then SIGKILL if it hasn't exited after
// StopTimeout. If this binary didn't start one, it asks whatever ledge answers on the
// socket to shut down instead, such as one run in a terminal or by another build.
// Returns false if nothing was running.
func (p *Process) Stop() (bool, error) {
	pid, ok := p.Running()
	if !ok {
		stopped, err := p.stopByAPI()
		// The pid file is only stale if its process is gone. A live one may be a ledge
		// started by another build of ledge, whose pid file this is too.
		if recorded, ok := p.recordedPID(); !ok || !alive(recorded) {
			os.Remove(p.PIDFile)
		}
		return stopped, err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return false, err
	}
	err = await(pid)
	os.Remove(p.PIDFile)
	return true, err
}

// stopByAPI asks the ledge answering on the socket to shut down, and waits StopTimeout
// for its API to stop answering. Returns false if nothing answers.
func (p *Process) stopByAPI() (bool, error) {
	if !p.answers() {
		return false, nil
	}
	var reply map[string]any
	if err := (api.Client{Socket: p.Socket}).Call("POST", "/shutdown", []byte("{}"), &reply); err != nil {
		return false, fmt.Errorf("asking the ledge at %s to shut down: %w", p.Socket, err)
	}
	for deadline := time.Now().Add(StopTimeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if !p.answers() {
			return true, nil
		}
	}
	return true, fmt.Errorf("the ledge at %s still answers %s after being asked to shut down", p.Socket, StopTimeout)
}

// await waits StopTimeout for pid to exit, then kills it.
func await(pid int) error {
	for deadline := time.Now().Add(StopTimeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if !alive(pid) {
			return nil
		}
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return fmt.Errorf("pid %d didn't exit after %s, so it was killed", pid, StopTimeout)
}

// LastArgs is what the last Start passed to `ledge run`, or false if nothing has
// been started yet.
func (p *Process) LastArgs() ([]string, bool, error) {
	data, err := os.ReadFile(p.ArgsFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		return nil, false, fmt.Errorf("%s: %w", p.ArgsFile, err)
	}
	return args, true, nil
}

func (p *Process) saveArgs(args []string) error {
	if args == nil {
		args = []string{}
	}
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return os.WriteFile(p.ArgsFile, append(data, '\n'), 0o644)
}

func (p *Process) answers() bool {
	var reply struct {
		Events []any `json:"events"`
	}
	return api.Client{Socket: p.Socket}.Call("GET", "/events", nil, &reply) == nil
}

// alive reports whether pid is still a process (a zombie counts as gone).
func alive(pid int) bool {
	state, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "state=").Output()
	return err == nil && !strings.HasPrefix(strings.TrimSpace(string(state)), "Z")
}
