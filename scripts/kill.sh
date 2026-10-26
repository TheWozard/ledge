#!/usr/bin/env bash
# Stop every ledge running, by its process: `ledge stop` stops only the one on its
# port, and through POST /shutdown, which a ledge on another port, or one too old to
# have it, won't get. A ledge is any `<binary> run` whose binary was built from this
# module.
#
#   kill.sh   SIGTERM each, then SIGKILL any still running after 5s

set -euo pipefail

STATE="${XDG_STATE_HOME:-$HOME/.local/state}/ledge"

pids=()
while read -r pid binary arg _; do
  [[ "$arg" == run && -x "$binary" ]] || continue
  if go version -m "$binary" 2>/dev/null | grep -qE $'^\t(mod|dep)\tgithub.com/TheWozard/ledge\t'; then
    pids+=("$pid")
  fi
done < <(ps -axo pid=,command=)
if [[ ${#pids[@]} -eq 0 ]]; then
  echo "No ledge running" >&2
  exit
fi

ps -o pid=,command= -p "$(IFS=,; echo "${pids[*]}")" >&2
kill -TERM "${pids[@]}" 2>/dev/null || true
for _ in $(seq 50); do
  kill -0 "${pids[@]}" 2>/dev/null || break
  sleep 0.1
done
kill -KILL "${pids[@]}" 2>/dev/null && echo "Killed what didn't exit in 5s" >&2 || true
rm -f "$STATE/ledge.pid"
echo "Stopped ${#pids[@]} ledge(s)" >&2
