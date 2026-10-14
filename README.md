# ledge

An always-on-top desktop bar for macOS. Scripts, hooks and CI push events and status
pills to a JSON API on `127.0.0.1`, which checks everything it's sent against JSON
Schemas. It replaces desktop-view and keeps its HTTP contract. [PLAN.md](PLAN.md)
covers the design and the milestones.

Status: milestone 1. The API, the store and `ledge event` work, and `ledge run
--headless` runs the API without a window. The bar isn't built yet.

## Usage

```sh
go install ./ledge                 # puts ledge in $(go env GOPATH)/bin

ledge run --headless               # the API only, without the bar; --port, --config
ledge event '{"title": "Deploy finished", "priority": "high", "expires": "4h"}'
some-feed | ledge event            # or JSON on stdin
ledge event --list
ledge event --dismiss <id>
ledge event --schema event         # print a schema: dismiss, event, report, section, status
```

The API listens on port 47391; set `$LEDGE_PORT` to change it. Events are kept in
`${XDG_STATE_HOME:-~/.local/state}/ledge/events.json`.

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
| `internal/store/` | the event, status, report, section and dismiss schemas, normalizing events, and the event file |
| `internal/api/` | the HTTP server and the client `ledge event` uses |
| `internal/config/` | the `--config` file and its schema |
| `internal/state/` | the state directory and port |
| `internal/version/` | version metadata, set with `-ldflags` at release time |

## Testing

Never test against the bar you're using. Use a scratch port and state directory:

```sh
LEDGE_PORT=47392 XDG_STATE_HOME=/tmp/ledge-test ledge run --headless
```

Stop test instances by pid, never by name.
