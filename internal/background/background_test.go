package background

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary stands in for ledge: run as `<test binary> run`, with
// BACKGROUND_TEST_MODE set, it acts as a ledge whose API answers on
// BACKGROUND_TEST_PORT. The mode "crash" exits at once; "stubborn" ignores SIGTERM.
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
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	listener, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("BACKGROUND_TEST_PORT"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"events": []}`))
	}))
	fmt.Println("fake ledge up, args:", strings.Join(os.Args[1:], " "))
	for range signals {
		if mode != "stubborn" {
			os.Exit(0)
		}
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func newProcess(t *testing.T, mode string) *Process {
	t.Helper()
	port := freePort(t)
	t.Setenv("BACKGROUND_TEST_MODE", mode)
	t.Setenv("BACKGROUND_TEST_PORT", strconv.Itoa(port))
	p, err := New(t.TempDir(), port)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if pid, ok := p.Running(); ok {
			syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	return p
}

func TestStartStop(t *testing.T) {
	p := newProcess(t, "normal")
	if stopped, err := p.Stop(); stopped || err != nil {
		t.Fatalf("Stop before Start = %v, %v", stopped, err)
	}
	pid, started, err := p.Start([]string{"--headless"})
	if err != nil || !started {
		t.Fatalf("Start = %d, %v, %v", pid, started, err)
	}
	if running, ok := p.Running(); !ok || running != pid {
		t.Errorf("Running = %d, %v; want %d", running, ok, pid)
	}
	if again, started, err := p.Start(nil); started || err != nil || again != pid {
		t.Errorf("a second Start = %d, %v, %v; want the same pid and nothing started", again, started, err)
	}
	if log, _ := os.ReadFile(p.LogFile); !strings.Contains(string(log), "args: run --headless") {
		t.Errorf("log = %q", log)
	}
	if stopped, err := p.Stop(); !stopped || err != nil {
		t.Fatalf("Stop = %v, %v", stopped, err)
	}
	if alive(pid) {
		t.Error("still running after Stop")
	}
	if _, err := os.Stat(p.PIDFile); !os.IsNotExist(err) {
		t.Error("the pid file is still there")
	}
}

func TestStartReportsACrash(t *testing.T) {
	p := newProcess(t, "crash")
	_, _, err := p.Start(nil)
	if err == nil || !strings.Contains(err.Error(), "config: no such file") {
		t.Fatalf("Start = %v, want the log in the error", err)
	}
	if _, err := os.Stat(p.PIDFile); !os.IsNotExist(err) {
		t.Error("left a pid file for a process that exited")
	}
}

func TestStartRefusesATakenPort(t *testing.T) {
	p := newProcess(t, "normal")
	other := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"events": []}`))
	})}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p.Port))
	if err != nil {
		t.Fatal(err)
	}
	go other.Serve(listener)
	defer other.Close()
	if _, _, err := p.Start(nil); err == nil || !strings.Contains(err.Error(), "already answers") {
		t.Fatalf("Start = %v", err)
	}
}

func TestStopKillsAStubbornProcess(t *testing.T) {
	StopTimeout = 500 * time.Millisecond
	defer func() { StopTimeout = 5 * time.Second }()
	p := newProcess(t, "stubborn")
	pid, _, err := p.Start(nil)
	if err != nil {
		t.Fatal(err)
	}
	if stopped, err := p.Stop(); !stopped || err == nil || !strings.Contains(err.Error(), "killed") {
		t.Fatalf("Stop = %v, %v", stopped, err)
	}
	time.Sleep(100 * time.Millisecond)
	if alive(pid) {
		t.Error("still running after the kill")
	}
}

func TestReusedPIDIsLeftAlone(t *testing.T) {
	p := newProcess(t, "normal")
	// A pid file naming a process that isn't this ledge: the test itself.
	if err := os.WriteFile(p.PIDFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Running(); ok {
		t.Fatal("took another process for the background ledge")
	}
	if stopped, err := p.Stop(); stopped || err != nil {
		t.Errorf("Stop = %v, %v; want it left alone", stopped, err)
	}
	// It may be another build's ledge, so its pid file stays.
	if _, err := os.Stat(p.PIDFile); err != nil {
		t.Errorf("Stop removed the pid file of a live process: %v", err)
	}
}

func TestStaleFileIsRemoved(t *testing.T) {
	p := newProcess(t, "crash")
	// A pid that's no longer a process: a child that has exited.
	child := exec.Command("true")
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.PIDFile, []byte(strconv.Itoa(child.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	p.Stop()
	if _, err := os.Stat(p.PIDFile); !os.IsNotExist(err) {
		t.Error("Stop kept a pid file whose process is gone")
	}
}

func TestLastArgs(t *testing.T) {
	p := newProcess(t, "normal")
	if _, ok, err := p.LastArgs(); ok || err != nil {
		t.Fatalf("LastArgs before any Start = %v, %v", ok, err)
	}
	if _, _, err := p.Start([]string{"--config", "/a path/with spaces.json", "--headless"}); err != nil {
		t.Fatal(err)
	}
	p.Stop()
	// Kept after stopping, so a restart of a stopped ledge can use them.
	args, ok, err := p.LastArgs()
	if err != nil || !ok || strings.Join(args, "|") != "--config|/a path/with spaces.json|--headless" {
		t.Errorf("LastArgs = %q, %v, %v", args, ok, err)
	}
	if _, _, err := p.Start(nil); err != nil {
		t.Fatal(err)
	}
	if args, ok, _ := p.LastArgs(); !ok || len(args) != 0 {
		t.Errorf("after a Start with no args, LastArgs = %q, %v", args, ok)
	}
}
