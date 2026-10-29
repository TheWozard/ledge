# ledge

A desktop bar for macOS, along one edge of the screen. Scripts, hooks and CI push
events to a JSON API on a Unix socket, which checks everything it's sent against JSON
Schemas. Each event is a card, of one of several kinds (timers, progress, stats,
checks, messages); clicking one opens its report or its url, or runs its command.

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
ledge version                      # this ledge's version, and the running one's
```

Or with Go and Xcode's command line tools: `go install github.com/TheWozard/ledge@latest`.
To build from a clone, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Usage

```sh
ledge start                        # the bar and the API, in the background; --config, --headless
ledge event '{"title": "Deploy finished", "priority": "high", "expires": "4h"}'
some-feed | ledge event            # or JSON on stdin
ledge list                         # the current events, as JSON
ledge dismiss <id>...
ledge schema event                 # what an event can be: the full reference
ledge stop
```

`ledge run` runs it in the terminal instead, with the same flags. `--headless` runs only
the API, without the bar.

ledge keeps its socket (`ledge.sock`), its events (`events.json`) and its log in the
state dir, `${XDG_STATE_HOME:-~/.local/state}/ledge`, which only your user can open. An
events file it can't read, such as one from an older version, is kept as
`events.json.bad`, and ledge starts with none.
Each state dir is its own ledge: set `XDG_STATE_HOME` to run another, and to talk to it.

## Events

Every event has a `title`, and can have a `description`, a `priority` (`high`, `medium`
or `low`, which sorts it), a `color` (`green`, `yellow`, `red`, `blue`, `gray` or
`#rrggbb`; otherwise its priority's), a `schedule` of colour changes, and an `expires`
time, when it goes. Its `kind` says what else it shows:

| Kind | Shows |
|---|---|
| `basic` (default) | the title and description |
| `timer` | a countdown to `end`, or a count up from `start` |
| `progress` | how far along something is: `value` from 0 to 1, or a `label` such as `"3/5"` |
| `stat` | a `value`, how it moved (`change`), and a sparkline of its `history` |
| `checks` | a mark and name for each of its `checks`, such as a CI run's jobs |
| `message` | who it's `from`, when it was `sent`, and their `avatar` |

`ledge schema event` prints the full reference: every property, and what each kind
takes. Times are ISO 8601, or a duration from when the event is sent, such as `"30m"`.

Send an event with an `id` to update it later by sending it again; without one, ledge
makes one up and prints it. An id belongs to the source that sent it (`source`, or
`manual`), and a section, `{"source", "items"}`, replaces all of one source's events:
that's how a feed keeps its cards current.

A login that expires, turning yellow then red as it runs out, and logging in again
when clicked (with commands on, below):

```json
{"kind": "timer", "id": "aws-auth", "title": "AWS", "end": "1h", "color": "green", "command": "aws sso login",
 "schedule": [{"at": "50m", "color": "yellow"}, {"at": "1h", "color": "red"}]}
```

### Clicking

A card does at most one thing when clicked; the × on it dismisses it.

- **A `url`** opens with `open`, in whatever app handles it.
- **A `report`** opens in its own window (below).
- **A `command`** runs with `bash -lc` in your home directory, in its own session so it
  outlives ledge, with its output appended to `commands.log` in the state dir. To
  update the card, have the command send the event again.

Commands are off unless the config has `"commands": true`, and until then an event with
a `command` is refused. Anything running as you can send events, so turning commands
on lets anything running as you put a command on your bar, under any title: only turn
it on if you trust what sends to ledge.

### Reports

An event's `report` is its details as a list of blocks, drawn top to bottom in the bar's
theme. You send what to show; ledge decides how it looks. Text is always shown as text,
never as HTML. Each card's report has its own window, where Cmd-C, Cmd-A and Cmd-W work.

```json
{"title": "CI: 2 failures on main", "report": [
  {"type": "note", "text": "Two jobs failed after the last merge.", "color": "red"},
  {"type": "fields", "items": [{"label": "Branch", "value": "main"}, {"label": "Failed", "value": 2}]},
  {"type": "heading", "text": "Jobs"},
  {"type": "table", "columns": ["Job", "State", ""], "rows": [
    ["build", {"type": "strong", "text": "failed"}, {"type": "buttons", "items": [
      {"title": "Logs", "url": "https://example.com/build/logs"},
      {"title": "Retry", "color": "red", "command": "gh run rerun 1234 --failed"}]}],
    ["test", "passed"]]},
  {"type": "details", "summary": "build output", "blocks": [
    {"type": "code", "text": "internal/store/store.go:42: undefined: Refuse"}]},
  {"type": "list", "items": ["Fix the build", {"text": "Merge again", "items": ["after review"]}], "ordered": true},
  {"type": "buttons", "items": [{"title": "Done", "color": "green", "dismiss": true}]}
]}
```

The blocks are `heading` (`level` 1 to 6, default 2), `text`, `fields`, `table`, `list`
(an item can have a list under it), `code`, `note`, `quote`, `link` (http(s) only),
`buttons`, `image` (a base64 `data:` url), `details` (blocks hidden until clicked) and
`rule`.

Text in a block can be a string, a span, or an array of strings and spans run together:
`strong`, `em`, `mono` and `link` spans, as in `["Run ", {"type": "mono", "text": "make"},
" or see ", {"type": "link", "text": "the docs", "url": "https://example.com"}]`. Strong,
em and link spans can hold spans of their own. A table cell or a field's value can also be
a number, or a row of buttons.

A button opens its `url` (with `open`, as a card's does), runs its `command` (only with
`commands` on, as a card's does; otherwise it's disabled), or dismisses its event with
`"dismiss": true`. `ledge schema event` describes it all.

## Config

`--config` names a JSON file of settings, or without it, `$LEDGE_CONFIG` does; with
neither, ledge uses the defaults below. `--config ""` is none, whatever the environment
says. The file is read when ledge starts, so `ledge restart` after changing it, or after
changing `$LEDGE_CONFIG`: `restart` keeps the running ledge's config unless one of them
names another. Every key is optional, and an unknown key or a bad value stops ledge
from starting:

```json
{"screen": 0, "edge": "bottom", "height": 110, "level": "floating", "theme": "default", "commands": false}
```

| Key | What it sets |
|---|---|
| `screen` | which screen, as an index; 0 (the default) has the menu bar |
| `edge` | `bottom` (default) or `top` |
| `height` | the bar's height in points; default 110 |
| `level` | which windows the bar stays above: `top` for everything, the Dock included; `floating` for other apps' windows; `normal` (default) for none |
| `theme` | how the bar and reports look: `default`, rounded cards, light or dark with the system. Themes live in [ledge-frontend](https://github.com/TheWozard/ledge-frontend). |
| `commands` | `true` to let events carry a `command` (above); default `false` |

## Running in the background

```sh
ledge start                        # same flags as `ledge run`
ledge stop
ledge restart                      # rereads the config; keeps the running ledge's flags unless given new ones
```

A state dir's ledge is whatever answers on its socket, however it was started. `start`
does nothing if one already answers, and shows the log if a new one exits before it
answers. `stop` asks it to shut down, and kills it if it still answers after 5 seconds.
`restart` checks the config before stopping anything, so a broken config leaves the
running ledge running.

## API

`ledge event`, `list` and `dismiss` are a thin layer over an HTTP API on the socket.
Any HTTP client that can talk to a Unix socket can use it; the host name is ignored:

```sh
curl --unix-socket ~/.local/state/ledge/ledge.sock http://ledge/events -d '{"title": "Hello"}'
```

| Request | Body | Reply |
|---|---|---|
| `POST /events` | an event | `{"id"}` |
| `PUT /sources/{source}` | `{"items": [...]}`: events that replace all of that source's | `{"ids"}` |
| `DELETE /events/{id}` | | `{"dismissed": true}`, or a 404 if there's no such event |
| `DELETE /events` | | `{"dismissed": n}`: every event goes, whoever sent it |
| `GET /events` | | `{"events": [...]}`, highest priority first, then newest first |
| `GET /schemas/{name}` | | the schema: `event` or `section` |
| `GET /status` | | `{"version", "pid", "started", "options"}`; `options` are its flags, `{"config", "headless"}` |
| `POST /shutdown` | | `{"shutdown": true}`, then ledge exits, closing the bar |

Bodies are JSON, up to 8 MiB. A refused request gets a 400 with `{"error"}` saying which
schema rule it broke. A web page can't reach a Unix socket, and the socket is only open
to your user, so nothing else can post events.

[scripts/](scripts/) has examples that put events on the bar with curl; see
[CONTRIBUTING.md](CONTRIBUTING.md#examples).

## License

[MIT](LICENSE)
