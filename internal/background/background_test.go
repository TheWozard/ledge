package background

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/TheWozard/ledge/internal/api"
)

// The test binary stands in for ledge: run as `<test binary> run`, with
// BACKGROUND_TEST_MODE set, it acts as a ledge whose API answers on the socket
// BACKGROUND_TEST_SOCKET with its status, and exits when POST /shutdown asks. The
// mode "crash" exits at once; "stubborn" replies to POST /shutdown but keeps running.
func TestMain(m *testing.M) {
	if mode := os.Getenv("BACKGROUND_TEST_MODE"); mode != "" && len(os.Args) > 1 && os.Args[1] == "run" {
		fakeLedge(mode)
		return
	}
	os.Exit(m.Run())
}

func fakeLedge(mode string) {
	if mode == "crash" {
		fmt.Println("config: no such file")
		os.Exit(1)
	}
	// As ledge does, so a socket left by an earlier fake that exited is replaced.
	listener, err := api.Listen(os.Getenv("BACKGROUND_TEST_SOCKET"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("fake ledge up, args:", strings.Join(os.Args[1:], " "))
	http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shutdown" {
			w.Write([]byte(`{"shutdown": true}`))
			w.(http.Flusher).Flush()
			if mode != "stubborn" {
				go func() { time.Sleep(50 * time.Millisecond); os.Exit(0) }()
			}
			return
		}
		fmt.Fprintf(w, `{"version": "test", "pid": %d}`, os.Getpid())
	}))
}

// shortTempDir is a temporary directory with a short path, unlike t.TempDir(): a
// socket's path must fit in 104 bytes.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ledge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func newProcess(t *testing.T, mode string) *Process {
	t.Helper()
	p, err := New(shortTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("BACKGROUND_TEST_MODE", mode)
	t.Setenv("BACKGROUND_TEST_SOCKET", p.Socket)
	t.Cleanup(func() {
		if status, ok := p.Status(); ok && status.PID > 1 && status.PID != os.Getpid() {
			syscall.Kill(status.PID, syscall.SIGKILL)
		}
	})
	return p
}

// gone reports whether pid has exited, waiting up to a second.
func gone(pid int) bool {
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		// The test reaps what it starts, so an exited child doesn't linger.
		if syscall.Kill(pid, 0) != nil {
			return true
		}
	}
	return false
}

func TestStartStop(t *testing.T) {
	p := newProcess(t, "normal")
	if stopped, err := p.Stop(); stopped || err != nil {
		t.Fatalf("Stop before Start = %v, %v", stopped, err)
	}
	pid, started, err := p.Start(api.Options{Headless: true})
	if err != nil || !started {
		t.Fatalf("Start = %d, %v, %v", pid, started, err)
	}
	if status, ok := p.Status(); !ok || status.PID != pid {
		t.Errorf("Status = %+v, %v; want pid %d", status, ok, pid)
	}
	if again, started, err := p.Start(api.Options{}); started || err != nil || again != pid {
		t.Errorf("a second Start = %d, %v, %v; want the same pid and nothing started", again, started, err)
	}
	if log, _ := os.ReadFile(p.LogFile); !strings.Contains(string(log), "args: run --headless") {
		t.Errorf("log = %q", log)
	}
	if stopped, err := p.Stop(); !stopped || err != nil {
		t.Fatalf("Stop = %v, %v", stopped, err)
	}
	if !gone(pid) {
		t.Error("still running after Stop")
	}
}

func TestStartReportsACrash(t *testing.T) {
	p := newProcess(t, "crash")
	_, _, err := p.Start(api.Options{})
	if err == nil || !strings.Contains(err.Error(), "config: no such file") {
		t.Fatalf("Start = %v, want the log in the error", err)
	}
}

func TestStopKillsAStubbornLedge(t *testing.T) {
	StopTimeout = 500 * time.Millisecond
	defer func() { StopTimeout = 5 * time.Second }()
	p := newProcess(t, "stubborn")
	pid, _, err := p.Start(api.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if stopped, err := p.Stop(); !stopped || err == nil || !strings.Contains(err.Error(), "killed") {
		t.Fatalf("Stop = %v, %v", stopped, err)
	}
	if !gone(pid) {
		t.Error("still running after the kill")
	}
}

// A ledge Start didn't start, such as one run in a terminal, is found and stopped the
// same way.
func TestAnyLedgeOnTheSocket(t *testing.T) {
	p := newProcess(t, "normal")
	other := exec.Command(os.Args[0], "run")
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { other.Wait(); close(exited) }()
	t.Cleanup(func() { other.Process.Kill() })
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if _, ok := p.Status(); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the other ledge never answered")
		}
	}
	if pid, started, err := p.Start(api.Options{}); started || err != nil || pid != other.Process.Pid {
		t.Errorf("Start = %d, %v, %v; want the other's pid %d and nothing started", pid, started, err, other.Process.Pid)
	}
	if stopped, err := p.Stop(); !stopped || err != nil {
		t.Fatalf("Stop = %v, %v", stopped, err)
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Error("the other ledge didn't exit")
	}
}

func TestLastOptions(t *testing.T) {
	p := newProcess(t, "normal")
	if _, ok, err := p.LastOptions(); ok || err != nil {
		t.Fatalf("LastOptions before any Start = %v, %v", ok, err)
	}
	want := api.Options{Config: "/a path/with spaces.json", Headless: true}
	if _, _, err := p.Start(want); err != nil {
		t.Fatal(err)
	}
	if log, _ := os.ReadFile(p.LogFile); !strings.Contains(string(log), "args: run --config /a path/with spaces.json --headless") {
		t.Errorf("log = %q", log)
	}
	p.Stop()
	// Kept after stopping, so a restart of a stopped ledge can use them.
	if got, ok, err := p.LastOptions(); err != nil || !ok || got != want {
		t.Errorf("LastOptions = %+v, %v, %v; want %+v", got, ok, err, want)
	}
	if _, _, err := p.Start(api.Options{}); err != nil {
		t.Fatal(err)
	}
	if got, ok, _ := p.LastOptions(); !ok || got != (api.Options{}) {
		t.Errorf("after a Start with no options, LastOptions = %+v, %v", got, ok)
	}
}
