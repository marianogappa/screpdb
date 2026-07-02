#!/usr/bin/env bash
#
# Runs the SQLite ingestion benchmark against a fixed, committed replay corpus
# and prints a stable headline throughput figure (replays/sec + ms/replay).
#
# The corpus defaults to the markers testdata set (~154 .rep) so results are
# reproducible across runs and machines — the figure is still hardware-bound
# (the SQLite sink is single-threaded), so it is only comparable within the
# same runner.
#
# Usage: scripts/bench-ingest.sh [count]
#   count: number of benchmark samples (default 6, enough for benchstat).
#
# Env:
#   SCREPDB_BENCH_CORPUS  overrides the corpus dir (default: markers testdata).
#   BENCH_OUT             if set, the raw `go test -bench` output is written here
#                         (feed it to benchstat for PR-vs-main comparisons).
#
# Emits a machine-readable trailer line the CI parses:
#   ingest-bench-summary: replays_per_sec=<n> ms_per_replay=<n> corpus_replays=<n> samples=<n>
set -euo pipefail

COUNT="${1:-6}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORPUS="${SCREPDB_BENCH_CORPUS:-$repo_root/internal/patterns/markers/testdata/replays}"
cd "$repo_root"

raw="$(SCREPDB_BENCH_CORPUS="$CORPUS" go test ./internal/storage/ \
  -run '^$' -bench '^BenchmarkSQLiteIngestionCorpus$' \
  -benchmem -benchtime=1x -count="$COUNT")"

if [ -n "${BENCH_OUT:-}" ]; then
  printf '%s\n' "$raw" >"$BENCH_OUT"
fi

printf '%s\n' "$raw"

# Average the per-replay seconds across all samples, then derive replays/sec.
# The ingested-replay count comes from the s/ingest_<N>replays metric name, so
# it reflects the post-dedup corpus the benchmark actually ran (not raw file
# count).
summary="$(printf '%s\n' "$raw" | awk '
  {
    for (i = 1; i <= NF; i++) {
      if ($i == "s/replay") { sum += $(i-1); n++ }
      if ($i ~ /^s\/ingest_[0-9]+replays$/) { m = $i; gsub(/[^0-9]/, "", m); corpus = m }
    }
  }
  END {
    if (n == 0) { exit 1 }
    spr = sum / n
    printf "%.1f %.2f %d", 1.0 / spr, spr * 1000.0, corpus
  }')" || { echo "bench-ingest: could not parse s/replay from output" >&2; exit 1; }

replays_per_sec="$(echo "$summary" | cut -d' ' -f1)"
ms_per_replay="$(echo "$summary" | cut -d' ' -f2)"
corpus_count="$(echo "$summary" | cut -d' ' -f3)"

echo
echo "ingest-bench-summary: replays_per_sec=${replays_per_sec} ms_per_replay=${ms_per_replay} corpus_replays=${corpus_count} samples=${COUNT}"
