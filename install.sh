#!/bin/sh
# screpdb universal installer for macOS and Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh
#
# Downloads the right release binary for your OS/arch, verifies it against the
# release's minisign-signed SHA256SUMS, and drops it on your PATH. Because curl
# attaches no quarantine/Mark-of-the-Web, the binary just runs — no notarization
# or Gatekeeper "unidentified developer" dance.
#
# Environment overrides:
#   SCREPDB_VERSION      install a specific version (e.g. 1.4.0) instead of latest
#   SCREPDB_INSTALL_DIR  install directory (default: ~/.local/bin)
set -eu

REPO="marianogappa/screpdb"
# minisign public key that signs every release's SHA256SUMS (see README).
MINISIGN_PUBKEY="RWS9gPPOydPD/tR8JBOelXKhif526NoAKY18dau7QHR4dqg84QMhJ5L/"

err() { printf 'error: %s\n' "$1" >&2; exit 1; }
info() { printf '%s\n' "$1" >&2; }

have() { command -v "$1" >/dev/null 2>&1; }

# --- detect platform ---------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) err "unsupported OS '$os' — see the README for Windows (Scoop) or build-from-source" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) err "unsupported architecture '$arch'" ;;
esac

asset="screpdb-${os}-${arch}"

# --- resolve download URLs ---------------------------------------------------
version="${SCREPDB_VERSION:-}"
if [ -n "$version" ]; then
  base="https://github.com/${REPO}/releases/download/v${version#v}"
else
  base="https://github.com/${REPO}/releases/latest/download"
fi

# --- pick a downloader -------------------------------------------------------
if have curl; then
  dl() { curl -fsSL "$1" -o "$2"; }
elif have wget; then
  dl() { wget -qO "$2" "$1"; }
else
  err "need curl or wget to download"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

info "Downloading ${asset} (${version:-latest})..."
dl "${base}/${asset}" "${tmp}/${asset}" || err "failed to download ${base}/${asset}"
dl "${base}/SHA256SUMS" "${tmp}/SHA256SUMS" || err "failed to download SHA256SUMS"

# --- verify checksum (mandatory) ---------------------------------------------
expected="$(awk -v n="$asset" '$2 == n || $2 == "*"n {print $1}' "${tmp}/SHA256SUMS" | head -n1)"
[ -n "$expected" ] || err "no checksum for ${asset} in SHA256SUMS"

if have sha256sum; then
  actual="$(sha256sum "${tmp}/${asset}" | awk '{print $1}')"
elif have shasum; then
  actual="$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}')"
else
  err "need sha256sum or shasum to verify the download"
fi

[ "$actual" = "$expected" ] || err "checksum mismatch for ${asset} (expected ${expected}, got ${actual})"
info "Checksum OK."

# --- verify minisign signature (best-effort) ---------------------------------
if have minisign; then
  if dl "${base}/SHA256SUMS.minisig" "${tmp}/SHA256SUMS.minisig"; then
    if minisign -Vm "${tmp}/SHA256SUMS" -P "$MINISIGN_PUBKEY" >/dev/null 2>&1; then
      info "Signature OK."
    else
      err "minisign signature verification failed"
    fi
  fi
else
  info "(minisign not installed — skipping signature check; checksum already verified)"
fi

# --- install -----------------------------------------------------------------
dir="${SCREPDB_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$dir" || err "cannot create install dir ${dir}"
chmod +x "${tmp}/${asset}"
mv "${tmp}/${asset}" "${dir}/screpdb" || err "cannot install to ${dir} (try SCREPDB_INSTALL_DIR=/usr/local/bin)"

info ""
info "Installed screpdb to ${dir}/screpdb"

case ":${PATH}:" in
  *":${dir}:"*) info "Run: screpdb" ;;
  *)
    info "⚠️  ${dir} is not on your PATH. Add it, e.g.:"
    info "    echo 'export PATH=\"${dir}:\$PATH\"' >> ~/.profile && . ~/.profile"
    info "Or run it directly: ${dir}/screpdb"
    ;;
esac
