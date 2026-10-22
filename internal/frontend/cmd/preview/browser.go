package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/theme"
)

// barStub stands in for the app in a browser: what the bar page posts goes to
// /action, and what comes back is shown: new cards are morphed in, as the app does,
// and a report, or a card's menu, opens in the report frame.
const barStub = `<script>
window.webkit = { messageHandlers: { ledge: { postMessage(message) {
  console.log("bar posted", message);
  fetch("/action", { method: "POST", body: JSON.stringify(message) }).then(r => r.json()).then(reply => {
    if (reply.html != null) update(reply.html);
    if (reply.report) window.top.document.getElementById("report")?.setAttribute("src", "/report?id=" + encodeURIComponent(reply.report));
    if (reply.menu) window.top.document.getElementById("report")?.setAttribute("src", "/menu?id=" + encodeURIComponent(reply.menu));
  });
} } } };
</script>`

// reportStub logs what the report page posts, to the app or its window, and opens its
// links in a new tab rather than in its frame.
const reportStub = `<base target="_blank"><script>
window.webkit = { messageHandlers: {
  ledge: { postMessage(message) { console.log("report posted", message); } },
  frame: { postMessage(message) { console.log("report posted to its window", message); } },
} };
</script>`

// menuStub logs what a card's menu page posts.
const menuStub = `<script>
window.webkit = { messageHandlers: { ledge: { postMessage(message) { console.log("menu posted", message); } } } };
</script>`

// index is the page at /: the report above the bar, each in a frame at its window's
// size.
//
//go:embed index.html
var indexHTML string
var index = template.Must(template.New("index").Parse(indexHTML))

// browserServer serves the bar and a report, for a browser's inspector: / is both,
// the bar at the bottom at its height and the report above it at its window's size;
// /bar, /report?id= and /menu?id= are each alone. The theme is loaded afresh for every page, so
// an edit to its stylesheets shows on a reload. A click calls its Click here;
// dismissing a card takes it off the bar until the server stops.
type browserServer struct {
	name   string
	load   func() (theme.Theme, error)
	sample *sample

	mu      sync.Mutex
	opening string // the report a click opened, to send back with its reply
}

// serveBrowser serves sample on addr, as browserServer says, and opens it in the
// browser.
func serveBrowser(addr, name string, load func() (theme.Theme, error), sample *sample) {
	s := &browserServer{name: name, load: load, sample: sample}
	sample.open = func(id string) {
		s.mu.Lock()
		s.opening = id
		s.mu.Unlock()
	}
	// A report's clicks are only logged in the browser, so nothing's replaced: a reload
	// shows it as it is.
	sample.replace = func(string, string, templ.Component) {}
	sample.changed = func() {} // every reply to a click has the cards

	http.HandleFunc("GET /{$}", s.index)
	http.HandleFunc("GET /bar", s.bar)
	http.HandleFunc("GET /report", s.report)
	http.HandleFunc("GET /menu", s.menu)
	http.HandleFunc("POST /action", s.action)

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

// index serves the report above the bar.
func (s *browserServer) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Name, Report string
		BarHeight    int
	}{s.name, longID, barHeight}
	if err := index.Execute(w, data); err != nil {
		log.Print(err)
	}
}

// bar serves the bar's page, empty until it posts ready.
func (s *browserServer) bar(w http.ResponseWriter, r *http.Request) {
	s.page(w, func(th theme.Theme) templ.Component { return components.BarPage(th.Bar, "bottom") }, barStub)
}

// report serves the report for ?id=.
func (s *browserServer) report(w http.ResponseWriter, r *http.Request) {
	title, blocks := s.sample.report(r.URL.Query().Get("id"))
	s.page(w, func(th theme.Theme) templ.Component { return components.ReportPage(th.Report, title, blocks) }, reportStub)
}

// menu serves the menu of the card whose element id is ?id=, as its theme draws it;
// in a theme without one, only its layout.
func (s *browserServer) menu(w http.ResponseWriter, r *http.Request) {
	var items []components.MenuItem
	for _, card := range s.sample.shown() {
		if frame := card.Frame(); frame.ElementID() == r.URL.Query().Get("id") {
			items = frame.Menu()
		}
	}
	s.page(w, func(th theme.Theme) templ.Component { return components.MenuPage(th.Menu, items) }, menuStub)
}

// action takes what the bar page posts, as the app would, and replies with the bar's
// content and any report a click opened, or the card whose menu was asked for.
func (s *browserServer) action(w http.ResponseWriter, r *http.Request) {
	var message struct {
		Action, ID string
		Event      components.ClickEvent
	}
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("%s %s", message.Action, message.ID)
	reply := map[string]string{}
	switch message.Action {
	case "ready":
		reply["html"] = s.content()
	case "click":
		if click := components.Clicks(s.sample.shown())[message.ID]; click != nil {
			click(message.Event)
		}
		reply["html"] = s.content()
		s.mu.Lock()
		if s.opening != "" {
			reply["report"], s.opening = s.opening, ""
		}
		s.mu.Unlock()
	case "menu":
		reply["menu"] = message.ID
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reply)
}

// page serves the page render makes in the theme, with stub at the top of its head.
func (s *browserServer) page(w http.ResponseWriter, render func(theme.Theme) templ.Component, stub string) {
	th, err := s.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := render(th).Render(context.Background(), &buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, strings.Replace(buf.String(), "<head>", "<head>"+stub, 1))
}

// content is the bar's content: the sample's cards.
func (s *browserServer) content() string {
	shown := s.sample.shown()
	var buf bytes.Buffer
	if err := components.BarContent(shown, map[string]int{}).Render(context.Background(), &buf); err != nil {
		log.Print(err)
	}
	return buf.String()
}
