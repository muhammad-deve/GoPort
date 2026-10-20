#!/usr/bin/env bash
# Localhost baseline: the origin reached directly over loopback, no tunnel.
# Everything a tunnel adds is measured against this.
source "$(dirname "$0")/common.sh"

start_origin
run_bench baseline-localhost "http://127.0.0.1:$ORIGIN_PORT" \
  -client-version "none" -region "loopback" \
  -notes "Origin reached directly; no tunnel."
