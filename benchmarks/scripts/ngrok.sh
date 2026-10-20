#!/usr/bin/env bash
# ngrok (hosted). Needs a configured authtoken (`ngrok config add-authtoken ...`).
# Free endpoints show a browser warning page unless the request carries
# `ngrok-skip-browser-warning`, which this script sends.
#
#   NGROK_BIN     ngrok binary (default: ngrok on PATH)
#   NGROK_ARGS    extra flags for `ngrok http`, e.g. "--region eu"
source "$(dirname "$0")/common.sh"

NGROK_BIN="${NGROK_BIN:-ngrok}"
NGROK_ARGS="${NGROK_ARGS:-}"

start_origin
version="$("$NGROK_BIN" version 2>/dev/null | awk '{print $NF}')"

# shellcheck disable=SC2086
start_bg "$WORK_DIR/ngrok.log" "$NGROK_BIN" http "$ORIGIN_PORT" --log stdout --log-format logfmt $NGROK_ARGS
ngrok_pid=$LAST_PID

# The agent API lists the public URL. Like GoPort's inspector it takes the
# first free port from 4040.
url=""
deadline=$((SECONDS + 45))
while [ -z "$url" ] && [ "$SECONDS" -lt "$deadline" ] && is_running "$ngrok_pid"; do
  for port in 4040 4041 4042 4043 4044 4045; do
    url="$(curl -fsS -m 2 "http://127.0.0.1:$port/api/tunnels" 2>/dev/null |
      grep -Eo '"public_url":"https://[^"]+"' | head -n1 | cut -d'"' -f4 || true)"
    [ -n "$url" ] && break
  done
  [ -z "$url" ] && sleep 1
done

# The agent log doesn't name the region it picked; `ngrok diagnose` does.
region="$("$NGROK_BIN" diagnose 2>/dev/null | grep -Eo "region: '[^']+', latency: [0-9.]+ms" | head -n1 || true)"

if [ -z "$url" ]; then
  setup_failed ngrok "ngrok did not start: $(last_line "$WORK_DIR/ngrok.log")" -client-version "$version" -region "$region"
  exit 0
fi
log "ngrok: $url ($region)"

if ! wait_http "$url/ping" 30; then
  log "ngrok: /ping not answering yet, running anyway"
fi

run_bench ngrok "$url" -client-version "$version" -region "${region:-ngrok default (global)}" \
  -header "ngrok-skip-browser-warning: 1" \
  -notes "ngrok agent with default settings."
