# Restructuring plan

From the codebase review. Work through the phases in order; each one ends green
before the next starts. Commit the current theme-parking work first, so every move
below is a clean diff of its own.

## 0. Before starting

- [x] Commit the working tree (theme parking, `cmd/preview/browser.go`, `CLAUDE.md`).

## 1. Bugs

- [x] **Timer width ignores the theme.** `Timer.Frame()` hardcodes `Width: 100`
  ([components.go:113](components/components.go#L113)); a card's own width is inline
  and beats `li.timer { --card-width: 180px }` in `default.css`. Drop it, or give
  `Timer` a `Width` field that defaults to 0 like `Basic`.
- [x] **Action and note colours disagree.** An action's `data-color` is its raw
  `Color` (`""`, `"purple"`) while `--action` falls back to gray; a note maps `""`
  to `"gray"`. Add one `colorName(color string) string` (named colour, `#rrggbb`, or
  `"gray"`) and use it for both `data-color`s; replace `noteColor`.
- [x] **App code under `b.mu`.** `Render` and `draw` call `Frame()`/`Body()` with the
  lock held. Compute frames before locking, or say in the `Card` doc that `Frame`
  and `Body` mustn't call back into the bar.
- [x] **`Open` race.** Replace the `opened.Load()` … `opened.Store(true)` pair
  with `opened.CompareAndSwap(false, true)`, resetting it if `Open` then fails.

Verify: `just test`, `just gui`.

## 2. Stale docs (from parking the themes)

- [x] [README.md](README.md) and the `Manifest` / `Load` doc comments in
  [theme.go](theme/theme.go): replace the `handheld`/`silkscreen` example with one
  that exists (or a neutral one), drop the `OFL.txt` link and the bundled-fonts
  licence line.
- [x] [bar.templ](components/bar.templ) and [report.templ](components/report.templ):
  `theme.CSS` doesn't exist (say `theme.Theme.Bar` / `.Report`); `theme.Frames` is
  `theme.Frame`.
- [x] [shim.m:262-268](shim/shim.m#L262-L268): move the `titledMask` comment above
  `titledMask`, off `cascade`.

Verify: `just test`, `just check-generated`.

## 3. `components/`

- [x] **Move the page CSS and JS out of templ** into embedded files, as Idiomorph
  already is:

  ```
  components/
    page/bar.css     page/bar.js
    page/report.css  page/report.js
    assets.go        // go:embed page/* and vendor/idiomorph.min.js
  ```

  `BarPage` and `ReportPage` inline them with `style`/`script` helpers in `assets.go`. The theming-hooks
  comment moves to the top of `bar.css` / `report.css`.
- [x] **Split `components.go`:**

  | File | Contents |
  |---|---|
  | `card.go` | `Card`, `Frame`, `Action`, `Cards`, `SortByPriority`, `classes`, `style`, `ElementID`, `entering` |
  | `doc.go` | the package doc (was `components.go`) |
  | `click.go` | `Click`, `Clicks`, `clickID` / `dismissID` |
  | `color.go` | `named`, `hex`, `cssColor`, `colorName` |
  | `basic.templ` | `Basic`, its `Frame`/`Body`, `basicBody` |
  | `timer.templ` | `Timer`, its `Frame`/`Body`, `timeLeft`, `timerBody` |
  | `bar.templ` | `BarPage`, `BarContent`, `card`, `action` |
  | `report.templ` | `ReportPage` |
  | `blocks.templ` | `Heading` … `Link`, `Field`, `cell` |

- [x] Add `Action.style(fresh)` to match `Frame.style`, and drop the duplicated
  `style=` branches in the `action` template.
- [x] Split `components_test.go` to follow (`card_test.go`, `timer_test.go`,
  `blocks_test.go`, `page_test.go`, `helpers_test.go`); `TestReportPage` split into `TestReportPage` and `TestBlocks`.

Verify: `just test`, `just check-generated`, `just gui`.

## 4. `ui/`

- [x] **`ui/endings.go`**: an `endings` type (`schedule`, `finish`) holding the
  timers and completed ends, and `Bar.complete`.
- [x] **`ui/message.go`**: `message`, `readMessage`, `handle`.
- [x] **Report windows off the bar's lock**: a `*reportWindows` (`window`, `closed`)
  in `report.go`, with its own mutex.
- [x] **Break up `draw()`**: the new-card/new-action marking is the pure
  `arrivals(cards, shown) (fresh, now)`, unit-tested by `TestArrivals` (runs under
  `just test`).
- [x] **Split `ui_test.go`** (745 lines), by subject rather than by whether a test
  opens a window (each window test skips itself without `LEDGE_GUI_TEST`):
  - `harness_test.go` (was `ui_test.go`): `TestMain`, `recorder`, `eval` /
    `waitFor` / `eventually`, `show`, `withTheme`, `setTheme`, `showReport`,
    `testThemes`, `syncBuffer`
  - `bar_test.go`: rendering, morphing, entering and leaving, themes, the wheel,
    timers counting down, `TestArrivals`
  - `message_test.go`: `TestReadMessage`, clicks, refused and non-JSON messages
  - `report_test.go`: the report windows, and `TestNavigate`
  - `endings_test.go`: `TestComplete`

Verify: `just test`, `just gui`.

## 5. `cmd/preview/`

- [x] **`sample.go`**: the `sample` type and methods, `newSample(now)` and
  `longReport(now)` with the data that was in `main()`, and `longID`. `reportFor`
  is folded into `sample.report`, timers included.
- [x] **`theme.go`**: `loadTheme` and `builtinDir`.
- [x] **`browser.go`**: the index page is the embedded `index.html` (rendered
  with `html/template`, no `%%` escaping), and the handlers are methods on
  `browserServer`.
- [x] `main.go` is left with flags and `showWindows` (opening the windows and the
  signal handling).

Verify: `just test`, then ask for `just preview` and `just inspect` to be run.

## 6. `shim/`

- [ ] C enums in `shim.h` for level and appearance
  (`LedgeLevelNormal/Floating/Top`, `LedgeAppearanceSystem/Light/Dark`), used by
  `SetLevel` / `SetAppearance` in place of the `map[string]int` literals.
- [ ] `goRect(C.LedgeRect) Rect` for `PlaceBar` and `WindowFrame`.
- [ ] One `handlers` struct with a getter in place of the four lock/copy/unlock
  blocks in the `goLedge*` exports.
- [ ] Optional: split `shim.m` into `window.m`, `delegate.m` and `api.m` with a
  private header for the shared dictionaries. It's fine as it is at 427 lines.

Verify: `go test ./shim`, then `just gui`.

## 7. `theme/`

- [ ] Replace the anonymous-struct loop in `load` with
  `joinStylesheets(fsys, dir, files []string) (string, error)`, called once each
  for `Bar` and `Report`.

Verify: `just test`, `just gui-themes default`.
