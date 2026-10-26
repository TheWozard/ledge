package ui

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
)

// fakeNative stands in for the windows in the tests that don't open any: it records
// what each window is loaded with and what's run in it, which windows are showing,
// the native menus popped up and the links opened. It measures a theme's menu as
// 120 by 40, and chooses from a native menu what choose says, or nothing.
type fakeNative struct {
	mu      sync.Mutex
	loaded  map[int]string   // the page each window was last loaded with
	loads   map[int]int      // how many times each window has been loaded
	evals   map[int][]string // what's been run in each window since it was loaded
	visible map[int]bool
	menus   [][]shim.MenuItem
	choose  func(items []shim.MenuItem) shim.Chosen
	urls    []string
}

func (f *fakeNative) LoadHTML(window int, html string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loaded == nil {
		f.loaded, f.loads = map[int]string{}, map[int]int{}
	}
	f.loaded[window] = html
	f.loads[window]++
	delete(f.evals, window)
}

func (f *fakeNative) Eval(window int, js string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.evals == nil {
		f.evals = map[int][]string{}
	}
	f.evals[window] = append(f.evals[window], js)
}

func (f *fakeNative) EvalResult(window int, js string) (json.RawMessage, error) {
	if window == MenuWindow && js == measureMenu {
		return json.RawMessage(`[120, 40]`), nil
	}
	return nil, errors.New("the fake runs no JS")
}

func (f *fakeNative) CreateWindow(int, string, float64, float64) {}
func (f *fakeNative) SetTitle(int, string)                       {}
func (f *fakeNative) SetAppearance(int, string)                  {}
func (f *fakeNative) SetFrameless(int, bool)                     {}
func (f *fakeNative) ShowWindow(window int)                      { f.show(window, true) }
func (f *fakeNative) CreatePopup(int)                            {}
func (f *fakeNative) ShowPopup(window int, _, _ float64)         { f.show(window, true) }
func (f *fakeNative) HidePopup(window int)                       { f.show(window, false) }

func (f *fakeNative) show(window int, visible bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.visible == nil {
		f.visible = map[int]bool{}
	}
	f.visible[window] = visible
}

func (f *fakeNative) ShowMenu(items []shim.MenuItem) shim.Chosen {
	f.mu.Lock()
	f.menus = append(f.menus, items)
	choose := f.choose
	f.mu.Unlock()
	if choose == nil {
		return shim.Chosen{Index: -1}
	}
	return choose(items)
}

func (f *fakeNative) OpenURL(url string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.urls = append(f.urls, url)
}

// page is what window was last loaded with, or "".
func (f *fakeNative) page(window int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loaded[window]
}

// loadCount is how many times window has been loaded.
func (f *fakeNative) loadCount(window int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loads[window]
}

// ran is what's been run in window since it was last loaded, in order.
func (f *fakeNative) ran(window int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.evals[window])
}

// last is the last thing run in window, or "".
func (f *fakeNative) last(window int) string {
	if ran := f.ran(window); len(ran) > 0 {
		return ran[len(ran)-1]
	}
	return ""
}

func (f *fakeNative) showing(window int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.visible[window]
}

func (f *fakeNative) opened() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.urls)
}

// fakeBar is a bar in th, its windows a fakeNative, with its page ready, as a page
// says once it's loaded.
func fakeBar(t *testing.T, th theme.Theme) (*Bar, *fakeNative) {
	t.Helper()
	f := &fakeNative{}
	b := newBar(Options{Theme: th}, f)
	post(b, BarWindow, `{"action": "ready"}`)
	return b, f
}

// post is window's page posting message, which is carried out before it returns: a
// Click it calls has run.
func post(b *Bar, window int, message string) { b.received(window, []byte(message)) }

// defaultTheme is the built-in default theme, which draws a card's menu itself.
func defaultTheme(t *testing.T) theme.Theme {
	t.Helper()
	th, err := theme.Builtin(theme.Builtins.Default())
	if err != nil {
		t.Fatal(err)
	}
	return th
}
