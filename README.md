# ledge

An always-on-top desktop bar for macOS. Scripts, hooks and CI push events and status
pills to a JSON API on `127.0.0.1`, which checks everything it's sent against JSON
Schemas. It replaces desktop-view and keeps its HTTP contract. [PLAN.md](PLAN.md)
covers the design and the milestones.

Status: milestones 1, 2 and 4. `ledge run` shows the bar and runs the API;
`--headless` runs the API alone. Cards and pills render and update live, × dismisses,
and expired events go. Clicking a card or a pill does nothing yet (milestone 3).

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

From source, with Go and Xcode's command line tools: `go install ./ledge`. A source
build reports its version as `dev` and won't `update`; install a release instead.

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
| `bar.sh [CARDS [PILLS]]` | cards (each priority, tags, a description, a clamped long title, a url, HTML-looking text shown as plain text) and status pills (plain, a colour schedule, a command, a url, a hex colour). Extra CARDS and PILLS make the row scroll sideways and the pills vertically: `bar.sh 15 12`. |
| `report.sh` | a card whose report is an HTML file the script writes, with an image and CSS beside it |
| `feed.sh [SECONDS]` | a feed's section, replaced by a smaller one after a pause (default 5s), whose card then expires in 1 minute |

Each takes `clear` to remove what it sent, and nothing else: `scripts/bar.sh clear`.
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
| `internal/shim/` | the native layer (cgo + Objective-C): windows and WKWebView; no app logic |
| `internal/bar/` | the Go side of the bar: placement, rendering, and the page's actions (checked against `message.schema.json`) |
| `internal/pages/` | the HTML pages, built in with `go:embed` |
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

The bar's tests open a real window, so they only run when asked:

```sh
LEDGE_GUI_TEST=1 go test ./internal/bar
```

They read the page's DOM through the shim. They can't check hover, real clicks
or how the bar looks; those need a person.

## License

[MIT](LICENSE)
