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
	bar, err := ui.Open(ui.Options{Edge: "bottom", Height: 110, Level: "floating", Theme: must(theme.Builtin("default"))})
	if err != nil {
		panic(err)
	}
	go func() {
		report := []templ.Component{
			components.Note("Two jobs failed.", "red"),
			components.Table([]string{"Job", "State"}, [][]string{{"build", "failed"}}),
		}
		bar.Render([]components.Card{components.Basic{ID: "ci", Title: "CI failed", Color: "#e0362c",
			Click: func(e components.ClickEvent) {
				if e.Cmd { /* ⌘-click: open the run in the browser */ return }
				bar.ShowReport("ci", "CI failed", report)
			},
			Dismiss: func(components.ClickEvent) { /* forget it, then bar.Render again */ },
			Actions: []components.Action{{ID: "rerun", Title: "Rerun", Color: "yellow",
				Click: func(components.ClickEvent) { /* gh run rerun */ }}}}})
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
| `ui` | the bar and report windows: `Open` places the bar and loads its page, `Render` shows cards and `RenderCard` replaces one by its ID, `ShowReport` shows a report in its id's own window and `ShowReportBlock` replaces one block in it; clicks call the cards' `Click`s, a card's right-click menu its actions', and a report's buttons theirs |
| `components` | the templ components: the pages (`BarPage`, `ReportPage`), the bar's contents (`BarContent`, from `Card`s of any kind, such as the built-in `Basic`, and their `Action`s), and the report blocks |
| `theme` | a `Theme`: the bar's and report window's stylesheets, fonts inlined, and how the report window's frame looks; `ReadSet` finds the themes in any files, `Load` reads one, `Builtin` is one of the built-in themes |
| `shim` | the native layer (cgo + Objective-C): windows and WKWebView, and nothing else |

### Components

The bar is rendered in Go: `Bar.Render` renders `BarContent`, and the page morphs into it
with [Idiomorph](https://github.com/bigskysoftware/idiomorph), matching cards by id.
Only what changed is touched, so the rest keep their elements: the row keeps its
scroll, and a hovered card its hover. New cards animate in, and gone ones animate out
before they're removed. It renders only when the app calls `Render` or `RenderCard`, which replaces one card by its ID and leaves the rest.

A card is any `components.Card`, an interface of two methods, so an app can have kinds of
card of its own. `Frame()` is what the bar draws the same for every kind: the card's
id, its `Kind` (a class on it, for a theme to style it by), colour, width, `Click`,
`Dismiss` and actions. `Body()` is a templ component for what's inside the frame.
There are six built-in kinds:

- `components.Basic` (`basic`): a title and a description.
- `components.Timer` (`timer`): a `Basic` card that keeps time, shown under its title
  with its description beside it, such as
  `Timer{Basic: components.Basic{...}, End: meeting}`. The page keeps its time itself,
  from whichever of `Start` and `End` it has:
  - both: it counts down to `End`, setting `--progress` (0 to 1 from `Start` to `End`)
    on the card for a theme to fill it;
  - only `End`, for something coming up such as a meeting: it counts down, unfilled;
  - only `Start`, for something going on such as an incident: it counts up.

  One with an `End` has `data-done` once it's up, and its `Complete` is called when
  `End` passes.

- `components.Progress` (`progress`): a `Basic` card showing how far along something
  is, such as a deploy: its title, then its `Value` (0 to 1) as a percent, or its
  `Label` such as `"3/5"`, with its description beside it. It has `--progress` on the
  card, for a theme to fill it; the app renders it again as it moves. Any kind can
  set `--progress` on its card, through its frame's `Progress`.

- `components.Stat` (`stat`): a `Basic` card showing a number: its title, then its
  `Value` (such as `"2.3%"`), with its `Change` (such as `"↑0.8"`) small beside it in
  its `ChangeColor`, and its description beside those. Its `History`, oldest first,
  is drawn behind the whole card as a sparkline, an inline SVG `polyline` a theme
  strokes and insets, so it's there whatever else the card has.

- `components.Checks` (`checks`): a `Basic` card showing how a set of `Check`s went,
  such as a CI run's jobs: its title, then each check's mark and name in a
  `.check-list`, wrapping, then its description. A check's `State` is `passed`,
  `failed`, `running`, `pending` or `skipped`, in its `data-state` for a theme to
  colour it. A check with a `Click` is clicked on its own, such as to open a failed
  job's log, and an action can do something about one, such as rerunning it.

- `components.Message` (`message`): a `Basic` card for a message, such as a chat or an
  email: a line of its avatar, who it's `From` and when it was `Sent` (the time today,
  the day within the week, the date before that), then its title, such as a subject
  or channel, and its description, the start of it. Its `Avatar` is a `data:` URL of
  an image, base64; anything else shows `From`'s initials, as the page can't fetch
  anything and mustn't try.

Any kind can end: a frame with `Ends` and `Complete`, a `func()`, has `Complete`
called once when `Ends` passes, if the card's still on the bar, however many renders
show it.
`components.Cards` turns a slice of one kind into `[]Card` for `Render`.

Anything clickable has a `Click`, a `func(ClickEvent)` the bar calls on its own
goroutine with how it was clicked: the modifier keys held (`Alt`, `Cmd`, `Shift`,
`Ctrl`), and whether with the `Middle` mouse button, which clicks anything the main
one does but a card's ⋯. An action chosen from a menu has the keys held as it was
chosen, by a click or Return. (A ⌃-click on a card is a right-click: it shows the
card's menu.) There's a `Click` for a card itself, its `Dismiss` for its × (shown only if set), an
action's `Click`, and one for anything in a card's body clicked on its own, not as the
card, such as a check with a `Click`: a kind of card of the app's marks each in its
body with `{ components.Clickable(ctx, id, click)... }`, which gives it `data-click`, or
nothing for a nil `Click`, with an id unique within its card. The page has an id for each `Click`,
made from the card's id and its own, so two cards can use the same ids, and the app
never handles the page's ids itself; a click on something a later `Render` took off
is dropped. While a `Click` runs, what was clicked has `data-busy`, for a theme to
show, and a click on it meanwhile is dropped, so a double-click doesn't run it twice.
A card's ⋯ is busy from when its menu opens until it's closed and what was chosen
has run, so its menu can't be opened twice at once.

A card's actions are things to do about it without opening a report, in a menu:
right-clicking the card, or clicking its ⋯ (shown only if it has some), pops it up
at the mouse. Each is an item with a dot in its colour; one without a `Click` is
greyed out. Choosing one calls its `Click`, not the card's. A theme with a menu
stylesheet draws the menu itself, as a page in a pop-up window of its own, which
takes the arrow keys, Return and Escape and closes on a click anywhere else; in a
theme without one, it's the native macOS menu.

A report is a title and any `templ.Component`s, drawn top to bottom, shown for an id
such as the card it's about. Each id has its own window: showing the same id again
replaces what's in it, morphing it as the bar is, so it keeps its scroll, and closing
it destroys it, so the next `ShowReport` for that id opens a new one. New windows open
down and to the right of the last. A block given an id, as a
`components.Block{ID: id, Content: block}`, can be replaced on its own by
`ShowReportBlock(report, block)`, with another `Block` with that ID, which leaves the rest of
the report as it is and doesn't bring its window forward; it says whether the report is
open with such a block, as `RenderCard` says whether the bar has the card. Each of
`Render`, `RenderCard`, `ShowReport` and `ShowReportBlock` renders what it's given there
and then, and if it can't, says why and changes nothing. As the bar's cards are, a report's blocks are rendered with the bar's lock
held, so they mustn't call back into it. What's clicked in a report,
such as a `Buttons`' button, calls its `Click` as one on the bar does, busy while it
runs, until the report is shown again or closed; a block replaced with
`ShowReportBlock` brings its own. A block of the app's can have clickable things too:
`{ components.Clickable(ctx, id, click)... }`, spread on an element as it renders,
gives it `data-click`. An id is unique within its `Block`, or among the report's
clickable things in none. A theme styles what can be clicked by `[data-click]`. The built-in blocks:

| Block | Shows |
|---|---|
| `Heading(text)` | a section heading |
| `Paragraph(text)` | a paragraph; line breaks are kept |
| `Fields([]Field)` | label and value rows |
| `Table(columns, rows)` | a table; short rows are padded, long ones cut |
| `List(items, ordered)` | bullets, or numbers |
| `Code(text)` | fixed-width text that scrolls sideways |
| `Note(text, color)` | a callout in a colour |
| `Link(text, url)` | a link; clicking it opens it in the browser if it's http(s) |
| `Buttons([]Button)` | a row of buttons, each with a dot in its colour, calling its `Click`; one without is disabled |

Every string is escaped by templ, so text can't add markup. Colours are a theme's named
colours (`green`, `yellow`, `red`, `blue`, `gray`) or `#rrggbb`; anything else is gray,
and can't add CSS.

### What the pages can do

The bar page posts only `{action: "click", id, event}` (for a card, its ×, its ⋯ or
a right-click on it, which shows its menu, or a part of it, with `event` how it was
clicked, `{alt, cmd, shift, ctrl, middle}`), and `{action: "ready"}` once loaded,
checked strictly before it reaches a `Click`. It holds nothing but ids: it never names a url, path or command, so what a
click does is only ever the app's `Click`. A card's menu is built from its actions in
Go, not by the bar page; WebKit's own right-click menu never shows. A theme's menu
page posts only `{action: "choose", id, event}`, with the index of the item chosen and
the keys held,
`{action: "close"}`, and `{action: "ready"}`, checked as strictly. A
report page posts only `{action: "click", id, event}` and `{action: "ready"}`, checked as strictly, and reaching only
a `Click` in that report: never one on the bar or in another report. Its only other
channel is to its own window, handled natively: `drag` (on a mouse-down in its title
bar; a double-click zooms), `close`, `minimise` and `zoom`.
Neither window can navigate: a report's clicked http(s) link opens in the browser.
Every page has a Content-Security-Policy that runs only its own scripts, by their
hashes, and loads only `data:` images and fonts, so a script that got past templ's
escaping wouldn't run; the app's calls into the page aren't stopped by it.

## Themes

`ui.Options.Theme` is a `theme.Theme`: the bar's stylesheet, the report window's, a
card menu's (or none, for the native menu), and its `Frame` (whether the report window is light or dark, and whether the theme draws the
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
  "name": "neon",
  "bar": ["css/neon-font.css", "css/neon.css"],
  "report": ["css/neon-font.css", "css/neon.report.css"],
  "menu": ["css/neon-font.css", "css/neon.menu.css"],
  "frame": {"appearance": "dark"}
}
```

`menu` is optional: without it, a card's menu is the native one. `frame` is optional: `appearance` is `"light"`, `"dark"` or unset (follow the system),
and `frameless` is true for a theme that draws the whole window. A stylesheet names a
font (woff2, woff, ttf or otf) by a url relative to itself, such as
`url(../fonts/neon.woff2)`, and it can be anywhere in the `fs.FS`.

The pages are loaded from strings and can't fetch files, so `Load` inlines each font as
a `data:` url. The comments at the top of the layout styles in
[page/bar.css](components/page/bar.css), [page/report.css](components/page/report.css) and
[page/menu.css](components/page/menu.css) say what a theme can hook into: the colour variables it sets, the bar's padding (`--pad-top`,
`--pad-x` and `--pad-bottom`, which the row of cards scrolls through to the window's
edges), the row's own sideways padding (`--row-pad-x`), a card's kind as a class (such
as `basic`) and its `data-color` and `--card-color`, its width as `--card-width` (300px, or
100px for a timer, progress or stat card, 150px for a stat with a change, and 200px
with a description, set on such as `li.timer`, `li.stat:has(.change)` and
`li.timer:has(.description)`; a card's own `Width` overrides it), its `.card-buttons`
(`.menu`, its ⋯, and `.dismiss`, its ×), a new card's `data-new` and a gone one's
`data-leaving` (for enter and exit animations, the exit ending with the layout's `collapse` so the rest
close up), the report window's `.titlebar` and its buttons, the `main` that scrolls
under it, and the `article` in it that holds the blocks (for a background that scrolls
with them), and a menu's `#menu`, the only thing in its see-through window, and its
`.menu-item`s (`active` under the mouse or the keys, `aria-disabled` if they can't
be chosen, each a `.menu-dot` in its `--item-color` and a `.menu-title`).

Adding a built-in theme: add its manifest, `<name>.json`, to
[theme/themes/](theme/themes/), its stylesheets to [css/](theme/themes/css/), any fonts
to `theme/themes/fonts/`, with their licences beside them; nothing lists the themes, so
that's all. To try a theme of your own, `just preview path/to/neon.json`.

## Developing

The `.templ` files are compiled to `*_templ.go` with the templ CLI, and the generated
files are committed, so building doesn't need it. The CSS, the pages' layout styles and the
built-in themes', is linted with [Biome](https://biomejs.dev), set up in
[biome.json](biome.json):

```sh
go install github.com/a-h/templ/cmd/templ@v0.3.1070
brew install biome
just generate      # templ generate
just check-generated # fail if the committed *_templ.go are stale
just lint          # biome lint, failing on warnings too
just fmt           # format the CSS and sort each rule's properties, writing the changes
just test          # just lint, then go test ./...
just gui           # the tests that open windows, those for each theme in only the default
just gui-themes default # only the tests for each theme, in those themes
just gui-all       # every test that opens windows, in every theme (slow)
just preview default # a bar and a report window in a theme (or a .json), until ctrl-c
just inspect default # the same pages in the browser, to inspect (a card's menu opens in the report frame); theme edits show on a reload
```

On GitHub, [ci.yml](.github/workflows/ci.yml) runs the lint, `check-generated`,
`gofmt`, `go vet` and the tests that don't open windows on every push to `main` and
every pull request, on macOS (the shim is Cocoa, so nothing builds elsewhere). The
tests that open windows run only locally, with `just gui`.

The tests that open windows put a bar on your screen and a report window over it for a
few seconds. They read the pages' DOM through the shim and click from JavaScript; they
can't check hover, a real mouse drag, keyboard shortcuts or how the windows look.

## License

MIT; the vendored Idiomorph ([components/vendor/](components/vendor/)) is under 0BSD.
