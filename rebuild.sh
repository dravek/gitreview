#!/usr/bin/env bash

if [ -z "${BASH_VERSION:-}" ]; then
  exec bash "$0" "$@"
fi

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_CACHE="$ROOT_DIR/.cache/go-build"
OUTPUT_BIN="$ROOT_DIR/gitreview"
VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

sanitize_token() {
  printf '%s' "$1" | tr -cs 'A-Za-z0-9._:-' '_'
}

VERSION_SAFE="$(sanitize_token "$VERSION")"
COMMIT_SAFE="$(sanitize_token "$COMMIT")"
DATE_SAFE="$(sanitize_token "$DATE")"

mkdir -p "$BUILD_CACHE"

echo "Building gitreview..."
GOCACHE="$BUILD_CACHE" go build \
  -ldflags "-X gitreview/internal/version.Version=${VERSION_SAFE} -X gitreview/internal/version.Commit=${COMMIT_SAFE} -X gitreview/internal/version.Date=${DATE_SAFE}" \
  -o "$OUTPUT_BIN" \
  ./cmd/gitreview

echo "Built: $OUTPUT_BIN"
echo "Version: $VERSION_SAFE ($COMMIT_SAFE)"

if [[ "${1:-}" == "--install" ]]; then
  TARGET="/usr/local/bin/gitreview"
  TEMP_TARGET="${TARGET}.new"

  echo "Installing to ${TARGET}..."
  sudo cp "$OUTPUT_BIN" "$TEMP_TARGET"
  sudo chmod +x "$TEMP_TARGET"

  if ! sudo mv "$TEMP_TARGET" "$TARGET"; then
    echo "Install failed: ${TARGET} is likely still running." >&2
    echo "Stop running gitreview processes and try again:" >&2
    echo "  pkill -x gitreview" >&2
    exit 1
  fi

  echo "Installed: ${TARGET}"
fi
