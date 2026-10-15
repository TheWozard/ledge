#!/usr/bin/env bash
# A feed: sends a section of three items, then after a pause replaces it with a
# smaller one, whose card expires in 1 minute. The first section's items should all
# go at the swap, and the expiring card within 90 seconds of it (ledge drops expired
# events every 30 seconds).
#
#   feed.sh [SECONDS]   pause SECONDS before the swap (default 5)
#   feed.sh clear       remove everything this script sent
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
send() { "$LEDGE" event "$1" >/dev/null; }

if [[ "${1:-}" == "clear" ]]; then
  send "{\"source\": \"$SOURCE\", \"items\": []}"
  echo "Cleared $SOURCE" >&2
  exit
fi
seconds="${1:-5}"

send '{"source": "example-feed", "items": [
  {"id": "feed-1", "title": "Feed item 1"},
  {"id": "feed-2", "title": "Feed item 2", "priority": "medium"},
  {"kind": "status", "id": "feed-status", "title": "Feed", "color": "yellow"}]}'
echo "Sent the feed; replacing it in ${seconds}s..." >&2
sleep "$seconds"

send '{"source": "example-feed", "items": [
  {"id": "feed-3", "title": "Feed item 3 (replaced 1 and 2)", "expires": "1m",
   "description": "Expires 1 minute after it was sent."},
  {"kind": "status", "id": "feed-status", "title": "Feed", "color": "green"}]}'
echo "Replaced the feed" >&2
