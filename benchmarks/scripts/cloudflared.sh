#!/usr/bin/env bash
# Cloudflare Tunnel, quick tunnel mode (trycloudflare.com, no account).
# Quick tunnels are meant for testing and have no uptime guarantee; a named
# tunnel on your own zone may perform differently.
#
#   CLOUDFLARED_BIN   cloudflared binary (default: cloudflared on PATH)
source "$(dirname "$0")/common.sh"

CLOUDFLARED_BIN="${CLOUDFLARED_BIN:-cloudflared}"

start_origin
version="$("$CLOUDFLARED_BIN" --version 2>/dev/null | awk '{print $3}')"

start_bg "$WORK_DIR/cloudflared.log" "$CLOUDFLARED_BIN" tunnel --no-autoupdate --url "http://127.0.0.1:$ORIGIN_PORT"
cf_pid=$LAST_PID

url="$(wait_log "$WORK_DIR/cloudflared.log" 'https://[a-z0-9-]+\.trycloudflare\.com' 60 "$cf_pid" || true)"
if [ -z "$url" ]; then
  setup_failed cloudflared-quick "no trycloudflare URL: $(last_line "$WORK_DIR/cloudflared.log")" -client-version "$version"
  exit 0
fi

# Wait for an edge connection before the first DNS lookup, so a negative DNS
# answer isn't cached for the whole run.
wait_log "$WORK_DIR/cloudflared.log" 'Registered tunnel connection' 60 "$cf_pid" >/dev/null || true
sleep 5
locations="$(grep -Eo 'location=[A-Za-z0-9-]+' "$WORK_DIR/cloudflared.log" | sort -u | cut -d= -f2 | tr '\n' ' ' | sed 's/ $//')"
log "cloudflared: $url (edge: $locations)"

if ! wait_http "$url/ping" 30; then
  log "cloudflared: /ping not answering yet, running anyway"
fi

run_bench cloudflared-quick "$url" -client-version "$version" \
  -region "connector edges: ${locations:-unknown}" \
  -notes "Quick tunnel (trycloudflare.com), default protocol."
