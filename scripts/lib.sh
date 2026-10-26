# Shared by the example scripts: calls to ledge's API on its socket, and the local
# pages their cards open. Source it; it isn't run.
#
# The socket is the one of $XDG_STATE_HOME's ledge, as `ledge run` makes it.

SOCKET="${XDG_STATE_HOME:-$HOME/.local/state}/ledge/ledge.sock"
if [[ ! -S "$SOCKET" ]]; then
  echo "error: no ledge at $SOCKET; start one with \`ledge start\`" >&2
  exit 1
fi

# api METHOD PATH [BODY]: calls the API and prints its reply. A refused request
# prints ledge's error, {"error": "..."}, and fails.
api() {
  local args=(-s --fail-with-body --unix-socket "$SOCKET" -X "$1" "http://ledge$2")
  [[ $# -lt 3 ]] || args+=(--data-raw "$3")
  local reply
  if ! reply="$(curl "${args[@]}")"; then
    reply="$(tr -d '\n' <<<"$reply" | sed -E 's/^\{ *"error": *"(.*)" *\}$/\1/')"
    echo "error: $1 $2: ${reply:-no reply from the ledge at $SOCKET}" >&2
    return 1
  fi
  printf '%s\n' "$reply"
}

# send EVENT: adds the event, or replaces the one with its id.
send() { api POST /events "$1" >/dev/null; }

# section SOURCE ITEMS: makes ITEMS, a JSON list of events, all of SOURCE's events.
# An empty list clears it.
section() { api PUT "/sources/$1" "{\"items\": $2}" >/dev/null; }

# pages DIR: the directory the example pages of DIR go in.
pages() { local tmp="${TMPDIR:-/tmp}"; echo "${tmp%/}/ledge-examples/$1"; }

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
