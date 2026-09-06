#!/usr/bin/env bash
#
# Regenerate every README screenshot, in both languages, from a freshly staged
# corpus. See README.md in this folder for the prerequisites.
#
#   ./run.sh [--harvest DIR] [--port 8123] [--only game-hotkeys,alliances]
#
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"

harvest="${HOME}/Code/go/src/github.com/marianogappa/screpharvest/harvest"
port=8123
limit=500
only=""
work="${TMPDIR:-/tmp}/screpdb-readme-shots"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --harvest) harvest="$2"; shift 2 ;;
    --port)    port="$2";    shift 2 ;;
    --limit)   limit="$2";   shift 2 ;;
    --only)    only="$2";    shift 2 ;;
    --work)    work="$2";    shift 2 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

corpus="$work/corpus"

echo "==> staging corpus"
node "$here/stage.mjs" \
  --harvest "$harvest" \
  --out "$corpus" \
  --limit "$limit" \
  --per-handle 20 \
  --extras "$repo/internal/sampledata/replays,$here/replays"

echo "==> building screpdb"
make -C "$repo" build >/dev/null

echo "==> starting the dashboard on port $port"
# pkg/browser shells out to `open` on macOS; shadow it so the run stays headless.
fake="$work/bin"
mkdir -p "$fake"
printf '#!/bin/sh\nexit 0\n' > "$fake/open"
chmod +x "$fake/open"

PATH="$fake:$PATH" "$repo/screpdb" dashboard --replay-dir "$corpus" --port "$port" \
  > "$work/dashboard.log" 2>&1 &
dashboard_pid=$!
trap 'kill "$dashboard_pid" 2>/dev/null || true' EXIT

echo -n "    waiting for the corpus to load"
for _ in $(seq 1 120); do
  loaded="$(curl -sf "http://localhost:$port/api/games?limit=1" \
    | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{try{const c=JSON.parse(s).corpus;process.stdout.write(c&&c.complete?"1":"")}catch{}})' || true)"
  [[ "$loaded" == "1" ]] && break
  echo -n "."
  sleep 2
done
echo

echo "==> capturing"
for locale in en ko; do
  node "$here/capture.mjs" \
    --base "http://localhost:$port" \
    --out "$repo/docs/images" \
    --locale "$locale" \
    ${only:+--only "$only"}
done

echo "==> done: $repo/docs/images"
