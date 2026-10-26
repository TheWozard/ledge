#!/usr/bin/env bash
# Remove example events from the bar.
#
#   clear.sh         remove everything bar.sh, report.sh and feed.sh sent and the pages
#                    they wrote, and nothing else
#   clear.sh --all   remove every event on the bar, whoever sent it
#
# Sends to the API socket of $XDG_STATE_HOME's ledge with curl.

set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

case "${1:-}" in
  "")
    # A section with no items clears its source.
    for source in example-bar example-report example-feed; do
      section "$source" '[]'
    done
    rm -rf "$(pages bar)" "$(pages feed)"
    echo "Cleared the examples" >&2
    ;;
  --all)
    api DELETE /events >/dev/null
    echo "Cleared the bar" >&2
    ;;
  *)
    echo "usage: clear.sh [--all]" >&2
    exit 2
    ;;
esac
