#!/usr/bin/env bash
# frp (fatedier/frp): frps relay + frpc client, HTTP vhost proxy.
#
# FRP_MODE=loopback (default): starts frps and frpc on this machine. That
#   measures frp's software overhead only. It is not comparable with hosted
#   relays, which add a real network path.
# FRP_MODE=remote: uses your own frps. Set
#   FRP_SERVER_ADDR, FRP_SERVER_PORT (default 7000), FRP_TOKEN (if your frps
#   sets auth.token), FRP_DOMAIN (a hostname routed to frps' vhostHTTPPort or
#   vhostHTTPSPort), FRP_URL (the public base URL, e.g. https://bench.example.com)
#
#   FRPC_BIN / FRPS_BIN   binaries (default: frpc / frps on PATH)
source "$(dirname "$0")/common.sh"

FRP_MODE="${FRP_MODE:-loopback}"
FRPC_BIN="${FRPC_BIN:-frpc}"
FRPS_BIN="${FRPS_BIN:-frps}"

start_origin
version="$("$FRPC_BIN" --version 2>/dev/null | tr -d '\r')"

if [ "$FRP_MODE" = "loopback" ]; then
  target="frp-loopback"
  bind_port="${FRP_BIND_PORT:-17000}"
  vhost_port="${FRP_VHOST_PORT:-18081}"
  domain="bench.frp.test"
  cat >"$WORK_DIR/frps.toml" <<EOF
bindAddr = "127.0.0.1"
bindPort = $bind_port
vhostHTTPPort = $vhost_port
EOF
  start_bg "$WORK_DIR/frps.log" "$FRPS_BIN" -c "$WORK_DIR/frps.toml"
  if ! wait_log "$WORK_DIR/frps.log" 'started successfully' 15 "$LAST_PID" >/dev/null; then
    setup_failed "$target" "frps did not start: $(last_line "$WORK_DIR/frps.log")" -client-version "$version"
    exit 0
  fi
  server_addr="127.0.0.1"
  server_port="$bind_port"
  token=""
  connect_url="http://127.0.0.1:$vhost_port"
  host_args=(-host "$domain")
  region="loopback"
  notes="Loopback: frps and frpc on the same machine. Software overhead only."
else
  target="frp-remote"
  server_addr="${FRP_SERVER_ADDR:?set FRP_SERVER_ADDR}"
  server_port="${FRP_SERVER_PORT:-7000}"
  token="${FRP_TOKEN:-}"
  domain="${FRP_DOMAIN:?set FRP_DOMAIN}"
  connect_url="${FRP_URL:?set FRP_URL}"
  host_args=()
  region="${REGION:-self-hosted frps ($server_addr)}"
  notes="Remote frps."
fi

{
  echo "serverAddr = \"$server_addr\""
  echo "serverPort = $server_port"
  echo "loginFailExit = true"
  if [ -n "$token" ]; then
    echo 'auth.method = "token"'
    echo "auth.token = \"$token\""
  fi
  echo ""
  echo "[[proxies]]"
  echo 'name = "tunnelbench"'
  echo 'type = "http"'
  echo 'localIP = "127.0.0.1"'
  echo "localPort = $ORIGIN_PORT"
  echo "customDomains = [\"$domain\"]"
} >"$WORK_DIR/frpc.toml"
chmod 600 "$WORK_DIR/frpc.toml"

start_bg "$WORK_DIR/frpc.log" "$FRPC_BIN" -c "$WORK_DIR/frpc.toml"
frpc_pid=$LAST_PID

if ! wait_log "$WORK_DIR/frpc.log" 'start proxy success' 30 "$frpc_pid" >/dev/null; then
  setup_failed "$target" "frpc did not start: $(last_line "$WORK_DIR/frpc.log")" -client-version "$version"
  exit 0
fi

run_bench "$target" "$connect_url" ${host_args[@]+"${host_args[@]}"} -client-version "frp $version" \
  -region "$region" -probe "$server_addr:$server_port" -notes "$notes"
