#!/usr/bin/env bash
# Regenerate the Scoop manifest (bucket/screpdb.json) for a release.
#
# Usage: scripts/update-scoop-manifest.sh <version> <path-to-SHA256SUMS>
#   <version>        release version, with or without a leading "v" (e.g. 1.3.0)
#   <SHA256SUMS>     the SHA256SUMS file produced by `make cross-binaries`
#
# Run by the release workflow after building binaries; can also be run locally.
set -euo pipefail

VERSION="${1:?usage: update-scoop-manifest.sh <version> <SHA256SUMS>}"
SUMS="${2:?usage: update-scoop-manifest.sh <version> <SHA256SUMS>}"
VERSION="${VERSION#v}"

hash_for() {
  local name="$1" h
  h="$(awk -v n="$name" '$2 == n {print $1}' "$SUMS")"
  if [ -z "$h" ]; then
    echo "error: no checksum for '$name' in $SUMS" >&2
    exit 1
  fi
  printf '%s' "$h"
}

CLI_HASH="$(hash_for screpdb-windows-amd64.exe)"
DASH_HASH="$(hash_for screpdb-dashboard-windows-amd64.exe)"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/bucket/screpdb.json"

cat > "$OUT" <<JSON
{
    "version": "${VERSION}",
    "description": "Advanced StarCraft: Brood War replay reporting tool (CLI + local dashboard).",
    "homepage": "https://github.com/marianogappa/screpdb",
    "license": "MIT",
    "notes": [
        "Run 'screpdb' (or 'screpdb dashboard') to open the local dashboard in your browser.",
        "The binaries are not code-signed; SmartScreen/Defender may warn on first run. See the README."
    ],
    "architecture": {
        "64bit": {
            "url": [
                "https://github.com/marianogappa/screpdb/releases/download/v${VERSION}/screpdb-windows-amd64.exe",
                "https://github.com/marianogappa/screpdb/releases/download/v${VERSION}/screpdb-dashboard-windows-amd64.exe"
            ],
            "hash": [
                "${CLI_HASH}",
                "${DASH_HASH}"
            ],
            "bin": [
                [
                    "screpdb-windows-amd64.exe",
                    "screpdb"
                ],
                [
                    "screpdb-dashboard-windows-amd64.exe",
                    "screpdb-dashboard"
                ]
            ],
            "shortcuts": [
                [
                    "screpdb-dashboard-windows-amd64.exe",
                    "screpdb dashboard"
                ]
            ]
        }
    },
    "checkver": "github",
    "autoupdate": {
        "architecture": {
            "64bit": {
                "url": [
                    "https://github.com/marianogappa/screpdb/releases/download/v\$version/screpdb-windows-amd64.exe",
                    "https://github.com/marianogappa/screpdb/releases/download/v\$version/screpdb-dashboard-windows-amd64.exe"
                ]
            }
        },
        "hash": {
            "url": "https://github.com/marianogappa/screpdb/releases/download/v\$version/SHA256SUMS"
        }
    }
}
JSON

echo "Wrote $OUT (version ${VERSION})"
