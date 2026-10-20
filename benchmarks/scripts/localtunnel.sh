#!/usr/bin/env bash
# LocalTunnel via its public server (loca.lt). Needs Node.js for npx.
# Browsers see a reminder page unless the request carries
# `bypass-tunnel-reminder`, which this script sends.
#
#   LT_VERSION    localtunnel npm version (default 2.0.2)
#   LT_HOST       server, e.g. a self-hosted localtunnel-server
#                 (default https://localtunnel.me, which hands out *.loca.lt URLs)
source "$(dirname "$0")/common.sh"

LT_VERSION="${LT_VERSION:-2.0.2}"
LT_HOST="${LT_HOST:-https://localtunnel.me}"

start_origin
start_bg "$WORK_DIR/localtunnel.log" npx --yes "localtunnel@$LT_VERSION" \
  --port "$ORIGIN_PORT" --local-host 127.0.0.1 --host "$LT_HOST"
lt_pid=$LAST_PID

url="$(wait_log "$WORK_DIR/localtunnel.log" 'your url is: https://[A-Za-z0-9.-]+' 90 "$lt_pid" || true)"
url="${url#your url is: }"
if [ -z "$url" ]; then
  setup_failed localtunnel "no URL from localtunnel: $(last_line "$WORK_DIR/localtunnel.log")" -client-version "$LT_VERSION"
  exit 0
fi
log "localtunnel: $url"

if ! wait_http "$url/ping" 30; then
  log "localtunnel: /ping not answering yet, running anyway"
fi

run_bench localtunnel "$url" -client-version "localtunnel $LT_VERSION (npm)" \
  -region "public server ($LT_HOST)" \
  -header "bypass-tunnel-reminder: 1" \
  -notes "Public LocalTunnel server."
