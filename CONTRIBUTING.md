# Working on ledge

## Building

ledge needs Go and Xcode's command line tools: the bar's window shim is Objective-C,
built through cgo against AppKit and WebKit, so it builds only on macOS. The bar's
windows, pages and themes are [ledge-frontend](https://github.com/TheWozard/ledge-frontend),
a Go module that `go.mod` takes from GitHub.

```sh
go build .                         # or go install .
go run . run --headless
```

A build from a checkout reports its version as `dev`, with its commit (`+dirty` if it
had changes), and won't `ledge update`.

The [justfile](justfile) builds ledge from this repo into the state dir and runs it:
`just start`, `just stop` and `just restart` pass on any flags to `ledge start`, `stop`
or `restart`. `just kill` stops every ledge built from this repo by its process,
whatever state dir it uses. Run `just` to list the recipes.

## Examples

[scripts/](scripts/) puts example events on the bar. The scripts call the API with
`curl --unix-socket` through the helpers in [scripts/lib.sh](scripts/lib.sh), as a
script of yours could. They send to the ledge of `$XDG_STATE_HOME`, so point them at a
scratch instance:

```sh
export XDG_STATE_HOME=/tmp/ledge-scratch
go run . run &
scripts/bar.sh
```

| Script | Shows |
|---|---|
| `bar.sh [CARDS]` | cards of every kind: each priority, a description, a clamped long title, a report, a url, a command (with commands on), a colour schedule, and HTML-looking text shown as plain text. Extra CARDS make the row scroll sideways. |
| `report.sh` | a report with one of each block, notes in every colour and an https link; plus a long report to scroll |
| `feed.sh [SECONDS]` | a feed's section, replaced by a smaller one after a pause (default 5s), whose card then expires in 1 minute |

`bar.sh` and `feed.sh` write the pages their cards' `file://` urls open under
`$TMPDIR/ledge-examples/`. Each script takes `clear` to remove what it sent;
`scripts/clear.sh` removes what all three sent, and `scripts/clear.sh --all` empties the
bar. `just bar`, `just report`, `just feed` and `just clear` run them.

## Layout

The bar and report windows (the native shim, window placement, pages, components and
themes) are ledge-frontend. ledge is everything else:

| Path | What it is |
|---|---|
| `main.go` | `main`, which hands off to `internal/cli` |
| `internal/cli/` | the CLI, one file per subcommand (cobra) |
| `internal/app/` | `ledge run`: the store, the API on the socket, pruning, and the bar unless headless |
| `internal/api/` | the HTTP server on the socket, and the client the CLI uses |
| `internal/store/` | the `Event` type sent events are checked and decoded into, the event and section schemas, and the events file |
| `internal/schema/` | compiles a directory of JSON Schemas and picks the error to report |
| `internal/bar/` | connects the store to ledge-frontend's bar: events as cards, reports as blocks, and what clicking asks for by id |
| `internal/config/` | the `--config` file and its schema |
| `internal/background/` | `ledge start`, `stop` and `restart`: starting a background ledge, and finding and stopping one through its socket |
| `internal/update/` | `ledge update`: finding the latest release, checking its checksum, replacing the binary |
| `internal/state/` | the state dir and the socket in it |
| `internal/version/` | the version: from `-ldflags` at release time, or Go's build info |

The schemas in `internal/store/` are the API's contract: `ledge schema event` prints
the event's, and the README points there rather than repeating it.

## Testing

```sh
go test ./...
```

Never test against the bar you're using. Use a scratch state dir, which gives the test
instance its own socket, and stop it the same way:

```sh
XDG_STATE_HOME=/tmp/ledge-test ledge start --headless
XDG_STATE_HOME=/tmp/ledge-test ledge stop
```

A socket's path must fit in 103 bytes, so keep the directory short: `t.TempDir()` and
some `$TMPDIR`s are too long, so the tests use `os.MkdirTemp("", "ledge")`.

Some of the bar's tests open a real window, so they only run when asked:

```sh
LEDGE_GUI_TEST=1 go test ./internal/bar
```

They check that the bar reaches ledge: a click dismisses from the store, opens a report
or url, or runs a command. They read the pages' DOM through ledge-frontend's shim and
click from JavaScript; `open` is recorded rather than run, so they never open your
browser, but commands do run. How the bar and reports look and behave is tested in
ledge-frontend.

[CI](.github/workflows/ci.yml) checks gofmt, `go mod tidy`, `go vet` and staticcheck,
runs the tests with `-race`, and shellchecks the scripts. It runs the window tests too,
in a job whose failure doesn't fail the run, as a runner's window server can be flaky.

## Releasing

Pushing a semver tag runs [release.yml](.github/workflows/release.yml) on a macOS
runner. GoReleaser ([.goreleaser.yaml](.goreleaser.yaml)) builds `ledge` for
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
