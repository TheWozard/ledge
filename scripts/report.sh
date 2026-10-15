#!/usr/bin/env bash
# An example card with a report: an HTML file this script writes, with a stylesheet
# and an image beside it. Clicking the card shows the file in the report window (from
# milestone 3).
#
#   report.sh         write the report and send the card
#   report.sh clear   remove the card
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
SOURCE="example-report"
send() { "$LEDGE" event "$1" >/dev/null; }

if [[ "${1:-}" == "clear" ]]; then
  send "{\"source\": \"$SOURCE\", \"items\": []}"
  echo "Cleared $SOURCE" >&2
  exit
fi

dir="${TMPDIR:-/tmp}"; dir="${dir%/}/ledge-examples/report"
mkdir -p "$dir"
cat > "$dir/style.css" <<'CSS'
body { font: 14px/1.5 -apple-system, sans-serif; margin: 24px; color: #1d1d1f; }
h1 { font-size: 20px; margin: 0 0 8px; }
.failed { color: #c4352b; font-weight: 600; }
table { border-collapse: collapse; margin-top: 12px; }
td, th { border: 1px solid #ddd; padding: 4px 10px; text-align: left; }
pre { background: #f4f4f5; padding: 8px 12px; border-radius: 6px; }
@media (prefers-color-scheme: dark) {
  body { background: #1d1d1f; color: #ececef; }
  td, th { border-color: #444; }
  pre { background: #2a2a2e; }
}
CSS
cat > "$dir/status.svg" <<'SVG'
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="16"><rect width="120" height="16" rx="3" fill="#c4352b"/><text x="60" y="12" fill="#fff" font-family="sans-serif" font-size="11" text-anchor="middle">2 failed</text></svg>
SVG
cat > "$dir/index.html" <<'HTML'
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>CI: 2 failures on main</title>
<link rel="stylesheet" href="style.css">
</head>
<body>
<h1>CI: 2 failures on main</h1>
<p><img src="status.svg" alt="2 failed"> Two jobs failed after the last merge.</p>
<table>
  <tr><th>Job</th><th>State</th></tr>
  <tr><td>build</td><td class="failed">failed</td></tr>
  <tr><td>lint</td><td class="failed">failed</td></tr>
  <tr><td>test</td><td>passed</td></tr>
</table>
<pre>internal/store/store.go:42: undefined: Refuse</pre>
<p><a href="https://example.com/build/logs">Build logs</a> open in your browser.</p>
</body>
</html>
HTML

send '{"id": "report-ci", "source": "example-report", "priority": "high", "title": "CI: 2 failures on main",
       "description": "build and lint failed; 14 jobs passed", "tags": ["ci"],
       "report": "'"$dir/index.html"'"}'
echo "Wrote $dir/index.html and sent a card with it as the report" >&2
