#!/usr/bin/env bash
# Example cards and status pills on the bar.
#
#   bar.sh [CARDS [PILLS]]   the examples, plus CARDS extra cards and PILLS extra pills
#                            (so the row scrolls sideways and the pills vertically)
#   bar.sh clear             remove everything this script sent
#
# Sends with $LEDGE (default: ledge on PATH, else built from this repo) to
# $LEDGE_PORT (default 47391). Try it on a scratch instance:
#
#   go run ./ledge run --port 47392 &
#   LEDGE_PORT=47392 scripts/bar.sh 15 12

set -euo pipefail

REPO_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
if [[ -z "${LEDGE:-}" ]]; then
  LEDGE="$(command -v ledge || true)"
  if [[ -z "$LEDGE" ]]; then
    LEDGE="${TMPDIR:-/tmp}"; LEDGE="${LEDGE%/}/ledge-examples/ledge"
    (cd "$REPO_DIR" && go build -o "$LEDGE" ./ledge)
  fi
fi
SOURCE="example-bar"
send() { "$LEDGE" event "$1" >/dev/null; }

if [[ "${1:-}" == "clear" ]]; then
  send "{\"source\": \"$SOURCE\", \"items\": []}"
  echo "Cleared $SOURCE" >&2
  exit
fi
cards="${1:-0}"
pills="${2:-0}"

# Cards: each priority, tags, a description, a long title that clamps, a url, and text
# that looks like HTML but must show as plain text.
send '{"id": "bar-high", "source": "example-bar", "priority": "high", "title": "Deploy to production failed",
       "description": "The migration step timed out after 10 minutes.", "tags": ["deploy", "prod"]}'
send '{"id": "bar-medium", "source": "example-bar", "priority": "medium", "title": "Review requested: fix flaky login test",
       "description": "@someone asked for your review", "tags": ["review"]}'
send '{"id": "bar-low", "source": "example-bar", "priority": "low", "title": "Weekly dependency update is ready",
       "tags": ["deps"]}'
send '{"id": "bar-plain", "source": "example-bar", "title": "A card with no priority, tags or description"}'
send '{"id": "bar-long", "source": "example-bar", "title": "A very long title that keeps going well past the width of a card so it has to clamp to two lines rather than pushing the meta line out of the bottom of the card",
       "description": "And a long description under it, which gives way first when the card runs out of height, so the title and the meta line stay visible no matter how much text is sent."}'
send '{"id": "bar-url", "source": "example-bar", "title": "Opens a url when clicked", "url": "https://example.com",
       "description": "Clicking does nothing until milestone 3."}'
send '{"id": "bar-html", "source": "example-bar", "title": "<b>Not bold</b> <img src=x onerror=alert(1)>",
       "description": "Event text is untrusted, so this should show the tags as text."}'

# Pills: plain, a colour schedule, a command, a url and a hex colour.
send '{"kind": "status", "id": "bar-ok", "source": "example-bar", "title": "Build", "color": "green",
       "detail": "main is green"}'
send '{"kind": "status", "id": "bar-session", "source": "example-bar", "title": "Session", "color": "green",
       "schedule": [{"at": "1m", "color": "yellow"}, {"at": "2m", "color": "red"}],
       "detail": "Turns yellow in 1 minute and red in 2. Hover to see the changes to come."}'
send '{"kind": "status", "id": "bar-command", "source": "example-bar", "title": "Log in", "color": "blue",
       "command": "echo logged in", "detail": "Clicking runs the command (from milestone 3)."}'
send '{"kind": "status", "id": "bar-link", "source": "example-bar", "title": "Status page", "color": "gray",
       "url": "https://example.com", "detail": "Clicking the label opens the url (from milestone 3)."}'
send '{"kind": "status", "id": "bar-hex", "source": "example-bar", "title": "Custom colour", "color": "#9b59b6"}'

priorities=(high medium low "")
for i in $(seq 1 "$cards"); do
  priority="${priorities[$(( i % 4 ))]}"
  send "{\"id\": \"bar-card-$i\", \"source\": \"$SOURCE\", \"title\": \"Card $i of $cards\"${priority:+, \"priority\": \"$priority\"}}"
done
colors=(green yellow red blue gray)
for i in $(seq 1 "$pills"); do
  send "{\"kind\": \"status\", \"id\": \"bar-pill-$i\", \"source\": \"$SOURCE\", \"title\": \"Pill $i of $pills\", \"color\": \"${colors[$(( i % 5 ))]}\"}"
done
echo "Sent 7 cards and 5 pills, plus $cards extra cards and $pills extra pills" >&2
