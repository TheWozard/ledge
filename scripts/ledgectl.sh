#!/usr/bin/env bash
# Run ledge in the background while working on it.
#
#   ledgectl.sh start [ARGS...]     build from this repo and start it; ARGS go to
#                                   `ledge run`, e.g. --config bar.json
#   ledgectl.sh stop
#   ledgectl.sh restart [ARGS...]   stop, rebuild and start, picking up changes
#
# Keeps the binary, a pid file and the log in ledge's state dir. Set $LEDGE_PORT for
# another port, and $XDG_STATE_HOME for another state dir.

set -euo pipefail

REPO_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/ledge"
BIN="$STATE_DIR/ledge"
PID_FILE="$STATE_DIR/ledge.pid"
LOG_FILE="$STATE_DIR/ledge.log"

usage() {
  sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

# The pid in the pid file, if that process is still the ledge built here. Checking the
# command line means a stale file whose pid has been reused is never acted on.
running_pid() {
  [[ -f "$PID_FILE" ]] || return 1
  local pid
  pid="$(cat "$PID_FILE")"
  [[ -n "$pid" ]] && ps -p "$pid" -o command= 2>/dev/null | grep -qF -- "$BIN run" || return 1
  echo "$pid"
}

start() {
  local pid
  if pid="$(running_pid)"; then
    echo "ledge: already running (pid $pid)"
    return
  fi
  mkdir -p "$STATE_DIR"
  (cd "$REPO_DIR" && go build -o "$BIN" ./ledge)
  # Another ledge (run by hand, or desktop-view on the same port) would answer the
  # readiness check below and make a failed start look like a success.
  if "$BIN" event --list >/dev/null 2>&1; then
    echo "ledge: something not started by ledgectl already has the port; stop it first" >&2
    exit 1
  fi
  nohup "$BIN" run "$@" >"$LOG_FILE" 2>&1 </dev/null &
  pid=$!
  echo "$pid" >"$PID_FILE"
  # Ready once the API answers; give up if it exits first (e.g. a bad config).
  for _ in $(seq 50); do
    if "$BIN" event --list >/dev/null 2>&1; then
      echo "ledge: started (pid $pid, log $LOG_FILE)"
      return
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
      rm -f "$PID_FILE"
      echo "ledge: failed to start:" >&2
      cat "$LOG_FILE" >&2
      exit 1
    fi
    sleep 0.2
  done
  echo "ledge: started (pid $pid) but the API isn't answering yet; see $LOG_FILE" >&2
}

stop() {
  local pid
  if ! pid="$(running_pid)"; then
    rm -f "$PID_FILE"
    echo "ledge: not running"
    return
  fi
  kill "$pid"
  for _ in $(seq 25); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.2
  done
  if kill -0 "$pid" 2>/dev/null; then
    echo "ledge: pid $pid didn't exit; killing it" >&2
    kill -9 "$pid"
  fi
  rm -f "$PID_FILE"
  echo "ledge: stopped"
}

command="${1:-}"
[[ $# -gt 0 ]] && shift
case "$command" in
  start) start "$@" ;;
  stop) stop ;;
  restart) stop; start "$@" ;;
  -h|--help) usage ;;
  *) usage >&2; exit 2 ;;
esac
