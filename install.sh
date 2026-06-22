#!/usr/bin/env bash
# Install fleet on macOS / Linux / WSL.
#
#   Local (from a cloned repo, after ./build.sh):  ./install.sh
#   Remote (from GitHub Releases):                 RELEASE_BASE_URL=... ./install.sh
#
# Overrides: BIN_DIR (install location), CMD_NAME (command name).
set -euo pipefail

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
CMD_NAME="${CMD_NAME:-fleet}"
# e.g. https://github.com/yourorg/command-center/releases/latest/download
RELEASE_BASE_URL="${RELEASE_BASE_URL:-}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin | linux) ;;
  *) echo "unsupported OS: $os — on native Windows use install.ps1, or use WSL" >&2; exit 1 ;;
esac

asset="fleet_${os}_${arch}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dest="$BIN_DIR/$CMD_NAME"

mkdir -p "$BIN_DIR"

if [ -f "$script_dir/dist/$asset" ]; then
  echo "installing from local build: dist/$asset"
  install -m 0755 "$script_dir/dist/$asset" "$dest"
elif [ -n "$RELEASE_BASE_URL" ]; then
  echo "downloading $RELEASE_BASE_URL/$asset"
  curl -fsSL "$RELEASE_BASE_URL/$asset" -o "$dest"
  chmod 0755 "$dest"
else
  echo "no local dist/$asset and RELEASE_BASE_URL not set." >&2
  echo "run ./build.sh first, or set RELEASE_BASE_URL to your releases URL." >&2
  exit 1
fi

echo "installed $CMD_NAME -> $dest"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo ""
    echo "note: $BIN_DIR is not on your PATH. Add this to your shell profile:"
    echo "      export PATH=\"$BIN_DIR:\$PATH\""
    ;;
esac
