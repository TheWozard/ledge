# ledge-frontend

The windows and UI of [ledge](https://github.com/TheWozard/ledge), a desktop bar for
macOS: a native shim for borderless and titled windows with web views, the bar and
report pages built from [templ](https://templ.guide) components, and the themes that
style them. ledge uses it for everything on screen; its own code is the API, the
event store and what clicks do.

```go
import (
	"time"

	"github.com/a-h/templ"

	"github.com/TheWozard/ledge-frontend/components"
	"github.com/TheWozard/ledge-frontend/ui"
)

func main() {
	ui.Init() // on the main goroutine; AppKit needs it
	bar, err := ui.Open(
		ui.Options{Edge: "bottom", Height: 110, Level: "floating", Theme: "win95"},
		ui.Handlers{
			Dismiss: func(id string) { /* forget it, then bar.Render again */ },
			Open:    func(id string) { /* bar.ShowReport(id, ...), or open its url */ },
			Run:     func(id string) { /* run the pill's command */ },
			OpenURL: func(url string) { /* a link clicked in a report */ },
		})
	if err != nil {
		panic(err)
	}
	go func() {
		bar.Render(
			[]components.Pill{{ID: "aws", Title: "AWS", Color: "green", Command: "aws sso login"}},
			[]components.Card{{ID: "ci", Title: "CI failed", Priority: "high", Opens: true, Source: "ci", Received: time.Now()}})
		bar.ShowReport("ci", "CI failed", []templ.Component{
			components.Note("Two jobs failed.", "red"),
			components.Table([]string{"Job", "State"}, [][]string{{"build", "failed"}}),
		})
	}()
	ui.Run() // until ui.Stop
}
```

## Packages

| Package | What it is |
|---|---|
| `ui` | the bar and report windows: `Open` places the bar and loads its page, `Render` shows pills and cards, `ShowReport` shows a report in its id's own window; the pages' clicks come back to `Handlers` as ids |
| `components` | the templ components: the pages (`BarPage`, `ReportPage`), the bar's contents (`BarContent`, from `Pill`s and `Card`s), and the report blocks |
| `theme` | the themes' stylesheets and fonts, built in, and how each theme's report window frame looks |
| `shim` | the native layer (cgo + Objective-C): windows and WKWebView, and nothing else |

### Components

The bar is rendered in Go: `Bar.Render` renders `BarContent`, and the page morphs into it
with [Idiomorph](https://github.com/bigskysoftware/idiomorph), matching pills and cards
by id. Only what changed is touched, so the rest keep their elements: the row and the
pills keep their scroll, a hovered card its tooltip, a clicked pill its flash. New pills
and cards animate in, and gone ones animate out before they're removed. It renders again on its own when a
relative time ("5m") goes stale or a pill's scheduled colour change is due.

A report is a title and any `templ.Component`s, drawn top to bottom, shown for an id
such as the card it's about. Each id has its own window: showing the same id again
replaces what's in it, and closing it destroys it, so the next `ShowReport` for that id
opens a new one. New windows open down and to the right of the last. The built-in
blocks:

| Block | Shows |
|---|---|
| `Heading(text)` | a section heading |
| `Paragraph(text)` | a paragraph; line breaks are kept |
| `Fields([]Field)` | label and value rows |
| `Table(columns, rows)` | a table; short rows are padded, long ones cut |
| `List(items, ordered)` | bullets, or numbers |
| `Code(text)` | fixed-width text that scrolls sideways |
| `Note(text, color)` | a callout in a colour |
| `Link(text, url)` | a link; clicking it calls `Handlers.OpenURL` if it's http(s) |

Every string is escaped by templ, so text can't add markup. Colours are a theme's named
colours (`green`, `yellow`, `red`, `blue`, `gray`) or `#rrggbb`; anything else is gray,
and can't add CSS.

### What the pages can do

The bar page posts only `{action, id}` (`dismiss`, `open`, `run`, and `ready` once
loaded), checked strictly before it reaches a handler; it never names a url, path or
command, so the app decides what an id means. The report page can't post to the app
at all. Its only channel is to its own window, handled natively: `drag` (on a
mouse-down in its title bar; a double-click zooms), `close`, `minimise` and `zoom`.
Neither window can navigate: a report's clicked http(s) link goes to `OpenURL`.

## Themes

`default`, `platinum`, `win95`, `handheld`, `synthwave` and `hifi`. A theme is two
stylesheets in [theme/themes/](theme/themes/): `<name>.css` on top of the bar's layout,
and `<name>.report.css` on top of the report window's. The comments at the top of the
layout styles in [bar.templ](components/bar.templ) and
[report.templ](components/report.templ) say what a theme can hook into: the colour
variables it sets, a pill's `data-color`, a new pill's or card's `data-new` and a gone
one's `data-leaving` (for enter and exit animations, the exit ending with the layout's
`collapse` so the rest close up), the report window's
`.titlebar` and its buttons, and the `main` that scrolls under it. `theme.Frames` says whether a theme's report window is
light or dark, and whether the theme draws the whole window (square corners, its own
buttons and border).

Adding a theme: add its two stylesheets, its name to `theme.Names`, and a `Frames`
entry if it needs one. Fonts not on every Mac go in `theme/themes/fonts/` as woff2,
named in a stylesheet as `url(fonts/<file>)`, and are inlined when it loads; their
licences are in [OFL.txt](theme/themes/fonts/OFL.txt).

## Developing

The `.templ` files are compiled to `*_templ.go` with the templ CLI, and the generated
files are committed, so building doesn't need it:

```sh
go install github.com/a-h/templ/cmd/templ@v0.3.1001
just generate      # templ generate
just check-generated # fail if the committed *_templ.go are stale
just test          # go test ./...
just gui           # the tests that open windows: LEDGE_GUI_TEST=1 go test ./ui
just preview win95 # a bar and a report window in a theme, until ctrl-c
```

The tests that open windows put a bar on your screen and a report window over it for a
few seconds. They read the pages' DOM through the shim and click from JavaScript; they
can't check hover, a real mouse drag, keyboard shortcuts or how the windows look.

## License

MIT; the bundled fonts are under the SIL Open Font License, and the vendored Idiomorph
([components/vendor/](components/vendor/)) under 0BSD.
