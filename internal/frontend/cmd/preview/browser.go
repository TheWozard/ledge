package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/shim"
	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/TheWozard/ledge-frontend/ui"
)

// index is the page at /: a frame for each of the bar's windows, the bar along the
// bottom at its height, the report in front above it at a report window's size, and
// a card's menu, while it's shown, to the left of that at its own size. It runs in
// them what the bar asks.
//
//go:embed index.html
var indexHTML string
var index = template.Must(template.New("index").Parse(indexHTML))

// browserServer serves the sample's bar and its windows in the browser, for its
// inspector. Each load of / opens a new ui.Bar, in the theme as it is then, so an
// edit to its stylesheets shows on a reload; the bar's windows are browserWindows,
// frames of /, so what's clicked, the menus, the reports and the busy marks are the
// bar's own, as on screen. A theme without a menu of its own has its menu's layout
// alone, as the browser has no native menu.
type browserServer struct {
	name   string
	load   func() (theme.Theme, error)
	sample *sample

	mu      sync.Mutex
	opened  int64           // how many bars have been opened, the last one's id
	bar     *ui.Bar         // the last one opened, or nil before any is
	windows *browserWindows // its windows
}

// serveBrowser serves sample on addr, as browserServer says, and opens it in the
// browser.
func serveBrowser(addr, name string, load func() (theme.Theme, error), sample *sample) {
	s := &browserServer{name: name, load: load, sample: sample}
	sample.open = func(id string) {
		if bar := s.current(); bar != nil {
			title, blocks := sample.report(id)
			if err := bar.ShowReport(id, title, blocks); err != nil {
				log.Print(err)
			}
		}
	}
	sample.replace = func(id string, block components.Block) {
		if bar := s.current(); bar != nil {
			if _, err := bar.ShowReportBlock(id, block); err != nil {
				log.Print(err)
			}
		}
	}
	sample.changed = func() {
		if bar := s.current(); bar != nil {
			if err := bar.Render(sample.shown()); err != nil {
				log.Print(err)
			}
		}
	}
	go sample.advance()

	http.HandleFunc("GET /{$}", s.index)
	http.HandleFunc("GET /bar/{bar}/events", s.events)
	http.HandleFunc("GET /bar/{bar}/window/{window}", s.page)
	http.HandleFunc("POST /bar/{bar}/window/{window}/post", s.post)
	http.HandleFunc("POST /bar/{bar}/answer", s.answer)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + listener.Addr().String() + "/"
	log.Printf("showing %s at %s; ctrl-c to quit", name, url)
	if err := exec.Command("open", url).Run(); err != nil {
		log.Print(err)
	}
	log.Fatal(http.Serve(listener, nil))
}

// current is the bar last opened, or nil before any is.
func (s *browserServer) current() *ui.Bar {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bar
}

// open opens a new bar, in the theme as it is now, showing the sample's cards and
// the long report, and returns its id.
func (s *browserServer) open() (int64, error) {
	th, err := s.load()
	if err != nil {
		return 0, err
	}
	if th.Menu == "" {
		th.Menu = "/* The theme has no menu of its own: in the browser, a card's menu has only its layout. */"
	}
	windows := newBrowserWindows()
	bar, err := ui.OpenIn(ui.Options{Edge: "bottom", Height: barHeight, Theme: th}, windows)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.opened++
	id, old := s.opened, s.bar
	s.bar, s.windows = bar, windows
	s.mu.Unlock()
	if old != nil {
		old.Render(nil) // so its timers don't complete the sample's cards as well
	}
	s.sample.changed()
	s.sample.open(longID)
	return id, nil
}

// at is the bar r's path names, and its windows, if it's the last one opened: a page
// of an older one goes nowhere.
func (s *browserServer) at(r *http.Request) (*ui.Bar, *browserWindows, bool) {
	id, err := strconv.ParseInt(r.PathValue("bar"), 10, 64)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil || id != s.opened || s.bar == nil {
		return nil, nil, false
	}
	return s.bar, s.windows, true
}

// index opens a new bar, and serves the page that shows it.
func (s *browserServer) index(w http.ResponseWriter, r *http.Request) {
	id, err := s.open()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	data := struct {
		Name                  string
		Bar                   int64
		BarWindow, MenuWindow int
		BarHeight             int
	}{s.name, id, ui.BarWindow, ui.MenuWindow, barHeight}
	if err := index.Execute(w, data); err != nil {
		log.Print(err)
	}
}

// events serves the bar's events from ?from=, waiting for one if there are none yet,
// or {"stale": true} for a bar that isn't the last opened.
func (s *browserServer) events(w http.ResponseWriter, r *http.Request) {
	_, windows, ok := s.at(r)
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		json.NewEncoder(w).Encode(map[string]bool{"stale": true})
		return
	}
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	json.NewEncoder(w).Encode(map[string][]event{"events": windows.take(r.Context(), from)})
}

// page serves the page window was last loaded with, if it's ?gen=, with a stub at
// the top of its head that posts what's done in it to post, and opens its links in a
// new tab rather than in its frame.
func (s *browserServer) page(w http.ResponseWriter, r *http.Request) {
	_, windows, ok := s.at(r)
	window, _ := strconv.Atoi(r.PathValue("window"))
	gen, _ := strconv.Atoi(r.URL.Query().Get("gen"))
	var html string
	if ok {
		html, ok = windows.page(window, gen)
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	stub := fmt.Sprintf(`<base target="_blank"><script>
window.webkit = { messageHandlers: {
  ledge: { postMessage(message) {
    console.log("posted", message);
    fetch("/bar/%s/window/%d/post?gen=%d", { method: "POST", body: JSON.stringify(message) });
  } },
  frame: { postMessage(message) { console.log("posted to its window", message); } },
} };
</script>`, r.PathValue("bar"), window, gen)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, strings.Replace(html, "<head>", "<head>"+stub, 1))
}

// post has the bar carry out what the page in window, if it's still ?gen=, posted.
func (s *browserServer) post(w http.ResponseWriter, r *http.Request) {
	bar, windows, ok := s.at(r)
	window, _ := strconv.Atoi(r.PathValue("window"))
	gen, _ := strconv.Atoi(r.URL.Query().Get("gen"))
	message, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ok {
		_, ok = windows.page(window, gen)
	}
	if !ok {
		log.Printf("a page that's gone posted %s", message)
		w.WriteHeader(http.StatusGone)
		return
	}
	bar.Post(window, message)
	w.WriteHeader(http.StatusNoContent)
}

// answer takes what a page gave back for EvalResult: {id, value} or {id, error}.
func (s *browserServer) answer(w http.ResponseWriter, r *http.Request) {
	_, windows, ok := s.at(r)
	var a struct {
		ID    int             `json:"id"`
		Value json.RawMessage `json:"value"`
		Error string          `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ok {
		windows.answer(a.ID, a.Value, a.Error)
	}
	w.WriteHeader(http.StatusNoContent)
}

// event is something / does with the bar's windows' frames, in order: a window was
// loaded with a page, its gen, so the frame loads it; JS is to be run in a window's
// page, if it's still that gen, with what it gives back answering ask if that isn't
// 0; a report is in front, or with window 0, none is; or the menu's window is shown,
// at width by height, or hidden.
type event struct {
	Window int     `json:"window"`
	Gen    int     `json:"gen,omitempty"`
	Load   bool    `json:"load,omitempty"`
	JS     string  `json:"js,omitempty"`
	Ask    int     `json:"ask,omitempty"`
	Front  bool    `json:"front,omitempty"`
	Show   bool    `json:"show,omitempty"`
	Hide   bool    `json:"hide,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// browserWindows is ui.Windows for a bar shown in the browser, in frames of /: what's
// asked of them is events, which / takes in order. A window's page has a gen, new
// with each load, so what's asked of an older page goes nowhere.
type browserWindows struct {
	mu      sync.Mutex
	pages   map[int]string // what each window was last loaded with
	gens    map[int]int    // each window's page's gen
	gen     int            // the last gen given out
	events  []event        // those / hasn't taken yet
	taken   int            // how many events / has taken, and so were dropped
	more    chan struct{}  // closed, and made again, as events are added
	asked   int            // the last EvalResult asked
	answers map[int]chan answer
}

// answer is what a page gave back for EvalResult.
type answer struct {
	value json.RawMessage
	err   error
}

func newBrowserWindows() *browserWindows {
	return &browserWindows{pages: map[int]string{}, gens: map[int]int{}, more: make(chan struct{}),
		answers: map[int]chan answer{}}
}

// add adds e to the events. Call it with w's lock held.
func (w *browserWindows) add(e event) {
	w.events = append(w.events, e)
	close(w.more)
	w.more = make(chan struct{})
}

// take is the events from from on, waiting a while for one if there are none yet,
// or until ctx is done. Those before from, which / has, are dropped.
func (w *browserWindows) take(ctx context.Context, from int) []event {
	timeout := time.After(25 * time.Second)
	for {
		w.mu.Lock()
		if i := from - w.taken; i > 0 && i <= len(w.events) {
			w.events, w.taken = w.events[i:], from
		}
		if i := from - w.taken; i >= 0 && i < len(w.events) {
			events := slices.Clone(w.events[i:])
			w.mu.Unlock()
			return events
		}
		more := w.more
		w.mu.Unlock()
		select {
		case <-more:
		case <-ctx.Done():
			return nil
		case <-timeout:
			return []event{}
		}
	}
}

// page is what window was last loaded with, if that's gen's page.
func (w *browserWindows) page(window, gen int) (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	html, ok := w.pages[window]
	return html, ok && w.gens[window] == gen
}

// answer hands what a page gave back for EvalResult to what asked it.
func (w *browserWindows) answer(id int, value json.RawMessage, message string) {
	w.mu.Lock()
	done := w.answers[id]
	delete(w.answers, id)
	w.mu.Unlock()
	if done == nil {
		return
	}
	if message != "" {
		done <- answer{err: errors.New(message)}
		return
	}
	done <- answer{value: value}
}

func (w *browserWindows) LoadHTML(window int, html string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.gen++
	w.pages[window], w.gens[window] = html, w.gen
	w.add(event{Window: window, Gen: w.gen, Load: true})
}

func (w *browserWindows) Eval(window int, js string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.add(event{Window: window, Gen: w.gens[window], JS: js})
}

func (w *browserWindows) EvalResult(window int, js string) (json.RawMessage, error) {
	w.mu.Lock()
	w.asked++
	id, done := w.asked, make(chan answer, 1)
	w.answers[id] = done
	w.add(event{Window: window, Gen: w.gens[window], JS: js, Ask: id})
	w.mu.Unlock()
	select {
	case a := <-done:
		return a.value, a.err
	case <-time.After(5 * time.Second):
		w.mu.Lock()
		delete(w.answers, id)
		w.mu.Unlock()
		return nil, errors.New("the page didn't answer")
	}
}

// A report's window is its frame, so it's made, titled and framed by /.
func (w *browserWindows) CreateWindow(int, string, float64, float64) {}
func (w *browserWindows) SetTitle(int, string)                       {}
func (w *browserWindows) SetAppearance(int, string)                  {}
func (w *browserWindows) SetFrameless(int, bool)                     {}

func (w *browserWindows) ShowWindow(window int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.add(event{Window: window, Front: true})
}

func (w *browserWindows) CreatePopup(int) {}

func (w *browserWindows) ShowPopup(window int, width, height float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.add(event{Window: window, Show: true, Width: width, Height: height})
}

func (w *browserWindows) HidePopup(window int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.add(event{Window: window, Hide: true})
}

// ShowMenu is never asked: the server gives every theme a menu of its own.
func (w *browserWindows) ShowMenu([]shim.MenuItem) shim.Chosen {
	log.Print("the browser has no native menu")
	return shim.Chosen{Index: -1}
}

// OpenURL is never asked: a report's links open in a new tab themselves.
func (w *browserWindows) OpenURL(url string) { log.Printf("open %s", url) }
