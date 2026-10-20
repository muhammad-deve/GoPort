#!/usr/bin/env bash
# zrok (openziti/zrok) public share. Needs an enabled environment first:
#   zrok invite            # or an account on your own zrok instance
#   zrok enable <token>
#
#   ZROK_BIN    zrok binary (default: zrok on PATH)
source "$(dirname "$0")/common.sh"

ZROK_BIN="${ZROK_BIN:-zrok}"

start_origin
version="$("$ZROK_BIN" version 2>/dev/null | grep -Eo 'v[0-9][^ ]*' | head -n1 || true)"

start_bg "$WORK_DIR/zrok.log" "$ZROK_BIN" share public "http://127.0.0.1:$ORIGIN_PORT" --headless
zrok_pid=$LAST_PID

url="$(wait_log "$WORK_DIR/zrok.log" 'https://[A-Za-z0-9.-]+\.zrok\.[A-Za-z.]+' 60 "$zrok_pid" || true)"
if [ -z "$url" ]; then
  setup_failed zrok "no share URL: $(last_line "$WORK_DIR/zrok.log")" -client-version "$version"
  exit 0
fi
log "zrok: $url"

if ! wait_http "$url/ping" 30; then
  log "zrok: /ping not answering yet, running anyway"
fi

run_bench zrok "$url" -client-version "$version" -region "${REGION:-zrok.io public frontend}" \
  -notes "zrok public share, default backend mode (proxy)."
