#!/usr/bin/env bash
# Remove example events from the bar.
#
#   clear.sh         remove everything bar.sh, report.sh and feed.sh sent and the pages
#                    they wrote, and nothing else
#   clear.sh --all   remove every event and status on the bar, whoever sent it
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

case "${1:-}" in
  "")
    # A section with no items clears its source.
    for source in example-bar example-report example-feed; do
      "$LEDGE" event "{\"source\": \"$source\", \"items\": []}" >/dev/null
    done
    pages="${TMPDIR:-/tmp}"; pages="${pages%/}/ledge-examples"
    rm -rf "$pages/bar" "$pages/feed"
    echo "Cleared the examples" >&2
    ;;
  --all)
    # Every event has a source, so clearing each source clears the bar. Each event's
    # own source is the only "source" four spaces in. It's printed JSON-escaped, which
    # is just what goes back inside the JSON string.
    sources="$("$LEDGE" event --list | { grep -E '^    "source": ' || true; } \
               | sed -E 's/^    "source": "(.*)",?$/\1/' | sort -u)"
    count=0
    while IFS= read -r source; do
      [[ -n "$source" ]] || continue
      "$LEDGE" event "{\"source\": \"$source\", \"items\": []}" >/dev/null
      count=$(( count + 1 ))
    done <<<"$sources"
    echo "Cleared the bar ($count sources)" >&2
    ;;
  *)
    echo "usage: clear.sh [--all]" >&2
    exit 2
    ;;
esac
