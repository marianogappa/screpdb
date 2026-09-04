#!/usr/bin/env bash
#
# Rewrites the replay-load throughput badge and its detail note in README.md.
# The badge lives in the badge row; the note lives between the
# <!-- load-bench-start --> / <!-- load-bench-end --> markers.
#
# The figure tracks the replay-library load path (what a user waits for on
# launch), not the SQLite `screpdb ingest` path — see scripts/bench-load.sh.
#
# Usage: scripts/update-readme-bench.sh <replays_per_sec> <ms_per_replay> <corpus_replays> [runner_label]
set -euo pipefail

rps="$1"
mspr="$2"
corpus="$3"
runner="${4:-GitHub-hosted 2-core runner}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readme="$repo_root/README.md"

badge="[![Replay load throughput](https://img.shields.io/badge/replay%20load-${rps}%20replays%2Fsec-brightgreen)](.github/workflows/bench-load.yml)"
note="<sub>${mspr} ms/replay · corpus: ${corpus} replays · ${runner} · updated automatically on merge to main</sub>"

awk -v badge="$badge" -v note="$note" '
  /^\[!\[Replay load throughput\]/ { print badge; next }
  /<!-- load-bench-start -->/       { print; print note; skip = 1; next }
  /<!-- load-bench-end -->/         { skip = 0 }
  skip                              { next }
  { print }
' "$readme" >"$readme.tmp"
mv "$readme.tmp" "$readme"
