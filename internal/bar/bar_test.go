package bar

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/ui"

	"github.com/TheWozard/ledge/internal/config"
	"github.com/TheWozard/ledge/internal/schema"
	"github.com/TheWozard/ledge/internal/store"
)

// The tests that open the bar only run with LEDGE_GUI_TEST=1; the rest always run.
// They check that the bar's pages reach the store and the actions: how the bar looks
// and behaves is ledge-frontend's to test.
var (
	testBar   *Bar
	testStore *store.Store
	testDir   string
	opened    recorder // what `open` would have opened
)

// recorder keeps the urls `open` was run with. Tests never open the browser.
type recorder struct {
	mu   sync.Mutex
	urls []string
}

func (r *recorder) has(url string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.urls {
		if u == url {
			return true
		}
	}
	return false
}

// testStart runs a status's bash command for real, but only records `open`.
func testStart(cmd *exec.Cmd) error {
	if filepath.Base(cmd.Path) == "open" {
		opened.mu.Lock()
		opened.urls = append(opened.urls, cmd.Args[1])
		opened.mu.Unlock()
		return nil
	}
	return startDetached(cmd)
}

func TestMain(m *testing.M) {
	if os.Getenv("LEDGE_GUI_TEST") == "" {
		fmt.Println("skipping the tests that open the bar; set LEDGE_GUI_TEST=1 to run them")
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "ledge-bar-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	testDir = dir
	if testStore, err = store.Open(filepath.Join(dir, "events.json")); err != nil {
		log.Fatal(err)
	}

	// The main loop has the main goroutine, so the tests run beside it.
	ui.Init()
	if testBar, err = Open(testStore, config.Defaults, dir); err != nil {
		log.Fatal(err)
	}
	testBar.start = testStart
	code := make(chan int, 1)
	go func() {
		code <- m.Run()
		ui.Stop()
	}()
	ui.Run()
	os.Exit(<-code)
}

func gui(t *testing.T) {
	t.Helper()
	if testBar == nil {
		t.Skip("opens the bar; set LEDGE_GUI_TEST=1")
	}
}

func evalIn(t *testing.T, window int, js string) string {
	t.Helper()
	value, err := shim.EvalResult(window, js)
	if err != nil {
		t.Fatalf("%s: %v", js, err)
	}
	return string(value)
}

// waitIn evaluates js in the window until it gives want.
func waitIn(t *testing.T, window int, js, want string) {
	t.Helper()
	var got string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if value, err := shim.EvalResult(window, js); err == nil {
			if got = string(value); got == want {
				return
			}
		}
	}
	t.Fatalf("%s = %s, want %s", js, got, want)
}

// eventually waits for check to be true.
func eventually(t *testing.T, what string, check func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if check() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// add adds an event for the length of the test, and shows it.
func add(t *testing.T, body string) {
	t.Helper()
	value, err := schema.Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	id, err := testStore.Add(value, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	testBar.Render()
	t.Cleanup(func() {
		testStore.Remove(id)
		testBar.Render()
	})
}

// click clicks the element that find (a JS expression) gives in the bar, once the bar
// shows it.
func click(t *testing.T, find string) {
	t.Helper()
	waitIn(t, ui.BarWindow, `!!(`+find+`)`, "true")
	evalIn(t, ui.BarWindow, `(`+find+`).click(), true`)
}

// cardJS is a JS expression for the card with this title.
func cardJS(title string) string {
	return fmt.Sprintf(`[...document.querySelectorAll("li")].find(li => li.querySelector(".title").textContent === %q)`, title)
}

func TestDismiss(t *testing.T) {
	gui(t)
	add(t, `{"id": "gone", "title": "Dismiss me"}`)
	click(t, cardJS("Dismiss me")+`?.querySelector(".dismiss")`)
	waitIn(t, ui.BarWindow, `!!`+cardJS("Dismiss me"), "false")
	if _, ok := testStore.Get("gone"); ok {
		t.Error("dismissed in the page but still in the store")
	}
}
