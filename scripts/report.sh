#!/usr/bin/env bash
# Example cards with reports, for trying each part of the report window.
#
#   - a CI report with one of each block: heading, text, fields, table, list, code,
#     notes in each colour, quote, link, buttons, image, details, rule and run; with
#     strong, em, mono and linked text, links and buttons in table cells, and a
#     nested list. Links open in your browser; Retry and the run's Re-run lint run
#     only with commands on, and Done dismisses the card.
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
         {"type": "text", "text": ["The merge of ", {"type": "mono", "text": "store-refactor"}, " broke ",
           {"type": "strong", "text": ["the build, ", {"type": "em", "text": "and"}, " lint"]}, ". See ",
           {"type": "link", "text": "the run", "url": "https://example.com/run/1234"}, "."]},
         {"type": "fields", "items": [
           {"label": "Branch", "value": "main"}, {"label": "Commit", "value": {"type": "link", "text": "c81d1a5", "url": "https://example.com/commit/c81d1a5"}},
           {"label": "Failed", "value": 2}, {"label": "Sent at", "value": "'"$(date +%T)"'"}]},
         {"type": "heading", "text": "Jobs"},
         {"type": "table", "columns": ["Job", "State", "Time", "Logs", ""], "rows": [
           ["build", {"type": "strong", "text": "failed"}, "1m 12s", {"type": "link", "text": "logs", "url": "https://example.com/build/logs"},
            {"type": "buttons", "items": [{"title": "Retry", "color": "red", "command": "echo retrying build"}]}],
           ["lint", {"type": "strong", "text": "failed"}, "41s", {"type": "link", "text": "logs", "url": "https://example.com/lint/logs"},
            {"type": "buttons", "items": [{"title": "Retry", "color": "red", "command": "echo retrying lint"}]}],
           ["test", "passed", "3m 05s"],
           ["release", {"type": "em", "text": "skipped"}]]},
         {"type": "heading", "text": "build"},
         {"type": "code", "text": "$ go build ./...\ninternal/store/store.go:42: undefined: Refuse"},
         {"type": "details", "summary": "Full output", "blocks": [
           {"type": "code", "text": "$ go build -v ./...\ngithub.com/TheWozard/ledge/internal/store\ninternal/store/store.go:42: undefined: Refuse"}]},
         {"type": "heading", "text": "Next"},
         {"type": "list", "ordered": true, "items": ["Fix the build", "Re-run lint",
           {"text": "Merge again", "items": ["after review", {"text": "and once CI passes", "items": ["on main"]}]}]},
         {"type": "quote", "text": "lint fails on the same file as the build."},
         {"type": "rule"},
         {"type": "note", "text": "lint fails on the same file; it should pass once the build does.", "color": "yellow"},
         {"type": "note", "text": "test passed.", "color": "green"},
         {"type": "note", "text": "A blue note.", "color": "blue"},
         {"type": "note", "text": "A note in a hex colour.", "color": "#9b59b6"},
         {"type": "link", "text": "Build logs", "url": "https://example.com/build/logs"},
         {"type": "image", "alt": "A red dot", "src": "data:image/svg+xml;base64,'"$(printf '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><circle cx="12" cy="12" r="10" fill="#ff3b30"/></svg>' | base64)"'"},
         {"type": "heading", "text": "Lint again"},
         {"type": "run", "title": "Re-run lint",
          "command": "for i in $(seq 1 30); do echo \"checking file $i\"; sleep 0.1; done; echo \"store.go:42: undefined: Refuse\" >&2; exit 1"},
         {"type": "buttons", "items": [
           {"title": "Open the run", "color": "blue", "url": "https://example.com/run/1234"},
           {"title": "Done", "color": "green", "dismiss": true}, {"title": "Does nothing"}]}]}'

paragraphs=""
for i in $(seq 1 12); do
  paragraphs+='{"type": "heading", "text": "Section '"$i"'"},
    {"type": "text", "text": "Paragraph '"$i"'. <b>Not bold</b> <img src=x onerror=alert(1)>\nA second line, kept as a line break."},'
done
send '{"id": "report-long", "source": "example-report", "title": "A long report",
       "description": "Twelve sections, to scroll. Its HTML-looking text should show as text.",
       "report": ['"$paragraphs"' {"type": "text", "text": "The end."}]}'
echo "Sent 2 cards with reports" >&2
