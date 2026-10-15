#!/usr/bin/env bash
# A feed: sends a section of three items, then after a pause replaces it with a
# smaller one, whose card expires in 1 minute. The first section's items should all
# go at the swap, and the expiring card within 90 seconds of it (ledge drops expired
# events every 30 seconds).
#
# A card opens a local page this script writes; the others have reports, which differ
# before and after the swap.
#
#   feed.sh [SECONDS]   pause SECONDS before the swap (default 5)
#   feed.sh clear       remove everything this script sent, and its pages
#
# Sends with $LEDGE (default: ledge on PATH, else built from this repo) to
# $LEDGE_PORT (default 47391).

set -euo pipefail

REPO_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
if [[ -z "${LEDGE:-}" ]]; then
  LEDGE="$(command -v ledge || true)"
  if [[ -z "$LEDGE" ]]; then
    LEDGE="${TMPDIR:-/tmp}"; LEDGE="${LEDGE%/}/ledge-examples/ledge"
    (cd "$REPO_DIR" && go build -o "$LEDGE" ./ledge)
  fi
fi
SOURCE="example-feed"
PAGES="${TMPDIR:-/tmp}"; PAGES="${PAGES%/}/ledge-examples/feed"
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
seconds="${1:-5}"

link="$(url "$(page "$PAGES/item-2.html" "Feed item 2" "<p>Item 2's url.</p>")")"
send '{"source": "example-feed", "items": [
  {"id": "feed-1", "title": "Feed item 1", "description": "Opens the feed report.",
   "report": [{"type": "text", "text": "Feed report, first version: sent with items 1 and 2."}]},
  {"id": "feed-2", "title": "Feed item 2", "priority": "medium", "description": "Opens a local page.", "url": "'"$link"'"},
  {"kind": "status", "id": "feed-status", "title": "Feed", "color": "yellow"}]}'
echo "Sent the feed; replacing it in ${seconds}s..." >&2
sleep "$seconds"

send '{"source": "example-feed", "items": [
  {"id": "feed-3", "title": "Feed item 3 (replaced 1 and 2)", "expires": "1m",
   "description": "Expires 1 minute after it was sent. Opens the second report.",
   "report": [{"type": "text", "text": "Feed report, second version: sent when item 3 replaced items 1 and 2."}]},
  {"kind": "status", "id": "feed-status", "title": "Feed", "color": "green"}]}'
echo "Replaced the feed" >&2
