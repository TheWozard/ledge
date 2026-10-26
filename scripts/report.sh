#!/usr/bin/env bash
# Example cards with reports, for trying each part of the report window.
#
#   - a CI report with one of each block: heading, text, fields, table, list, code,
#     notes in each colour, and a link (which opens in your browser)
#   - a long report, to scroll, whose text looks like HTML but must show as text
#
#   report.sh         send the cards
#   report.sh clear   remove them
#
# Sends to the API socket of $XDG_STATE_HOME's ledge with curl.

set -euo pipefail

# shellcheck source=scripts/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SOURCE="example-report"

if [[ "${1:-}" == "clear" ]]; then
  section "$SOURCE" '[]'
  echo "Cleared $SOURCE" >&2
  exit
fi

send '{"id": "report-ci", "source": "example-report", "priority": "high", "title": "CI: 2 failures on main",
       "description": "build and lint failed; 14 jobs passed",
       "report": [
         {"type": "note", "text": "Two jobs failed after the last merge.", "color": "red"},
         {"type": "fields", "items": [
           {"label": "Branch", "value": "main"}, {"label": "Commit", "value": "c81d1a5"},
           {"label": "Failed", "value": 2}, {"label": "Sent at", "value": "'"$(date +%T)"'"}]},
         {"type": "heading", "text": "Jobs"},
         {"type": "table", "columns": ["Job", "State", "Time"], "rows": [
           ["build", "failed", "1m 12s"], ["lint", "failed", "41s"], ["test", "passed", "3m 05s"],
           ["release", "skipped"]]},
         {"type": "heading", "text": "build"},
         {"type": "code", "text": "$ go build ./...\ninternal/store/store.go:42: undefined: Refuse"},
         {"type": "heading", "text": "Next"},
         {"type": "list", "ordered": true, "items": ["Fix the build", "Re-run lint", "Merge again"]},
         {"type": "note", "text": "lint fails on the same file; it should pass once the build does.", "color": "yellow"},
         {"type": "note", "text": "test passed.", "color": "green"},
         {"type": "note", "text": "A blue note.", "color": "blue"},
         {"type": "note", "text": "A note in a hex colour.", "color": "#9b59b6"},
         {"type": "link", "text": "Build logs", "url": "https://example.com/build/logs"}]}'

paragraphs=""
for i in $(seq 1 12); do
  paragraphs+='{"type": "heading", "text": "Section '"$i"'"},
    {"type": "text", "text": "Paragraph '"$i"'. <b>Not bold</b> <img src=x onerror=alert(1)>\nA second line, kept as a line break."},'
done
send '{"id": "report-long", "source": "example-report", "title": "A long report",
       "description": "Twelve sections, to scroll. Its HTML-looking text should show as text.",
       "report": ['"$paragraphs"' {"type": "text", "text": "The end."}]}'
echo "Sent 2 cards with reports" >&2
