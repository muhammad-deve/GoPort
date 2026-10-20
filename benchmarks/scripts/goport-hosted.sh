#!/usr/bin/env bash
# GoPort hosted service (goport.uz).
#
# Needs an account token: either GOPORT_TOKEN in the environment or one saved
# with `goport auth <token>`. The token is never passed on the command line or
# written to the result.
#
#   GOPORT_BIN    goport binary to use (default: goport on PATH)
#   TARGET_NAME   result name (default: goport-hosted)
#   REGION        recorded region (default: the goport.uz server's location)
source "$(dirname "$0")/common.sh"

GOPORT_BIN="${GOPORT_BIN:-goport}"
TARGET_NAME="${TARGET_NAME:-goport-hosted}"
REGION="${REGION:-Contabo, Lauterbourg, France (goport.uz)}"
SERVER_ADDR="${GOPORT_SERVER_ADDR:-goport.uz:7000}"

start_origin
version="$("$GOPORT_BIN" --version 2>/dev/null | awk '{print $NF}')"

# The CLI draws a terminal UI; its log is only read for errors.
start_bg "$WORK_DIR/goport.log" "$GOPORT_BIN" http "$ORIGIN_PORT"
goport_pid=$LAST_PID

# The local inspector reports the public URL. It takes the first free port
# from 4040, so probe a few.
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
  setup_failed "$TARGET_NAME" "goport did not come online: $(last_line "$WORK_DIR/goport.log")" \
    -client-version "$version" -region "$REGION"
  exit 0
fi
log "$TARGET_NAME: $url"

if ! wait_http "$url/ping" 30; then
  setup_failed "$TARGET_NAME" "public URL never answered /ping" -client-version "$version" -region "$REGION"
  exit 0
fi

run_bench "$TARGET_NAME" "$url" -client-version "$version" -region "$REGION" \
  -probe "$SERVER_ADDR" \
  -notes "CLI connects to $SERVER_ADDR over plain TCP + yamux; TLS ends at the server's nginx."
