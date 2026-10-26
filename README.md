# ledge

A desktop bar for macOS, along one edge of the screen. Scripts, hooks and CI push
events and statuses to a JSON API on `127.0.0.1`, which checks everything it's sent
against JSON Schemas. Both show as cards, statuses first; clicking one opens its
report, its url or runs its command.

## Install

ledge runs on macOS, on Apple silicon or Intel. The install script downloads the
latest release for your Mac, checks it against the release's `checksums.txt`, and
puts `ledge` in `~/.local/bin` (set `LEDGE_INSTALL_DIR` to put it elsewhere):

```sh
curl -fsSL https://raw.githubusercontent.com/TheWozard/ledge/main/scripts/install.sh | bash
```

Then keep it up to date with:

```sh
ledge update --check               # say whether there's a newer release
ledge update                       # install it; a running ledge keeps the old one until `ledge restart`
ledge version
```

From source, with Go and Xcode's command line tools: `go install ./ledge`. The bar's
windows, pages and themes are [ledge-frontend](https://github.com/TheWozard/ledge-frontend),
a Go module that `go.mod` takes from GitHub. A source build reports its version as `dev` and won't `update`; install a
release instead.

## Usage

```sh

ledge run                          # the bar and the API; --port, --config
ledge run --headless               # the API only, without the bar
ledge event '{"title": "Deploy finished", "priority": "high", "expires": "4h"}'
some-feed | ledge event            # or JSON on stdin
ledge event --list
ledge event --dismiss <id>
ledge event --schema event         # print a schema: dismiss, event, section, status
```

The API listens on port 47391; set `$LEDGE_PORT` to change it. Events are kept in
`${XDG_STATE_HOME:-~/.local/state}/ledge/events.json`.

### Config

`--config` names a JSON file of window settings, read when ledge starts (so
`ledge restart` after changing it). Every key is optional, and an unknown key or a bad
value stops ledge from starting:

```json
{"screen": 0, "edge": "bottom", "height": 110, "level": "floating", "theme": "default"}
```

| Key | What it sets |
|---|---|
| `screen` | which screen, as an index; 0 (the default) has the menu bar |
| `edge` | `bottom` (default) or `top` |
| `height` | the bar's height in points; default 110 |
| `level` | which windows the bar stays above: `top` for everything, the Dock included; `floating` for other apps' windows but not the Dock; `normal` (default) for none |
| `theme` | how the bar and reports look (below) |

### Clicking

What a click does is looked up from the event's id; the page never passes a url,
path or command itself.

- **A card with a `url`** runs `open <url>`, so it opens in whatever app handles it.
- **A card with a `report`** shows it in the report window (below).
- **A status with a `command`** runs it with `bash -lc` in your home directory, in its
  own session so it outlives ledge. Its output is appended to `commands.log` in the
  state dir, after a line naming the status. Nothing tracks whether it worked; to
  update the status, have the command send it again.
- **A status with an http(s) `url`** opens it.

A status is in the colour of the latest step of its `schedule` that's passed, and one
with a schedule counts down to its last step. An event's `priority` colours its card.

### Reports

An event's `report` is its details as a list of blocks, drawn top to bottom under the
event's title in the bar's theme. You send what to show; ledge decides how it looks.

```json
{"title": "CI: 2 failures on main", "report": [
  {"type": "note", "text": "Two jobs failed after the last merge.", "color": "red"},
  {"type": "fields", "items": [{"label": "Branch", "value": "main"}, {"label": "Failed", "value": 2}]},
  {"type": "heading", "text": "Jobs"},
  {"type": "table", "columns": ["Job", "State"], "rows": [["build", "failed"], ["test", "passed"]]},
  {"type": "code", "text": "internal/store/store.go:42: undefined: Refuse"},
  {"type": "list", "items": ["Fix the build", "Merge again"], "ordered": true},
  {"type": "link", "text": "Build logs", "url": "https://example.com/build/logs"}
]}
```

| Block | Shows |
|---|---|
| `heading` | a section heading |
| `text` | a paragraph; line breaks are kept |
| `fields` | label and value rows |
| `table` | `columns`, then `rows` of cells; short rows are padded, long ones cut |
| `list` | bullets, or numbers with `"ordered": true` |
| `code` | fixed-width text, such as a log; long lines scroll sideways |
| `note` | a callout in a colour, named as for statuses or `#rrggbb` |
| `link` | an http(s) link, which opens in your browser |

The schema (`ledge event --schema event`) checks a report when it's sent. Text is
always shown as text, never as HTML, and the page can't do anything but draw it. To
change a report, send the event again with the same id. Each card's report has its
own window; Cmd-C, Cmd-A and Cmd-W work in it. The theme draws its title bar, which
drags and double-clicks like a real one. The page can ask only that of its window, and nothing of ledge.

### Themes

Pick how the bar looks with `theme` in the `--config` file; it's read when ledge starts,
so `ledge restart` to switch:

```json
{"theme": "default"}
```

| Theme | Looks like |
|---|---|
| `default` | rounded cards, light or dark with the system |

The theme, and the pages it styles, are in ledge-frontend, whose README says how to
write one.

### In the background

```sh
ledge start                        # same flags as `ledge run`: --config, --port, --headless
ledge stop
ledge restart                      # rereads the config; reuses start's flags unless given new ones
```

`start` waits until the API answers, and shows the log if ledge exits first. Its pid
file and log (`ledge.log`) go in the state dir. `stop` only acts on the ledge `start`
began, never on one run in a terminal or a process that has reused its pid, and
`start` refuses if something else already answers on the port. `restart` checks the
config before stopping anything, so a broken config leaves the running ledge running.

While working on ledge, the [justfile](justfile) builds it from this repo first:
`just start`, `just stop` and `just restart` build into the state dir and then run
`ledge start`, `stop` or `restart`, passing on any flags. `just bar`, `just report`,
`just feed` and `just clear` run the examples. Run `just` to list them.

The build in the state dir and an installed `ledge` are different binaries, so each
only stops or restarts the ledge it started itself.

## Examples

[scripts/](scripts/) puts example events on the bar. Each script uses `ledge` from
your `PATH` (or builds one) and sends to `$LEDGE_PORT`, so point them at a scratch
instance to try them:

```sh
go run ./ledge run --port 47392 &
LEDGE_PORT=47392 scripts/bar.sh
```

| Script | Shows |
|---|---|
| `bar.sh [CARDS [STATUSES]]` | cards (each priority, a description, a clamped long title, a report, a url, HTML-looking text shown as plain text) and statuses (plain, a colour schedule, a command, a url, a hex colour). Extra CARDS and STATUSES make the row scroll sideways: `bar.sh 15 12`. Extra cards take turns with a report, a url and neither. |
| `report.sh` | a report with one of each block, notes in every colour and an https link; plus a long report to scroll, whose HTML-looking text shows as text |
| `feed.sh [SECONDS]` | a feed's section, replaced by a smaller one after a pause (default 5s), whose card then expires in 1 minute. The swap sends a new report, so the card's report differs before and after. |

`bar.sh` and `feed.sh` write the pages their card urls open under
`$TMPDIR/ledge-examples/`. The urls are `file://` urls, so clicking a card works offline. A status's url has
to be http(s) to be clickable, so `bar.sh`'s status links to `https://example.com`.

Each takes `clear` to remove what it sent and the pages it wrote, and nothing else:
`scripts/bar.sh clear`.
`scripts/clear.sh` removes what all three sent, and `scripts/clear.sh --all` empties
the bar, whoever sent the events.

## API

| Request | Body | Reply |
|---|---|---|
| `POST /event` | an event, a status, or a section (`{"source", "items"}`) that replaces all of that source's events | `{"id"}`, or `{"ids"}` for a section |
| `POST /dismiss` | `{"id"}` | `{"dismissed": true\|false}` |
| `GET /events` | | `{"events": [...]}`, highest priority first, then newest first |
| `GET /schemas/{name}` | | the schema |

POSTs must be `Content-Type: application/json`, and no request may carry an `Origin`
header, so a web page can't post events. A refused request gets a 400 with
`{"error"}` saying which schema rule it broke.

## Layout

The bar and report windows (the native shim, window placement, pages, components and
themes) are ledge-frontend. ledge is everything else:

| Path | What it is |
|---|---|
| `ledge/` | `main`, which hands off to `cmd` |
| `cmd/` | the CLI, one file per subcommand (cobra) |
| `internal/schema/` | compiles a directory of JSON Schemas and picks the error to report |
| `internal/store/` | the event, status, section and dismiss schemas, normalizing events, and the event file |
| `internal/api/` | the HTTP server and the client `ledge event` uses |
| `internal/config/` | the `--config` file and its schema |
| `internal/background/` | `ledge start`, `stop` and `restart`: the background process, its pid file, log and saved flags |
| `internal/update/` | `ledge update`: finding the latest release, checking its checksum, replacing the binary |
| `internal/bar/` | connects the store to ledge-frontend's bar: events and statuses as cards, reports as blocks, and what the pages ask for by id (dismiss, open a report or url, run a command) |
| `internal/state/` | the state directory and port |
| `internal/version/` | version metadata, set with `-ldflags` at release time |

## Releasing

Pushing a semver tag runs [.github/workflows/release.yml](.github/workflows/release.yml)
on a macOS runner. GoReleaser ([.goreleaser.yaml](.goreleaser.yaml)) builds `ledge` for
darwin/arm64 and darwin/amd64 with cgo, sets the version, and publishes
`ledge_darwin_<arch>.tar.gz` and `checksums.txt` as release assets, with notes from the
commits since the last tag.

```sh
git checkout main && git pull
git tag v0.1.0
git push origin v0.1.0
```

To fix a bad release, delete the GitHub release and the tag, then push a corrected tag:

```sh
git push --delete origin v0.1.0
git tag -d v0.1.0
```

`LEDGE_GITHUB_API` points `ledge update` and `scripts/install.sh` at a stand-in for
`https://api.github.com`, for testing a release without publishing one.

## Testing

Never test against the bar you're using. Use a scratch port and state directory:

```sh
LEDGE_PORT=47392 XDG_STATE_HOME=/tmp/ledge-test ledge run --headless
```

Stop test instances by pid, never by name.

Some of the bar's tests open a real window, so they only run when asked:

```sh
LEDGE_GUI_TEST=1 go test ./internal/bar
```

They check that the bar reaches ledge: a click dismisses from the store, opens a
report or url, or runs a command. They read the pages' DOM through ledge-frontend's
shim, and click from JavaScript; `open` is recorded rather than run, so they never
open your browser, but status commands do run. How the bar and reports look and
behave is tested in ledge-frontend.

## License

[MIT](LICENSE)
