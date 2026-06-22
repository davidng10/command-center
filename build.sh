#!/usr/bin/env bash
# Cross-compile fleet for every supported platform into dist/.
# Usage: ./build.sh [version]   (version defaults to the git tag/commit)
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
OUT="dist"
LDFLAGS="-s -w -X main.version=${VERSION}"

rm -rf "$OUT"
mkdir -p "$OUT"

# "GOOS GOARCH" pairs. Windows is amd64+arm64; arm64 runs natively or emulated.
targets=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

echo "building fleet ${VERSION}"
for t in "${targets[@]}"; do
  read -r goos goarch <<<"$t"
  ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  out="$OUT/fleet_${goos}_${goarch}${ext}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$out" .
  echo "  ✓ $out"
done

echo ""
echo "artifacts in $OUT/:"
ls -lh "$OUT"
