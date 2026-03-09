#!/usr/bin/env bash

if [ -z "${BASH_VERSION:-}" ]; then
  exec bash "$0" "$@"
fi

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_CACHE="$ROOT_DIR/.cache/go-build"
DIST_DIR="$ROOT_DIR/dist"
VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

sanitize_token() {
  printf '%s' "$1" | tr -cs 'A-Za-z0-9._:-' '_'
}

VERSION_SAFE="$(sanitize_token "$VERSION")"
COMMIT_SAFE="$(sanitize_token "$COMMIT")"
DATE_SAFE="$(sanitize_token "$DATE")"

mkdir -p "$BUILD_CACHE" "$DIST_DIR"
rm -f "$DIST_DIR"/gitreview-*

build_target() {
  local goos="$1"
  local goarch="$2"
  local ext=""
  local bin_name="gitreview"
  local archive_base="gitreview-${VERSION_SAFE}-${goos}-${goarch}"

  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
    bin_name="gitreview.exe"
  fi

  echo "Building ${goos}/${goarch}..."
  GOOS="$goos" GOARCH="$goarch" GOCACHE="$BUILD_CACHE" \
    go build \
      -ldflags "-X gitreview/internal/version.Version=${VERSION_SAFE} -X gitreview/internal/version.Commit=${COMMIT_SAFE} -X gitreview/internal/version.Date=${DATE_SAFE}" \
      -o "$DIST_DIR/$bin_name" \
      ./cmd/gitreview

  if [[ "$goos" == "windows" ]]; then
    (
      cd "$DIST_DIR"
      zip -q "${archive_base}.zip" "$bin_name"
      rm -f "$bin_name"
    )
  else
    (
      cd "$DIST_DIR"
      tar -czf "${archive_base}.tar.gz" "$bin_name"
      rm -f "$bin_name"
    )
  fi
}

build_target linux amd64
build_target darwin amd64
build_target darwin arm64

echo "Release artifacts written to $DIST_DIR"
