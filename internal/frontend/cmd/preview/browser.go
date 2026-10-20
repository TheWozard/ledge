package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
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
// and a report opens in the report frame.
const barStub = `<script>
window.webkit = { messageHandlers: { ledge: { postMessage(message) {
  console.log("bar posted", message);
  fetch("/action", { method: "POST", body: JSON.stringify(message) }).then(r => r.json()).then(reply => {
    if (reply.html != null) update(reply.html);
    if (reply.report) window.top.document.getElementById("report")?.setAttribute("src", "/report?id=" + encodeURIComponent(reply.report));
  });
} } } };
</script>`

// reportStub logs what the report page posts to its window, and opens its links in
// a new tab rather than in its frame.
const reportStub = `<base target="_blank"><script>
window.webkit = { messageHandlers: { frame: { postMessage(message) { console.log("report posted", message); } } } };
</script>`

// serveBrowser serves the bar and a report on addr, for a browser's inspector, and
// opens them: / is both, the bar at the bottom at its height and the report above it
// at its window's size; /bar and /report?id= are each alone. The theme is loaded
// afresh for every page, so an edit to its stylesheets shows on a reload. A click
// calls its Click here; dismissing a card takes it off the bar until the server stops.
func serveBrowser(addr, name string, load func() (theme.Theme, error), sample *sample) {
	// opening is the report a click opened, to send back with its reply.
	var mu sync.Mutex
	var opening string
	sample.open = func(id string) {
		mu.Lock()
		opening = id
		mu.Unlock()
	}
	sample.changed = func() {} // every reply to a click has the cards

	page := func(w http.ResponseWriter, render func(theme.Theme) templ.Component, stub string) {
		th, err := load()
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
	content := func() string {
		shown := sample.shown()
		components.SortByPriority(shown)
		var buf bytes.Buffer
		if err := components.BarContent(shown, map[string]int{}).Render(context.Background(), &buf); err != nil {
			log.Print(err)
		}
		return buf.String()
	}

	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>ledge preview: %s</title>
<style>
  html, body { margin: 0; height: 100%%; }
  body { display: flex; flex-direction: column; align-items: center; background: #7a7a80; }
  #report { flex: none; width: 900px; max-width: calc(100%% - 32px); height: 700px; max-height: calc(100%% - %dpx - 32px);
            margin: auto 0; border: 0; box-shadow: 0 8px 30px rgba(0, 0, 0, .35); }
  #bar { flex: none; width: 100%%; height: %dpx; border: 0; }
</style>
</head>
<body>
<iframe id="report" src="/report?id=report"></iframe>
<iframe id="bar" src="/bar"></iframe>
</body>
</html>
`, html.EscapeString(name), barHeight, barHeight)
	})
	http.HandleFunc("GET /bar", func(w http.ResponseWriter, r *http.Request) {
		page(w, func(th theme.Theme) templ.Component { return components.BarPage(th.Bar, "bottom") }, barStub)
	})
	http.HandleFunc("GET /report", func(w http.ResponseWriter, r *http.Request) {
		title, blocks := sample.report(r.URL.Query().Get("id"))
		page(w, func(th theme.Theme) templ.Component { return components.ReportPage(th.Report, title, blocks) }, reportStub)
	})
	http.HandleFunc("POST /action", func(w http.ResponseWriter, r *http.Request) {
		var message struct{ Action, ID string }
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("%s %s", message.Action, message.ID)
		reply := map[string]string{}
		switch message.Action {
		case "ready":
			reply["html"] = content()
		case "click":
			if click := components.Clicks(sample.shown())[message.ID]; click != nil {
				click()
			}
			reply["html"] = content()
			mu.Lock()
			if opening != "" {
				reply["report"], opening = opening, ""
			}
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reply)
	})

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
