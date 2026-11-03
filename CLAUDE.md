# ledge

ledge is everything outside `internal/frontend/`: the CLI, the API, the event store
and `internal/bar`, which turns events into what the frontend shows. The frontend,
`internal/frontend/`, is the bar's and reports' windows, pages and themes; it was its
own module, ledge-frontend, until it was merged in. [CONTRIBUTING.md](CONTRIBUTING.md)
and [internal/frontend/README.md](internal/frontend/README.md) say more.

## Themes

`default` is the only built-in theme for now. The others are parked in
`internal/frontend/parked-themes/` (git-ignored, same layout as
`internal/frontend/theme/themes/`) while the core features change. Don't edit them or
keep them in step; `just preview internal/frontend/parked-themes/<name>.json` still
opens one.

## Testing

Run the narrowest tests that cover the change. Most changes need only `just test`.
Paths under the frontend are from `internal/frontend/`.

| Changed | Run | Takes |
|---|---|---|
| anything (always, first) | `just test`: generates the templ files, lints the CSS and checks the Go is gofmt'd (`just lint`), then the tests that don't open windows | ~5s |
| one theme's files: `theme/themes/<name>.json`, `css/<name>.css`, `css/<name>.report.css` | `just gui-themes <name>`: that it loads | ~1s a theme |
| `components/*.templ`, `components/*.go`, `components/page/*`, `ui/*.go`, or `internal/bar/` | `just gui`: every window test, the frontend's (with the per-theme ones in the default theme only), then ledge's bar's | ~20s |
| `shim/` | `go test ./internal/frontend/shim`, then `just gui` | |

- No test checks how anything looks (computed styles, layout, positions, animations).
  The window tests check behaviour; the per-theme tests check only that a theme loads.
  Don't add style or layout assertions; check a look with `just inspect` instead.
- `just gui-all` runs every window test in every theme. Run it only when asked.
- To run one test: `LEDGE_GUI_TEST=1 go test -count=1 -run 'TestClicks' ./internal/frontend/ui`.
  `LEDGE_THEMES=default` limits the per-theme tests to those themes. The per-theme
  tests are listed in `theme_tests` in the justfile.
- Run a window test once. If it fails, read that run's output, report the failure
  and ask before running it again. Don't loop the suite to chase a failure that
  only happens sometimes.
- The window tests put windows on screen and can't see how they look. Screenshots
  aren't possible from here, so to check a look, ask for `just preview <theme>` to be
  run, or `just inspect <theme>` to see the pages in the browser's inspector.
- After editing a `.templ` file, `just check-generated` confirms the committed
  `*_templ.go` files match it. Generate from the repo's root: the generated files
  name their `.templ` files by their path from it.
- The CSS lint is Biome (`biome.json`), and a warning fails it. Fix what it flags
  rather than turning a rule off; if a rule is wrong for this code, ask first.
  `just lint` runs it and the gofmt check alone; `just fmt` fixes what gofmt flags.
- Never try ledge against the bar in use. Run it with a scratch state dir with a
  short path (a socket's path must fit in 103 bytes), such as
  `XDG_STATE_HOME=$(mktemp -d /tmp/lg.XXXX) ledge start --headless`, and stop it the
  same way.

## Committing

Before every commit, amend or history rewrite, propose the commit message and ask
for confirmation. Commit only once it's confirmed, with the message as agreed.
