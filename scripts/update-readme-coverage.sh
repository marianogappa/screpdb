#!/usr/bin/env bash
#
# Rewrites the coverage badge in README.md to the given percentage. The badge
# lives in the badge row and links to scripts/coverage.sh. Color tracks the
# value: >=80 brightgreen, >=70 green, >=50 yellow, else red.
#
# Usage: scripts/update-readme-coverage.sh <percent>   # e.g. 81.0
set -euo pipefail

pct="$1"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readme="$repo_root/README.md"

# Round to a whole number for the badge label.
pct_int="$(printf '%.0f' "$pct")"
if   [ "$pct_int" -ge 80 ]; then color="brightgreen"
elif [ "$pct_int" -ge 70 ]; then color="green"
elif [ "$pct_int" -ge 50 ]; then color="yellow"
else                             color="red"
fi

badge="[![Coverage](https://img.shields.io/badge/coverage-${pct_int}%25-${color})](scripts/coverage.sh)"

awk -v badge="$badge" '
  /^\[!\[Coverage\]/ { print badge; next }
  { print }
' "$readme" >"$readme.tmp"
mv "$readme.tmp" "$readme"
