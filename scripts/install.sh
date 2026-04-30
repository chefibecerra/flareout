#!/usr/bin/env sh
# FlareOut installer: downloads the latest release archive for the current
# platform, extracts the binary, and installs it under $PREFIX/bin (default
# $HOME/.local/bin) so it lands on the user's PATH for typical setups.
#
# Override with PREFIX=/usr/local sh install.sh — that requires sudo.
#
# Verifies the SHA256 checksum from checksums.txt before installing.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/chefibecerra/flareout/main/scripts/install.sh | sh
#   PREFIX=/usr/local curl -fsSL ... | sudo sh

set -eu

REPO="chefibecerra/flareout"
PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="$PREFIX/bin"

die() {
  printf 'install: %s\n' "$1" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"
}

need curl
need tar
need uname

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux|darwin) : ;;
  *) die "unsupported OS: $OS (linux and darwin only via this script; Windows users grab the .zip from the release page)" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "unsupported architecture: $ARCH_RAW" ;;
esac

API_URL="https://api.github.com/repos/$REPO/releases/latest"
TAG="$(curl -fsSL "$API_URL" | grep '"tag_name":' | head -n1 | sed -E 's/.*"([^"]+)".*/\1/' || true)"
if [ -z "${TAG:-}" ]; then
  die "could not resolve latest release tag from $API_URL"
fi

VERSION="${TAG#v}"
ASSET="flareout_${VERSION}_${OS}_${ARCH}.tar.gz"
ASSET_URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"
SUMS_URL="https://github.com/$REPO/releases/download/$TAG/checksums.txt"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

printf 'install: fetching %s\n' "$ASSET"
curl -fsSL -o "$TMP/$ASSET" "$ASSET_URL"
curl -fsSL -o "$TMP/checksums.txt" "$SUMS_URL"

if command -v sha256sum >/dev/null 2>&1; then
  ( cd "$TMP" && grep "  $ASSET\$" checksums.txt | sha256sum -c - >/dev/null ) \
    || die "checksum mismatch on $ASSET"
elif command -v shasum >/dev/null 2>&1; then
  ( cd "$TMP" && grep "  $ASSET\$" checksums.txt | shasum -a 256 -c - >/dev/null ) \
    || die "checksum mismatch on $ASSET"
else
  printf 'install: WARNING: no sha256sum or shasum on PATH; SKIPPING checksum verification\n' >&2
fi

tar -xzf "$TMP/$ASSET" -C "$TMP" flareout

mkdir -p "$BIN_DIR"
install -m 0755 "$TMP/flareout" "$BIN_DIR/flareout"

printf 'install: %s installed to %s\n' "$TAG" "$BIN_DIR/flareout"
case ":$PATH:" in
  *":$BIN_DIR:"*) : ;;
  *) printf 'install: WARNING: %s is not on your PATH. Add it to your shell rc:\n  export PATH="%s:$PATH"\n' "$BIN_DIR" "$BIN_DIR" >&2 ;;
esac
