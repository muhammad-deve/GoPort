#!/usr/bin/env bash
# Run several targets for several rounds, interleaved: every round runs each
# target once, and the order rotates between rounds. That way a slow hour on
# the network hits every tool, not just the one that happened to run then.
#
#   ./scripts/run-all.sh baseline goport-hosted cloudflared
#   ROUNDS=5 ./scripts/run-all.sh
#
# Targets are script names in this folder without ".sh". The default list is
# the ones that need no extra setup beyond their own accounts/binaries.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BENCH_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROUNDS="${ROUNDS:-3}"
PAUSE="${PAUSE:-5}"
export OUT_DIR="${OUT_DIR:-$BENCH_DIR/results/$(date -u +%Y-%m-%d)}"
export WORK_DIR="${WORK_DIR:-${TMPDIR:-/tmp}/tunnelbench}"

if [ "$#" -gt 0 ]; then
  targets=("$@")
else
  targets=(baseline goport-hosted ngrok cloudflared localtunnel frp)
fi

for t in "${targets[@]}"; do
  if [ ! -f "$SCRIPT_DIR/$t.sh" ]; then
    echo "unknown target: $t" >&2
    exit 2
  fi
done

mkdir -p "$OUT_DIR" "$WORK_DIR"

# Build once and share the binary with every target script.
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*) exe=".exe" ;;
  *) exe="" ;;
esac
if [ -z "${TUNNELBENCH:-}" ]; then
  export TUNNELBENCH="$WORK_DIR/tunnelbench$exe"
  (cd "$BENCH_DIR" && go build -o "$TUNNELBENCH" ./cmd/tunnelbench)
fi

n=${#targets[@]}
for ((round = 1; round <= ROUNDS; round++)); do
  for ((i = 0; i < n; i++)); do
    t="${targets[$(((i + round - 1) % n))]}"
    echo "=== round $round/$ROUNDS: $t" >&2
    if ! ROUND="$round" bash "$SCRIPT_DIR/$t.sh"; then
      echo "=== $t round $round exited with an error; see $WORK_DIR" >&2
    fi
    sleep "$PAUSE"
  done
done

"$TUNNELBENCH" report "$OUT_DIR"/*.json >"$OUT_DIR/SUMMARY.md"
echo "Summary: $OUT_DIR/SUMMARY.md" >&2
