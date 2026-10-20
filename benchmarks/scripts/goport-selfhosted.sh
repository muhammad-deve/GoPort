#!/usr/bin/env bash
# GoPort against your own server (see https://goport.uz/self-hosting).
#
# Point the CLI at your server first:
#   export GOPORT_SERVER_ADDR=tunnel.example.com:7000
#   export GOPORT_DOMAIN=example.com
#   export GOPORT_API_URL=https://back.example.com
#   export GOPORT_TOKEN=...            # a token from *your* server's dashboard
#
# Loopback mode (server and CLI on one machine, no DNS or TLS): also set
#   export SELFHOSTED_CONNECT_URL=http://127.0.0.1:8090
# and requests go to that address with the tunnel's hostname in the Host header.
# That measures software overhead only and can't be compared with remote relays.
#
#   TARGET_NAME   result name (default: goport-selfhosted, or
#                 goport-selfhosted-loopback in loopback mode)
#   REGION        where your server runs, recorded in the result
set -euo pipefail

for var in GOPORT_SERVER_ADDR GOPORT_DOMAIN GOPORT_API_URL; do
  if [ -z "${!var:-}" ]; then
    echo "$var is not set; see the header of $0" >&2
    exit 2
  fi
done

if [ -n "${SELFHOSTED_CONNECT_URL:-}" ]; then
  export TARGET_NAME="${TARGET_NAME:-goport-selfhosted-loopback}"
  export REGION="${REGION:-loopback}"
else
  export TARGET_NAME="${TARGET_NAME:-goport-selfhosted}"
  export REGION="${REGION:-self-hosted ($GOPORT_SERVER_ADDR)}"
fi

if [ -z "${SELFHOSTED_CONNECT_URL:-}" ]; then
  exec bash "$(dirname "$0")/goport-hosted.sh"
fi

# Loopback mode: same as the hosted script, except requests are sent to
# SELFHOSTED_CONNECT_URL with the tunnel's hostname as the Host header.
source "$(dirname "$0")/common.sh"
GOPORT_BIN="${GOPORT_BIN:-goport}"

start_origin
version="$("$GOPORT_BIN" --version 2>/dev/null | awk '{print $NF}')"
start_bg "$WORK_DIR/goport-selfhosted.log" "$GOPORT_BIN" http "$ORIGIN_PORT"
goport_pid=$LAST_PID

url=""
deadline=$((SECONDS + 45))
while [ -z "$url" ] && [ "$SECONDS" -lt "$deadline" ] && is_running "$goport_pid"; do
  for port in 4040 4041 4042 4043 4044 4045; do
    body="$(curl -fsS -m 2 "http://127.0.0.1:$port/api/tunnel" 2>/dev/null || true)"
    candidate="$(printf '%s' "$body" | sed -n 's/.*"url":"\(https\{0,1\}:[^"]*\)".*/\1/p')"
    if [ -n "$candidate" ] && printf '%s' "$body" | grep -q '"status":"online"'; then
      url="$candidate"
      break
    fi
  done
  [ -z "$url" ] && sleep 1
done

if [ -z "$url" ]; then
  setup_failed "$TARGET_NAME" "goport did not come online: $(last_line "$WORK_DIR/goport-selfhosted.log")" \
    -client-version "$version" -region "$REGION"
  exit 0
fi

host="${url#*://}"
host="${host%%/*}"
log "$TARGET_NAME: $url via $SELFHOSTED_CONNECT_URL (Host: $host)"
run_bench "$TARGET_NAME" "$SELFHOSTED_CONNECT_URL" -host "$host" \
  -client-version "$version" -region "$REGION" -probe "$GOPORT_SERVER_ADDR" \
  -notes "Loopback: server and CLI on the same machine; plain HTTP to the server, no nginx or TLS. Software overhead only."
