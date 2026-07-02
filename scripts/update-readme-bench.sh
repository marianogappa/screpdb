#!/usr/bin/env bash
#
# Rewrites the ingestion-throughput badge and its detail note in README.md.
# The badge lives in the badge row; the note lives between the
# <!-- ingest-bench-start --> / <!-- ingest-bench-end --> markers.
#
# Usage: scripts/update-readme-bench.sh <replays_per_sec> <ms_per_replay> <corpus_replays>
set -euo pipefail

rps="$1"
mspr="$2"
corpus="$3"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readme="$repo_root/README.md"

badge="[![Ingestion throughput](https://img.shields.io/badge/ingestion-${rps}%20replays%2Fsec-brightgreen)](.github/workflows/bench-ingest.yml)"
note="<sub>${mspr} ms/replay · corpus: ${corpus} replays · GitHub-hosted 2-core runner · updated automatically on merge to main</sub>"

awk -v badge="$badge" -v note="$note" '
  /^\[!\[Ingestion throughput\]/ { print badge; next }
  /<!-- ingest-bench-start -->/  { print; print note; skip = 1; next }
  /<!-- ingest-bench-end -->/    { skip = 0 }
  skip                          { next }
  { print }
' "$readme" >"$readme.tmp"
mv "$readme.tmp" "$readme"
