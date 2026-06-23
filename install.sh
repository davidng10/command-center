#!/usr/bin/env bash
# Install fleet — works both locally (cloned repo) and remotely (curl | sh).
#
#   curl -fsSL https://raw.githubusercontent.com/davidng10/command-center/main/install.sh | sh
#
# Overrides:
#   BIN_DIR   — install location (default: ~/.local/bin)
#   VERSION   — specific release tag (default: latest)
set -euo pipefail

REPO="davidng10/command-center"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
CMD_NAME="fleet"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) echo "error: unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin | linux) ;;
  *) echo "error: unsupported OS: $os — on native Windows use install.ps1, or use WSL" >&2; exit 1 ;;
esac

asset="fleet_${os}_${arch}"
dest="$BIN_DIR/$CMD_NAME"
mkdir -p "$BIN_DIR"

# 1) Local build available? Use it.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}" 2>/dev/null)" 2>/dev/null && pwd 2>/dev/null || echo "")"
if [ -n "$script_dir" ] && [ -f "$script_dir/dist/$asset" ]; then
  echo "installing from local build: dist/$asset"
  install -m 0755 "$script_dir/dist/$asset" "$dest"
else
  # 2) Download from GitHub Releases.
  if [ -n "${VERSION:-}" ]; then
    url="https://github.com/$REPO/releases/download/$VERSION/$asset"
  else
    url="https://github.com/$REPO/releases/latest/download/$asset"
  fi
  echo "downloading $url"
  curl -fsSL "$url" -o "$dest"
  chmod 0755 "$dest"
fi

echo "installed $CMD_NAME -> $dest"
echo "version: $("$dest" --version 2>/dev/null || echo "unknown")"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo ""
    echo "note: $BIN_DIR is not on your PATH. Add this to your shell profile:"
    echo "      export PATH=\"$BIN_DIR:\$PATH\""
    ;;
esac
