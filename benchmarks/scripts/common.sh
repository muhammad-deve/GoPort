# Shared helpers for the target scripts. Source this file; don't run it.
#
# Environment (all optional):
#   ROUND          round number recorded in the result (default 1)
#   OUT_DIR        where result JSON goes (default results/<UTC date>)
#   WORK_DIR       logs, generated configs, the built tool (default $TMPDIR/tunnelbench)
#   ORIGIN_PORT    local origin port (default 18080)
#   TUNNELBENCH    path to a prebuilt tunnelbench binary (built on demand if unset)
#   BENCH_ARGS     extra flags for `tunnelbench run`, e.g. "-latency-requests 50"

set -euo pipefail

BENCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROUND="${ROUND:-1}"
OUT_DIR="${OUT_DIR:-$BENCH_DIR/results/$(date -u +%Y-%m-%d)}"
WORK_DIR="${WORK_DIR:-${TMPDIR:-/tmp}/tunnelbench}"
ORIGIN_PORT="${ORIGIN_PORT:-18080}"
BENCH_ARGS="${BENCH_ARGS:-}"

case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*) EXE=".exe" ;;
  *) EXE="" ;;
esac

mkdir -p "$OUT_DIR" "$WORK_DIR"

PIDS=()

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }

# stop_pid ends a background process and its children. Git Bash can't signal
# native Windows programs reliably, so there it goes through taskkill.
stop_pid() {
  local pid="$1"
  if [ -r "/proc/$pid/winpid" ]; then
    taskkill //F //T //PID "$(cat "/proc/$pid/winpid")" >/dev/null 2>&1 || true
  fi
  pkill -P "$pid" 2>/dev/null || true
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
}

cleanup() {
  local pid
  for pid in "${PIDS[@]:-}"; do
    [ -n "$pid" ] && stop_pid "$pid"
  done
  PIDS=()
}
trap cleanup EXIT INT TERM

# start_bg LOGFILE COMMAND... starts COMMAND in the background and remembers it
# for cleanup. The PID is stored in LAST_PID.
start_bg() {
  local logfile="$1"
  shift
  "$@" >"$logfile" 2>&1 &
  LAST_PID=$!
  PIDS+=("$LAST_PID")
}

is_running() { kill -0 "$1" 2>/dev/null; }

build_tunnelbench() {
  if [ -n "${TUNNELBENCH:-}" ]; then
    return
  fi
  TUNNELBENCH="$WORK_DIR/tunnelbench$EXE"
  log "building tunnelbench"
  (cd "$BENCH_DIR" && go build -o "$TUNNELBENCH" ./cmd/tunnelbench)
}

# wait_http URL SECONDS waits until URL answers 2xx.
wait_http() {
  local url="$1" deadline=$((SECONDS + $2))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if curl -fsS -m 3 -o /dev/null "$url" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# wait_log LOGFILE REGEX SECONDS [PID] prints the first match of REGEX in
# LOGFILE, waiting up to SECONDS. Gives up early if PID exits.
wait_log() {
  local logfile="$1" regex="$2" deadline=$((SECONDS + $3)) pid="${4:-}" match
  while [ "$SECONDS" -lt "$deadline" ]; do
    match="$(grep -Eo "$regex" "$logfile" 2>/dev/null | head -n1 || true)"
    if [ -n "$match" ]; then
      printf '%s\n' "$match"
      return 0
    fi
    if [ -n "$pid" ] && ! is_running "$pid"; then
      return 1
    fi
    sleep 1
  done
  return 1
}

start_origin() {
  build_tunnelbench
  start_bg "$WORK_DIR/origin.log" "$TUNNELBENCH" origin -addr "127.0.0.1:$ORIGIN_PORT"
  if ! wait_http "http://127.0.0.1:$ORIGIN_PORT/ping" 15; then
    log "origin did not start; see $WORK_DIR/origin.log"
    exit 1
  fi
}

result_file() { printf '%s/%s-r%s.json' "$OUT_DIR" "$1" "$ROUND"; }

# run_bench TARGET URL [tunnelbench run flags...]
run_bench() {
  local target="$1" url="$2"
  shift 2
  # shellcheck disable=SC2086 # BENCH_ARGS is a flag list on purpose
  "$TUNNELBENCH" run -url "$url" -target "$target" -round "$ROUND" \
    -out "$(result_file "$target")" $BENCH_ARGS "$@"
}

# setup_failed TARGET REASON [tunnelbench run flags...] records a run where the
# tunnel never came up, so the failure shows in the summary instead of the
# round silently going missing.
setup_failed() {
  local target="$1" reason="$2"
  shift 2
  log "$target: setup failed: $reason"
  # shellcheck disable=SC2086
  "$TUNNELBENCH" run -target "$target" -round "$ROUND" -setup-error "$reason" \
    -out "$(result_file "$target")" $BENCH_ARGS "$@"
}

# last_line FILE prints the last non-empty line of FILE without ANSI colours,
# for short failure reasons.
last_line() {
  sed -e 's/\x1b\[[0-9;?]*[A-Za-z]//g' "$1" 2>/dev/null | tr -d '\r' | grep -v '^[[:space:]]*$' | tail -n1 | cut -c1-200
}
