#!/usr/bin/env bash
# Example cards and status pills on the bar. Cards that open something open local
# pages this script writes: a report (shown in the report window) or a file:// url
# (opened in your browser), so they work offline.
#
#   bar.sh [CARDS [PILLS]]   the examples, plus CARDS extra cards and PILLS extra pills
#                            (so the row scrolls sideways and the pills vertically)
#   bar.sh clear             remove everything this script sent, and its pages
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
PAGES="${TMPDIR:-/tmp}"; PAGES="${PAGES%/}/ledge-examples/bar"
send() { "$LEDGE" event "$1" >/dev/null; }

# page FILE TITLE [HTML]: writes a small HTML page, stamped with the time it was
# written so a fresh load shows. Prints the file's path.
page() {
  mkdir -p "$(dirname "$1")"
  cat > "$1" <<HTML
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>$2</title>
<style>
  body { font: 14px/1.5 -apple-system, sans-serif; margin: 24px; color: #1d1d1f; }
  @media (prefers-color-scheme: dark) { body { background: #1d1d1f; color: #ececef; } }
</style>
</head>
<body>
<h1>$2</h1>
${3:-}
<p><small>Written by $(basename "$0") at $(date +%T).</small></p>
</body>
</html>
HTML
  echo "$1"
}

# url FILE: a file:// url for a local file.
url() { echo "file://${1// /%20}"; }

if [[ "${1:-}" == "clear" ]]; then
  send "{\"source\": \"$SOURCE\", \"items\": []}"
  rm -rf "$PAGES"
  echo "Cleared $SOURCE" >&2
  exit
fi
cards="${1:-0}"
pills="${2:-0}"

# Cards: each priority, tags, a description, a long title that clamps, a report, a
# url, and text that looks like HTML but must show as plain text.
report="$(page "$PAGES/deploy.html" "Deploy to production failed" \
  "<p>The migration step timed out after 10 minutes.</p><pre>migrate: context deadline exceeded</pre>")"
send '{"id": "bar-high", "source": "example-bar", "priority": "high", "title": "Deploy to production failed",
       "description": "The migration step timed out after 10 minutes. Click for the report.", "tags": ["deploy", "prod"],
       "report": "'"$report"'"}'
send '{"id": "bar-medium", "source": "example-bar", "priority": "medium", "title": "Review requested: fix flaky login test",
       "description": "@someone asked for your review", "tags": ["review"]}'
send '{"id": "bar-low", "source": "example-bar", "priority": "low", "title": "Weekly dependency update is ready",
       "tags": ["deps"]}'
send '{"id": "bar-plain", "source": "example-bar", "title": "A card with no priority, tags or description"}'
send '{"id": "bar-long", "source": "example-bar", "title": "A very long title that keeps going well past the width of a card so it has to clamp to two lines rather than pushing the meta line out of the bottom of the card",
       "description": "And a long description under it, which gives way first when the card runs out of height, so the title and the meta line stay visible no matter how much text is sent."}'
link="$(url "$(page "$PAGES/url.html" "Opened from a card's url" "<p>This page opened in your browser with <code>open</code>.</p>")")"
send '{"id": "bar-url", "source": "example-bar", "title": "Opens a local page in your browser", "url": "'"$link"'",
       "description": "A file:// url, opened with open."}'
send '{"id": "bar-html", "source": "example-bar", "title": "<b>Not bold</b> <img src=x onerror=alert(1)>",
       "description": "Event text is untrusted, so this should show the tags as text."}'

# Pills: plain, a colour schedule, a command, a url and a hex colour.
send '{"kind": "status", "id": "bar-ok", "source": "example-bar", "title": "Build", "color": "green",
       "detail": "main is green"}'
send '{"kind": "status", "id": "bar-session", "source": "example-bar", "title": "Session", "color": "green",
       "schedule": [{"at": "1m", "color": "yellow"}, {"at": "2m", "color": "red"}],
       "detail": "Turns yellow in 1 minute and red in 2. Hover to see the changes to come."}'
send '{"kind": "status", "id": "bar-command", "source": "example-bar", "title": "Log in", "color": "blue",
       "command": "echo logged in", "detail": "Clicking runs the command; its output goes to commands.log."}'
# Only http(s) urls make a pill's label clickable, so this one can't be a local page.
send '{"kind": "status", "id": "bar-link", "source": "example-bar", "title": "Status page", "color": "gray",
       "url": "https://example.com", "detail": "Clicking the label opens https://example.com."}'
send '{"kind": "status", "id": "bar-hex", "source": "example-bar", "title": "Custom colour", "color": "#9b59b6"}'

# Extra cards take turns: a report, a url, then neither.
priorities=(high medium low "")
for i in $(seq 1 "$cards"); do
  priority="${priorities[$(( i % 4 ))]}"
  case $(( i % 3 )) in
    1) opens=", \"report\": \"$(page "$PAGES/card-$i.html" "Card $i of $cards" "<p>The report for card $i.</p>")\"" ;;
    2) opens=", \"url\": \"$(url "$(page "$PAGES/card-$i-url.html" "Card $i of $cards" "<p>The url for card $i.</p>")")\"" ;;
    *) opens="" ;;
  esac
  send "{\"id\": \"bar-card-$i\", \"source\": \"$SOURCE\", \"title\": \"Card $i of $cards\"${priority:+, \"priority\": \"$priority\"}$opens}"
done
colors=(green yellow red blue gray)
for i in $(seq 1 "$pills"); do
  send "{\"kind\": \"status\", \"id\": \"bar-pill-$i\", \"source\": \"$SOURCE\", \"title\": \"Pill $i of $pills\", \"color\": \"${colors[$(( i % 5 ))]}\"}"
done
echo "Sent 7 cards and 5 pills, plus $cards extra cards and $pills extra pills; pages in $PAGES" >&2
