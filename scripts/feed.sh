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
# Sends to the API socket of $XDG_STATE_HOME's ledge with curl.

set -euo pipefail

# shellcheck source=scripts/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SOURCE="example-feed"
PAGES="$(pages feed)"

if [[ "${1:-}" == "clear" ]]; then
  section "$SOURCE" '[]'
  rm -rf "$PAGES"
  echo "Cleared $SOURCE" >&2
  exit
fi
seconds="${1:-5}"

link="$(url "$(page "$PAGES/item-2.html" "Feed item 2" "<p>Item 2's url.</p>")")"
section "$SOURCE" '[
  {"id": "feed-1", "title": "Feed item 1", "description": "Opens the feed report.",
   "report": [{"type": "text", "text": "Feed report, first version: sent with items 1 and 2."}]},
  {"id": "feed-2", "title": "Feed item 2", "priority": "medium", "description": "Opens a local page.", "url": "'"$link"'"},
  {"kind": "progress", "id": "feed-progress", "title": "Feed", "value": 0.5, "label": "1/2", "color": "yellow"}]'
echo "Sent the feed; replacing it in ${seconds}s..." >&2
sleep "$seconds"

section "$SOURCE" '[
  {"id": "feed-3", "title": "Feed item 3 (replaced 1 and 2)", "expires": "1m",
   "description": "Expires 1 minute after it was sent. Opens the second report.",
   "report": [{"type": "text", "text": "Feed report, second version: sent when item 3 replaced items 1 and 2."}]},
  {"kind": "progress", "id": "feed-progress", "title": "Feed", "value": 1, "label": "2/2", "color": "green"}]'
echo "Replaced the feed" >&2
