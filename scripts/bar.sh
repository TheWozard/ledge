#!/usr/bin/env bash
# Example cards of every kind on the bar. A card opens a report (shown in a report
# window), a file:// url of a local page this script writes (opened in your
# browser), so they work offline, or runs a command.
#
#   bar.sh [CARDS]   the examples, plus CARDS extra cards (so the row scrolls sideways)
#   bar.sh clear     remove everything this script sent, and its pages
#
# Sends with $LEDGE (default: ledge on PATH, else built from this repo) to the ledge
# of $XDG_STATE_HOME. Try it on a scratch instance:
#
#   export XDG_STATE_HOME=/tmp/ledge-scratch
#   go run ./ledge run &
#   scripts/bar.sh 15

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

# Cards: each priority, a description, a long title that clamps, a report, a
# url, and text that looks like HTML but must show as plain text.
send '{"id": "bar-high", "source": "example-bar", "priority": "high", "title": "Deploy to production failed",
       "description": "The migration step timed out after 10 minutes. Click for the report.",
       "report": [{"type": "text", "text": "The migration step timed out after 10 minutes."},
                  {"type": "code", "text": "migrate: context deadline exceeded"}]}'
send '{"id": "bar-medium", "source": "example-bar", "priority": "medium", "title": "Review requested: fix flaky login test",
       "description": "@someone asked for your review"}'
send '{"id": "bar-low", "source": "example-bar", "priority": "low", "title": "Weekly dependency update is ready"}'
send '{"id": "bar-plain", "source": "example-bar", "title": "A card with no priority or description"}'
send '{"id": "bar-long", "source": "example-bar", "title": "A very long title that keeps going well past the width of a card so it has to clamp to two lines rather than pushing the meta line out of the bottom of the card",
       "description": "And a long description under it, which gives way first when the card runs out of height, so the title and the meta line stay visible no matter how much text is sent."}'
link="$(url "$(page "$PAGES/url.html" "Opened from a card's url" "<p>This page opened in your browser with <code>open</code>.</p>")")"
send '{"id": "bar-url", "source": "example-bar", "title": "Opens a local page in your browser", "url": "'"$link"'",
       "description": "A file:// url, opened with open."}'
send '{"id": "bar-html", "source": "example-bar", "title": "<b>Not bold</b> <img src=x onerror=alert(1)>",
       "description": "Event text is untrusted, so this should show the markup as text."}'

# The other kinds, a colour schedule and a command.
send '{"kind": "timer", "id": "bar-session", "source": "example-bar", "title": "Session", "color": "green",
       "end": "2m", "schedule": [{"at": "1m", "color": "yellow"}, {"at": "2m", "color": "red"}],
       "description": "Yellow in 1 minute, red in 2."}'
send '{"kind": "timer", "id": "bar-incident", "source": "example-bar", "title": "Incident open", "priority": "high",
       "start": "'"$(date -u -v-7M +%Y-%m-%dT%H:%M:%SZ)"'", "description": "Counts up."}'
send '{"kind": "progress", "id": "bar-progress", "source": "example-bar", "title": "Deploy", "value": 0.6,
       "label": "3/5", "description": "Rolling out"}'
send '{"kind": "stat", "id": "bar-stat", "source": "example-bar", "title": "Error rate", "value": "2.3%",
       "change": "↑0.8", "change_color": "red", "history": [1.2, 1.4, 1.1, 1.5, 1.9, 2.3], "description": "last hour"}'
send '{"kind": "checks", "id": "bar-checks", "source": "example-bar", "title": "CI on main",
       "checks": [{"name": "lint", "state": "passed"}, {"name": "test", "state": "failed"},
                  {"name": "build", "state": "running"}, {"name": "deploy", "state": "pending"},
                  {"name": "docs", "state": "skipped"}]}'
send '{"kind": "message", "id": "bar-message", "source": "example-bar", "title": "#deploys", "from": "Ada Lovelace",
       "sent": "'"$(date -u -v-5M +%Y-%m-%dT%H:%M:%SZ)"'", "description": "Rolling back the 14:02 deploy now."}'
# Refused unless the ledge's config has "commands": true.
send '{"id": "bar-command", "source": "example-bar", "title": "Log in", "color": "blue",
       "command": "echo logged in", "description": "Clicking runs the command; its output goes to commands.log."}' \
  2>/dev/null || echo "Skipped the command card: set \"commands\": true in the ledge's --config to try it" >&2

# Extra cards take turns: a report, a url, then neither.
priorities=(high medium low "")
for i in $(seq 1 "$cards"); do
  priority="${priorities[$(( i % 4 ))]}"
  case $(( i % 3 )) in
    1) opens=", \"report\": [{\"type\": \"text\", \"text\": \"The report for card $i.\"}]" ;;
    2) opens=", \"url\": \"$(url "$(page "$PAGES/card-$i-url.html" "Card $i of $cards" "<p>The url for card $i.</p>")")\"" ;;
    *) opens="" ;;
  esac
  send "{\"id\": \"bar-card-$i\", \"source\": \"$SOURCE\", \"title\": \"Card $i of $cards\"${priority:+, \"priority\": \"$priority\"}$opens}"
done
echo "Sent the example cards, plus $cards extra cards; pages in $PAGES" >&2
