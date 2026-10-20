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
	"github.com/TheWozard/ledge-frontend/theme"
	"github.com/TheWozard/ledge-frontend/ui"
)

func main() {
	ui.Init() // on the main goroutine; AppKit needs it
	bar, err := ui.Open(
		ui.Options{Edge: "bottom", Height: 110, Level: "floating", Theme: must(theme.Builtin("default"))},
		ui.Handlers{OpenURL: func(url string) { /* a link clicked in a report */ }})
	if err != nil {
		panic(err)
	}
	go func() {
		report := []templ.Component{
			components.Note("Two jobs failed.", "red"),
			components.Table([]string{"Job", "State"}, [][]string{{"build", "failed"}}),
		}
		bar.Render([]components.Card{components.Basic{ID: "ci", Title: "CI failed", Priority: 1, Color: "#e0362c",
			Click:   func() { bar.ShowReport("ci", "CI failed", report) },
			Dismiss: func() { /* forget it, then bar.Render again */ },
			Actions: []components.Action{{ID: "rerun", Title: "Rerun", Color: "yellow",
				Click: func() { /* gh run rerun */ }}}}})
	}()
	ui.Run() // until ui.Stop
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
```

## Packages

| Package | What it is |
|---|---|
| `ui` | the bar and report windows: `Open` places the bar and loads its page, `Render` shows cards and their actions, `ShowReport` shows a report in its id's own window; clicks call the cards' and actions' `Click`s |
| `components` | the templ components: the pages (`BarPage`, `ReportPage`), the bar's contents (`BarContent`, from `Card`s of any kind, such as the built-in `Basic`, and their `Action`s), and the report blocks |
| `theme` | a `Theme`: the bar's and report window's stylesheets, fonts inlined, and how the report window's frame looks; `ReadSet` finds the themes in any files, `Load` reads one, `Builtin` is one of the built-in themes |
| `shim` | the native layer (cgo + Objective-C): windows and WKWebView, and nothing else |

### Components

The bar is rendered in Go: `Bar.Render` renders `BarContent`, and the page morphs into it
with [Idiomorph](https://github.com/bigskysoftware/idiomorph), matching cards and
actions by id. Only what changed is touched, so the rest keep their elements: the row
keeps its scroll, a hovered card its hover, a clicked action its flash. New cards
and actions animate in, and gone ones animate out before they're removed. It renders
only when the app calls `Render`.

A card is any `components.Card`, an interface of two methods, so an app can have kinds of
card of its own. `Frame()` is what the bar draws the same for every kind: the card's
id, its `Kind` (a class on it, for a theme to style it by), priority, colour, width,
`Click`,
`Dismiss` and actions. `Body()` is a templ component for what's inside the frame, above
the actions. There are two built-in kinds:

- `components.Basic` (`basic`): a title and a description.
- `components.Timer` (`timer`): a title and the time left from `Start` to `End`. The page
  counts it down itself, setting `--progress` (0 to 1) on the card for a theme to fill
  it, and `data-done` once it's up. Its `Complete` is called when `End` passes.

Any kind can end: a frame with `Ends` and `Complete` has `Complete` called once when
`Ends` passes, if the card's still on the bar, however many renders show it.
`components.Cards` turns a slice of one kind into `[]Card` for `Render`.

Anything clickable has a `Click`, a `func()` the bar calls on its own goroutine: a card's
`Click` for the card itself, its `Dismiss` for its × (shown only if set), and an
action's `Click`. An action without one is part of its card. The bar gives each
`Click` an id for the page, made from the card's and action's ids, so the app never
handles click ids itself; a click on something a later `Render` took off is dropped.

A card's actions sit at its bottom: things to do about it without opening a
report. Each has a colour.

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

The bar page posts only `{action: "click", id}`, and `{action: "ready"}` once loaded,
checked strictly before it reaches a `Click`. It holds nothing but ids: it never names a
url, path or command, so what a click does is only ever the app's `Click`. The report page can't post to the app
at all. Its only channel is to its own window, handled natively: `drag` (on a
mouse-down in its title bar; a double-click zooms), `close`, `minimise` and `zoom`.
Neither window can navigate: a report's clicked http(s) link goes to `OpenURL`.

## Themes

`ui.Options.Theme` is a `theme.Theme`: the bar's stylesheet, the report window's, and
its `Frame` (whether the report window is light or dark, and whether the theme draws the
whole window: square corners, its own buttons and border). The zero `Theme` is the
built-in default. Get one with:

- `theme.ReadSet(fsys)`: every theme in an `fs.FS`, such as `os.DirFS` of a user's
  themes directory: each `.json` in it, at any depth, is a theme's manifest. `Names()`
  lists them, `Default()` names the one whose manifest says `"default": true` (at most
  one may), and `Theme(name)` loads one.
- `theme.Builtin(name)`: one of the built-in themes, `theme.Builtins`: for now, just
  `default`.
- `theme.Load(fsys, file)`: one from its json manifest `file` in any `fs.FS`, such as
  `os.DirFS` of a user's themes directory.
- Or make one up: the stylesheets are plain strings. `theme.InlineFonts` inlines a
  stylesheet's fonts from an `fs.FS`.

A manifest names the theme, says if it's its set's default, and lists each page's stylesheets, joined in order, so files
can be shared between pages and themes (paths are relative to the manifest):

```json
{
  "name": "handheld",
  "bar": ["css/silkscreen.css", "css/handheld.css"],
  "report": ["css/silkscreen.css", "css/handheld.report.css"],
  "frame": {"appearance": "light"}
}
```

`frame` is optional: `appearance` is `"light"`, `"dark"` or unset (follow the system),
and `frameless` is true for a theme that draws the whole window. A stylesheet names a
font (woff2, woff, ttf or otf) by a url relative to itself, such as
`url(../fonts/silkscreen.woff2)`, and it can be anywhere in the `fs.FS`.

The pages are loaded from strings and can't fetch files, so `Load` inlines each font as
a `data:` url. The comments at the top of the layout styles in
[bar.templ](components/bar.templ) and [report.templ](components/report.templ) say what a
theme can hook into: the colour variables it sets, the bar's padding (`--pad-top`,
`--pad-x` and `--pad-bottom`, which the row of cards scrolls through to the window's
edges), the row's own sideways padding (`--row-pad-x`), a card's kind as a class (such
as `basic`) and its `data-color` and `--card-color`, its width as `--card-width` (which a
card's own `Width` overrides), an action's `data-color`
and `--action`, a new card's or action's `data-new` and a gone one's
`data-leaving`
(for enter and exit animations, the exit ending with the layout's `collapse` so the rest
close up), the report window's `.titlebar` and its buttons, the `main` that scrolls
under it, and the `article` in it that holds the blocks (for a background that scrolls
with them).

Adding a built-in theme: add its manifest, `<name>.json`, to
[theme/themes/](theme/themes/), its stylesheets to [css/](theme/themes/css/), any fonts
to [fonts/](theme/themes/fonts/); nothing lists the themes, so that's all. The bundled fonts'
licences are in [OFL.txt](theme/themes/fonts/OFL.txt), and their `@font-face` rules in
`css/<font>.css`. To try a theme of your own, `just preview path/to/neon.json`.

## Developing

The `.templ` files are compiled to `*_templ.go` with the templ CLI, and the generated
files are committed, so building doesn't need it:

```sh
go install github.com/a-h/templ/cmd/templ@v0.3.1001
just generate      # templ generate
just check-generated # fail if the committed *_templ.go are stale
just test          # go test ./...
just gui           # the tests that open windows, those for each theme in only the default
just gui-themes default # only the tests for each theme, in those themes
just gui-all       # every test that opens windows, in every theme (slow)
just preview default # a bar and a report window in a theme (or a .json), until ctrl-c
just inspect default # the same pages in the browser, to inspect; theme edits show on a reload
```

The tests that open windows put a bar on your screen and a report window over it for a
few seconds. They read the pages' DOM through the shim and click from JavaScript; they
can't check hover, a real mouse drag, keyboard shortcuts or how the windows look.

## License

MIT; the bundled fonts are under the SIL Open Font License, and the vendored Idiomorph
([components/vendor/](components/vendor/)) under 0BSD.
