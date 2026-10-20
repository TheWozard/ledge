# ledge-frontend

## Themes

`default` is the only built-in theme for now. The others are parked in
`parked-themes/` (git-ignored, same layout as `theme/themes/`) while the core features
change. Don't edit them or keep them in step; `just preview parked-themes/<name>.json`
still opens one.

## Testing

Run the narrowest tests that cover the change. Most changes need only `just test`.

| Changed | Run | Takes |
|---|---|---|
| anything (always, first) | `just test`: generates the templ files, then the tests that don't open windows | ~1s |
| one theme's files: `theme/themes/<name>.json`, `css/<name>.css`, `css/<name>.report.css` | `just gui-themes <name>`: that it loads | ~1s a theme |
| `components/*.templ`, `components/*.go`, `components/page/*`, `ui/*.go` | `just gui`: every window test, with the per-theme ones in the default theme only | ~13s |
| `shim/` | `go test ./shim`, then `just gui` | |

- No test checks how anything looks (computed styles, layout, positions, animations).
  The window tests check behaviour; the per-theme tests check only that a theme loads.
  Don't add style or layout assertions; check a look with `just inspect` instead.
- `just gui-all` runs every window test in every theme. Run it only when asked.
- To run one test: `LEDGE_GUI_TEST=1 go test -count=1 -run 'TestClicks' ./ui`.
  `LEDGE_THEMES=default` limits the per-theme tests to those themes. The per-theme
  tests are listed in `theme_tests` in the justfile.
- Run a window test once. If it fails, read that run's output, report the failure
  and ask before running it again. Don't loop the suite to chase a failure that
  only happens sometimes.
- The window tests put windows on screen and can't see how they look. Screenshots
  aren't possible from here, so to check a look, ask for `just preview <theme>` to be
  run, or `just inspect <theme>` to see the pages in the browser's inspector.
- After editing a `.templ` file, `just check-generated` confirms the committed
  `*_templ.go` files match it.
