#!/usr/bin/env bash
# Computes test coverage over hand-written, meaningful code only.
#
# Excluded from the denominator (testing them is noise, not signal):
#   - Generated code: any file carrying a "// Code generated ... DO NOT EDIT."
#     header (OpenAPI apigen, sqlc sqlcgen, the openapi handler bridge).
#   - scripts/ : one-off throwaway analysis `package main` programs.
#   - **/tools/ : code-generation tooling, not shipped logic.
#
# Usage: scripts/coverage.sh [--html]
#   (no args)  print the excluded-generated total + per-package breakdown
#   --html     also open an HTML report of the filtered profile
set -euo pipefail

cd "$(dirname "$0")/.."

MODULE="github.com/marianogappa/screpdb"
RAW="$(mktemp -t cov-raw.XXXXXX)"
FILTERED="$(mktemp -t cov-filtered.XXXXXX)"
trap 'rm -f "$RAW" "$FILTERED"' EXIT

echo "Running tests with coverage..." >&2
go test ./... -coverprofile="$RAW" -covermode=atomic >/dev/null

# Module-qualified paths of every generated file (detected, not hard-coded).
GEN_RE="$(grep -rl '^// Code generated' --include='*.go' . 2>/dev/null \
  | sed "s#^\./#${MODULE//\//\\/}\/#" | paste -sd'|' -)"

awk -v gen="$GEN_RE" '
  NR == 1 { print; next }                       # keep the "mode:" header
  {
    path = $1; sub(/:.*/, "", path)
    if (gen != "" && path ~ ("^(" gen ")$")) next  # generated file
    if (path ~ "/scripts/") next                    # throwaway analysis mains
    if (path ~ "/tools/") next                      # codegen tooling
    print
  }
' "$RAW" > "$FILTERED"

echo
go tool cover -func="$FILTERED" | tail -1
echo
echo "(generated code, scripts/, and **/tools/ excluded from the denominator)"

if [[ "${1:-}" == "--html" ]]; then
  go tool cover -html="$FILTERED"
fi
