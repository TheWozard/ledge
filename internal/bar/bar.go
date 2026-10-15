// Package bar is the Go side of the bar: where it sits, what it shows, and what the
// page's actions do. The shim only carries out its calls.
package bar

import (
	"embed"
	"encoding/json"
	"log"
	"os/exec"
	"strings"
	"sync"

	"ledge/internal/config"
	"ledge/internal/pages"
	"ledge/internal/schema"
	"ledge/internal/shim"
	"ledge/internal/store"
)

// Window is the bar's window id in the shim.
const Window = 1

//go:embed *.schema.json
var schemaFiles embed.FS

var schemas = func() *schema.Set {
	set, err := schema.Load(schemaFiles)
	if err != nil {
		panic(err)
	}
	return set
}()

// Bar is the bar window, the report window, and the store they show.
type Bar struct {
	store    *store.Store
	config   config.Config
	stateDir string // where commands.log goes

	// start runs a command in the background: `open` for a url, or a status's
	// command. Tests replace it.
	start func(*exec.Cmd) error

	mu          sync.Mutex
	reportDir   string // the report window may load files in here
	reportTitle string // its title when the page has none
}

// Open creates the bar, places it and loads its page. Call it after shim.Init and
// before shim.Run. Status commands' output goes to commands.log in stateDir.
func Open(events *store.Store, cfg config.Config, stateDir string) *Bar {
	b := &Bar{store: events, config: cfg, stateDir: stateDir, start: startDetached}
	shim.CreateBar(Window)
	b.place()
	shim.OnScreensChanged(func() { b.place() })
	shim.OnNavigate(b.navigate)
	shim.OnTitleChanged(b.titleChanged)
	shim.OnMessage(func(window int, message []byte) {
		if window == Window {
			go b.handle(message)
		}
	})
	shim.LoadHTML(Window, page(cfg))
	return b
}

// page is the bar's page in cfg's theme, for its edge.
func page(cfg config.Config) string {
	return strings.Replace(pages.Bar(cfg.Theme), "<body>", `<body class="`+cfg.Edge+`">`, 1)
}

// Render shows the store's current events. Safe from any goroutine.
func (b *Bar) Render() {
	data, err := json.Marshal(b.store.Visible())
	if err != nil {
		log.Printf("ledge: rendering the bar: %v", err)
		return
	}
	shim.Eval(Window, "render("+string(data)+")")
}

// place puts the bar along its edge of its screen, and returns where it went.
func (b *Bar) place() shim.Rect {
	return shim.PlaceBar(Window, b.config.Screen, b.config.Edge == "top", float64(b.config.Height))
}

// handle carries out one message from the page.
func (b *Bar) handle(data []byte) {
	value, err := schema.Decode(data)
	if err == nil {
		err = schemas.Validate("message", value)
	}
	if err != nil {
		log.Printf("ledge: bar sent %s: %v", data, err)
		return
	}
	message := value.(map[string]any)
	id, _ := message["id"].(string)
	switch message["action"] {
	case "ready":
		b.Render()
	case "dismiss":
		if _, err := b.store.Remove(id); err != nil {
			log.Printf("ledge: dismissing %s: %v", id, err)
		}
		b.Render()
	case "open":
		b.open(id)
	case "run":
		b.run(id)
	}
}
